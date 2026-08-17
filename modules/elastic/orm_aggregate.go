/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"encoding/json"
	"strconv"
	"time"
	"fmt"

	"infini.sh/framework/core/aggregate"
	api "infini.sh/framework/core/orm"
)

// Aggregate implements orm.MetricsAPI for the elastic backend.
//
// Bucket and metric aggregations execute natively on the cluster through
// the existing SearchV2 + AggreationBuilder path; the ES-shaped response is
// parsed into the typed tree, and pipeline aggregations are then recomputed
// by the framework engine so elastic and sqlite agree exactly (design doc
// §6.1, option b).
func (handler *ElasticORM) Aggregate(ctx *api.Context, qb *api.QueryBuilder) (*api.AggregationResult, error) {
	if qb == nil || len(qb.Aggs) == 0 {
		return nil, fmt.Errorf("no aggregations set on the query builder")
	}
	res, err := handler.SearchV2(ctx, qb)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations map[string]interface{} `json:"aggregations"`
	}
	if payload, ok := res.Payload.([]byte); ok && len(payload) > 0 {
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return nil, fmt.Errorf("parse aggregation response: %w", err)
		}
	}
	if len(envelope.Aggregations) == 0 {
		return &api.AggregationResult{Aggs: map[string]*api.AggNode{}}, nil
	}

	nodes := parseESAggregations(envelope.Aggregations)
	if envelope.Hits.Total.Value == 0 {
		// Metric aggregations over zero matched docs report value:0.0 in ES -
		// a placeholder, not a set value. Clear ValueSet so consumers (and
		// the cross-backend conformance suite) read "no value", matching the
		// sqlite backend and ES's own null-on-percentiles semantics.
		for _, node := range nodes {
			if node.ValueSet && len(node.Buckets) == 0 {
				node.ValueSet = false
				node.Value = 0
			}
		}
	}
	result := &api.AggregationResult{Aggs: nodes}
	if err := aggregate.ApplyPipelines(result, qb.Aggs); err != nil {
		return nil, err
	}
	return result, nil
}

// parseESAggregations converts ES-shaped aggregation maps into the typed
// tree (recursive over buckets and sub-aggregations).
func parseESAggregations(raw map[string]interface{}) map[string]*api.AggNode {
	out := make(map[string]*api.AggNode, len(raw))
	for name, v := range raw {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		out[name] = parseESAggNode(m)
	}
	return out
}

func parseESAggNode(m map[string]interface{}) *api.AggNode {
	node := &api.AggNode{}
	if rawBuckets, ok := m["buckets"].([]interface{}); ok {
		buckets := make([]api.Bucket, 0, len(rawBuckets))
		for _, rb := range rawBuckets {
			bm, ok := rb.(map[string]interface{})
			if !ok {
				continue
			}
			bucket := api.Bucket{Aggs: map[string]*api.AggNode{}}
			switch k := bm["key"].(type) {
			case string:
				bucket.Key = k
				bucket.KeyRaw = k
			case float64:
				bucket.Key = fmt.Sprintf("%v", k)
				// Numeric (epoch-millis) keys are normalized to int64 — the
				// conformance suite asserts int64 KeyRaw and sqlite already
				// yields int64 (UnixMilli).
				if k == float64(int64(k)) {
					bucket.KeyRaw = int64(k)
				} else {
					bucket.KeyRaw = k
				}
			case json.Number:
				bucket.Key = k.String()
				if i, err := k.Int64(); err == nil {
					bucket.KeyRaw = i
				} else {
					bucket.KeyRaw = k
				}
			}
			if kas, ok := bm["key_as_string"].(string); ok {
				bucket.Key = normalizeESBucketKey(kas)
			}
			if dc, ok := bm["doc_count"].(float64); ok {
				bucket.DocCount = int64(dc)
			}
			for k, v := range bm {
				switch k {
				case "key", "key_as_string", "doc_count", "doc_count_error_upper_bound":
					continue
				}
				if sub, ok := v.(map[string]interface{}); ok {
					bucket.Aggs[k] = parseESAggNode(sub)
				}
			}
			buckets = append(buckets, bucket)
		}
		node.Buckets = buckets
		return node
	}
	if v, ok := m["value"].(float64); ok {
		node.Value = v
		node.ValueSet = true
	} else if _, ok := m["value"]; !ok {
		// single-bucket aggregations (filter): inline scope, no buckets array
		bucket := api.Bucket{Aggs: map[string]*api.AggNode{}}
		if dc, ok := m["doc_count"].(float64); ok {
			bucket.DocCount = int64(dc)
		}
		for k, v := range m {
			if k == "doc_count" {
				continue
			}
			if sub, ok := v.(map[string]interface{}); ok {
				bucket.Aggs[k] = parseESAggNode(sub)
			}
		}
		if len(bucket.Aggs) > 0 || bucket.DocCount > 0 {
			node.Buckets = []api.Bucket{bucket}
		}
	}
	if rawValues, ok := m["values"].(map[string]interface{}); ok {
		values := make(map[string]float64, len(rawValues))
		for k, v := range rawValues {
			if f, ok := v.(float64); ok {
				// ES emits percentiles keys as "50.0"/"100.0"; normalize to
				// the bare number ("50"/"100") so both backends share keys.
				if fk, err := strconv.ParseFloat(k, 64); err == nil && fk == float64(int64(fk)) {
					k = strconv.FormatInt(int64(fk), 10)
				}
				values[k] = f
			}
		}
		node.Values = values
	}
	if hits, ok := m["hits"].(map[string]interface{}); ok {
		if inner, ok := hits["hits"].([]interface{}); ok && len(inner) > 0 {
			if hit, ok := inner[0].(map[string]interface{}); ok {
				if src, ok := hit["_source"]; ok {
					raw, _ := json.Marshal(src)
					doc := json.RawMessage(raw)
					node.TopHit = &doc
				}
			}
		}
	}
	return node
}


// normalizeESBucketKey renders ES date_histogram key_as_string values in the
// canonical second-precision layout (2006-01-02T15:04:05, UTC, no millis)
// used by the cross-backend conformance suite — ES emits RFC3339 with
// milliseconds ("2026-08-13T00:00:00.000Z"), which broke parity with the
// sqlite backend's keys.
func normalizeESBucketKey(kas string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000", "2006-01-02T15:04:05"} {
		if ts, err := time.Parse(layout, kas); err == nil {
			return ts.UTC().Format("2006-01-02T15:04:05")
		}
	}
	return kas
}

/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"encoding/json"
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
				bucket.KeyRaw = k
			case json.Number:
				bucket.Key = k.String()
				bucket.KeyRaw = k
			}
			if kas, ok := bm["key_as_string"].(string); ok {
				bucket.Key = kas
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

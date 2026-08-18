/* Copyright © INFINI LTD. All rights reserved. */

package orm

import (
	"encoding/json"
	"errors"
)

// ──────────────────────────────────────────────────────────────────────────
// Aggregation contract.
//
// Aggregations are a first-class operation: Aggregate(ctx, qb) executes the
// qb.Aggs tree and returns a typed, recursive result model — independent of
// the ES-shaped JSON that SearchV2 still returns for backward compatibility.
// Bucket and metric aggregations are computed by each backend natively;
// pipeline aggregations are computed uniformly by the framework engine
// (core/aggregate) so every backend behaves identically.
// ──────────────────────────────────────────────────────────────────────────

// AggregationResult is the root of the typed aggregation tree. Keys of Aggs
// correspond one-to-one to the qb.Aggs names.
type AggregationResult struct {
	Aggs map[string]*AggNode `json:"aggs,omitempty"`
}

// AggNode is one named aggregation's result. Exactly one shape is populated:
// a single value (Value), multi values (Values, e.g. percentiles), a top
// document (TopHit), or buckets (Buckets).
type AggNode struct {
	Value    float64            `json:"value,omitempty"`
	ValueSet bool               `json:"value_set,omitempty"` // distinguishes 0 from "no value" (ES null)
	Values   map[string]float64 `json:"values,omitempty"`
	TopHit   *json.RawMessage   `json:"top_hit,omitempty"`
	Buckets  []Bucket           `json:"buckets,omitempty"`
}

// Bucket is one bucket of a bucket aggregation. Key is the display key
// (term value or formatted time bucket); KeyRaw carries the numeric key
// when the bucket is time-based (epoch milliseconds, ES convention).
type Bucket struct {
	Key      string              `json:"key,omitempty"`
	KeyRaw   interface{}         `json:"key_raw,omitempty"`
	DocCount int64               `json:"doc_count,omitempty"`
	Aggs     map[string]*AggNode `json:"aggs,omitempty"`
}

// Aggregate executes the aggregations of qb through the registered backend.
// The WHERE clauses of qb scope the aggregation set, exactly like SearchV2.
func Aggregate(ctx *Context, qb *QueryBuilder) (*AggregationResult, error) {
	if ctx == nil {
		ctx = NewContext()
	}
	if qb == nil {
		return nil, errors.New("query builder is required for aggregation")
	}
	if len(qb.Aggs) == 0 {
		return nil, errors.New("no aggregations set on the query builder")
	}
	m, ok := getHandler().(MetricsAPI)
	if !ok {
		return nil, errors.New("ORM backend does not support Aggregate")
	}
	return m.Aggregate(ctx, qb)
}

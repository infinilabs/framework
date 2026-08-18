/* Copyright © INFINI LTD. All rights reserved. */

package aggregate

import (
	"math"
	"testing"

	"infini.sh/framework/core/orm"
)

func mkBuckets(values ...int64) []orm.Bucket {
	out := make([]orm.Bucket, 0, len(values))
	for _, v := range values {
		out = append(out, orm.Bucket{
			Key:      "",
			DocCount: v,
			Aggs:     map[string]*orm.AggNode{"count": {Value: float64(v), ValueSet: true}},
		})
	}
	return out
}

func TestApplyPipelines_Derivative(t *testing.T) {
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{
		"dates": {Buckets: mkBuckets(10, 25, 20, 40)},
	}}
	derivative := &orm.DerivativeAggregation{BucketsPath: "count"}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dates.AddNested("derivative", derivative)

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"dates": dates}); err != nil {
		t.Fatal(err)
	}
	buckets := res.Aggs["dates"].Buckets
	if len(buckets) != 4 {
		t.Fatalf("buckets = %d", len(buckets))
	}
	if buckets[0].Aggs["derivative"] != nil && buckets[0].Aggs["derivative"].ValueSet {
		t.Fatal("first bucket has no derivative")
	}
	want := []float64{15, -5, 20}
	for i, w := range want {
		node := buckets[i+1].Aggs["derivative"]
		if node == nil || !node.ValueSet || node.Value != w {
			t.Fatalf("bucket %d derivative = %+v, want %v", i+1, node, w)
		}
	}
}

func TestApplyPipelines_DerivativeOfDocCount(t *testing.T) {
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{
		"dates": {Buckets: mkBuckets(5, 9)},
	}}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dates.AddNested("rate", &orm.DerivativeAggregation{BucketsPath: "._count"})

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"dates": dates}); err != nil {
		t.Fatal(err)
	}
	node := res.Aggs["dates"].Buckets[1].Aggs["rate"]
	if node == nil || node.Value != 4 {
		t.Fatalf("derivative of _count = %+v, want 4", node)
	}
}

func TestApplyPipelines_SumBucket(t *testing.T) {
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{
		"dates": {Buckets: mkBuckets(1, 2, 3, 4)},
	}}
	sum := &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "dates>count"}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1d"}

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"dates": dates, "total": sum}); err != nil {
		t.Fatal(err)
	}
	if got := res.Aggs["total"].Value; got != 10 {
		t.Fatalf("sum_bucket = %v, want 10", got)
	}
}

func TestApplyPipelines_MaxBucket(t *testing.T) {
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{
		"dates": {Buckets: mkBuckets(7, 2, 9, 4)},
	}}
	mb := &orm.MaxBucketAggregation{BucketsPath: "dates>count"}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1d"}

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"dates": dates, "peak": mb}); err != nil {
		t.Fatal(err)
	}
	if got := res.Aggs["peak"].Value; got != 9 {
		t.Fatalf("max_bucket = %v, want 9", got)
	}
}

func TestApplyPipelines_BucketScript(t *testing.T) {
	// Latency ratio per time bucket: query_time / query_total * 100.
	buckets := []orm.Bucket{
		{Key: "t1", Aggs: map[string]*orm.AggNode{
			"qt": {Value: 30, ValueSet: true},
			"qc": {Value: 100, ValueSet: true},
		}},
		{Key: "t2", Aggs: map[string]*orm.AggNode{
			"qt": {Value: 45, ValueSet: true},
			"qc": {Value: 150, ValueSet: true},
		}},
		{Key: "t3", Aggs: map[string]*orm.AggNode{
			"qt": {Value: 10, ValueSet: true},
			"qc": {Value: 0, ValueSet: true}, // division by zero → 0
		}},
	}
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{"dates": {Buckets: buckets}}}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dates.AddNested("ratio", &orm.BucketScriptAggregation{
		BucketsPath: map[string]string{"a": "qt", "b": "qc"},
		Script:      "params.a / params.b * 100",
	})

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"dates": dates}); err != nil {
		t.Fatal(err)
	}
	want := []float64{30, 30, 0}
	for i, w := range want {
		node := buckets[i].Aggs["ratio"]
		if node == nil || !nearly(node.Value, w) {
			t.Fatalf("bucket %d ratio = %+v, want %v", i, node, w)
		}
	}
}

func TestApplyPipelines_BucketSort(t *testing.T) {
	buckets := []orm.Bucket{
		{Key: "a", Aggs: map[string]*orm.AggNode{"v": {Value: 3, ValueSet: true}}},
		{Key: "b", Aggs: map[string]*orm.AggNode{"v": {Value: 1, ValueSet: true}}},
		{Key: "c", Aggs: map[string]*orm.AggNode{"v": {Value: 2, ValueSet: true}}},
	}
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{"terms": {Buckets: buckets}}}
	terms := &orm.TermsAggregation{Field: "x"}
	terms.AddNested("top2", &orm.BucketSortAggregation{
		Sort: []orm.BucketSortSpec{{Path: "v", Desc: true}},
		Size: 2,
	})

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"terms": terms}); err != nil {
		t.Fatal(err)
	}
	// Top-2 by value desc: a(3), c(2); b truncated.
	if len(buckets) < 2 || buckets[0].Key != "a" || buckets[1].Key != "c" {
		t.Fatalf("sort order wrong: %+v", buckets)
	}
	if buckets[2].Key != "" || buckets[2].Aggs != nil {
		t.Fatalf("trailing bucket not cleared: %+v", buckets[2])
	}
}

func TestApplyPipelines_NestedChains(t *testing.T) {
	// terms(stream) → date_histogram → sum; sum_bucket over "dates>bytes".
	streams := []orm.Bucket{
		{Key: "s1", Aggs: map[string]*orm.AggNode{
			"dates": {Buckets: []orm.Bucket{
				{Aggs: map[string]*orm.AggNode{"bytes": {Value: 100, ValueSet: true}}},
				{Aggs: map[string]*orm.AggNode{"bytes": {Value: 50, ValueSet: true}}},
			}},
		}},
		{Key: "s2", Aggs: map[string]*orm.AggNode{
			"dates": {Buckets: []orm.Bucket{
				{Aggs: map[string]*orm.AggNode{"bytes": {Value: 200, ValueSet: true}}},
			}},
		}},
	}
	res := &orm.AggregationResult{Aggs: map[string]*orm.AggNode{"streams": {Buckets: streams}}}

	streamsAgg := &orm.TermsAggregation{Field: "stream_id"}
	dates := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dates.AddNested("bytes", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	streamsAgg.AddNested("dates", dates)
	streamsAgg.AddNested("total", &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "dates>bytes"})

	if err := ApplyPipelines(res, map[string]orm.Aggregation{"streams": streamsAgg}); err != nil {
		t.Fatal(err)
	}
	if got := streams[0].Aggs["total"].Value; got != 150 {
		t.Fatalf("s1 total = %v, want 150", got)
	}
	if got := streams[1].Aggs["total"].Value; got != 200 {
		t.Fatalf("s2 total = %v, want 200", got)
	}
}

func TestEvalScript(t *testing.T) {
	cases := []struct {
		script string
		params map[string]float64
		want   float64
	}{
		{"params.a / params.b * 100", map[string]float64{"a": 30, "b": 100}, 30},
		{"params.a + params.b", map[string]float64{"a": 1, "b": 2}, 3},
		{"(params.a + params.b) * 2", map[string]float64{"a": 1, "b": 2}, 6},
		{"-params.a", map[string]float64{"a": 5}, -5},
		{"params.a * 1.5", map[string]float64{"a": 4}, 6},
		{"params.a - params.b / 2", map[string]float64{"a": 3, "b": 4}, 1},
	}
	for _, c := range cases {
		got, err := EvalScript(c.script, c.params)
		if err != nil {
			t.Fatalf("EvalScript(%q): %v", c.script, err)
		}
		if !nearly(got, c.want) {
			t.Errorf("EvalScript(%q) = %v, want %v", c.script, got, c.want)
		}
	}
	if _, err := EvalScript("params.a +", nil); err == nil {
		t.Error("malformed script must error")
	}
	if _, err := EvalScript("params.missing", nil); err == nil {
		t.Error("unknown param must error")
	}
}

func nearly(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

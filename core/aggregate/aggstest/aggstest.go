/* Copyright © INFINI LTD. All rights reserved. */

// Package aggstest is the cross-backend aggregation conformance suite
// (design doc §8): the same fixture + spec runs against every backend and
// must produce the same typed AggregationResult. Backends implement the
// Backend interface and call RunConformance from a unit test; RunParity
// deep-compares two backends (e.g. sqlite vs a live elastic cluster).
package aggstest

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/orm"
)

// Doc is one fixture document; backends materialize it into their store.
type Doc = map[string]interface{}

// Backend adapts a store for the suite. Setup registers schema, loads docs,
// and returns a context bound to the model plus a cleanup.
type Backend interface {
	Setup(t *testing.T, docs []Doc) (ctx *orm.Context, cleanup func())
	Aggregate(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.AggregationResult, error)
}

// RunConformance executes the full case catalog against one backend.
func RunConformance(t *testing.T, b Backend) {
	t.Run("metrics", func(t *testing.T) { conformanceMetrics(t, b) })
	t.Run("terms", func(t *testing.T) { conformanceTerms(t, b) })
	t.Run("nested terms", func(t *testing.T) { conformanceNestedTerms(t, b) })
	t.Run("date histogram", func(t *testing.T) { conformanceDateHistogram(t, b) })
	t.Run("date range", func(t *testing.T) { conformanceDateRange(t, b) })
	t.Run("filter bucket", func(t *testing.T) { conformanceFilter(t, b) })
	t.Run("top hits", func(t *testing.T) { conformanceTopHits(t, b) })
	t.Run("percentiles", func(t *testing.T) { conformancePercentiles(t, b) })
	t.Run("pipelines", func(t *testing.T) { conformancePipelines(t, b) })
	t.Run("deep chain", func(t *testing.T) { conformanceDeepChain(t, b) })
	t.Run("empty set", func(t *testing.T) { conformanceEmptySet(t, b) })
}

func fixtureDocs() []Doc {
	docs := []Doc{}
	// 6 hours × 2 streams; n increases; severities cycle.
	for h := 0; h < 6; h++ {
		for _, stream := range []string{"alpha", "beta"} {
			n := float64(h + 1)
			if stream == "beta" {
				n = float64(h + 7)
			}
			sev := "info"
			if h%3 == 0 {
				sev = "error"
			}
			docs = append(docs, Doc{
				"id":       docID(stream, h),
				"ts":       tsAt(h),
				"stream":   stream,
				"severity": sev,
				"n":        n,
			})
		}
	}
	return docs
}

func docID(stream string, h int) string { return stream + "-" + string(rune('a'+h)) }

func tsAt(hour int) string {
	// Fixed midnight UTC base keeps every backend deterministic.
	return "2026-08-13T" + pad(hour) + ":30:00Z"
}

func pad(h int) string {
	if h < 10 {
		return "0" + string(rune('0'+h))
	}
	return string(rune('0'+h/10)) + string(rune('0'+h%10))
}

func runAgg(t *testing.T, b Backend, docs []Doc, aggs map[string]orm.Aggregation) *orm.AggregationResult {
	t.Helper()
	ctx, cleanup := b.Setup(t, docs)
	t.Cleanup(cleanup)
	qb := orm.NewQuery()
	qb.SetAggregations(aggs)
	res, err := b.Aggregate(ctx, qb)
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

func conformanceMetrics(t *testing.T, b Backend) {
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{
		"total": &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"},
		"cnt":   &orm.MetricAggregation{Type: orm.MetricCount, Field: "n"},
		"avg":   &orm.MetricAggregation{Type: orm.MetricAvg, Field: "n"},
		"min":   &orm.MetricAggregation{Type: orm.MetricMin, Field: "n"},
		"max":   &orm.MetricAggregation{Type: orm.MetricMax, Field: "n"},
		"card":  &orm.MetricAggregation{Type: orm.MetricCardinality, Field: "stream"},
	})
	// n values: alpha 1..6 (21), beta 7..12 (57) → total 78 over 12 docs.
	assert.InEpsilon(t, 78, res.Aggs["total"].Value, 1e-9)
	assert.InEpsilon(t, 12, res.Aggs["cnt"].Value, 1e-9)
	assert.InEpsilon(t, 6.5, res.Aggs["avg"].Value, 1e-9)
	assert.InEpsilon(t, 1, res.Aggs["min"].Value, 1e-9)
	assert.InEpsilon(t, 12, res.Aggs["max"].Value, 1e-9)
	assert.InEpsilon(t, 2, res.Aggs["card"].Value, 1e-9)
}

func conformanceTerms(t *testing.T, b Backend) {
	terms := &orm.TermsAggregation{Field: "stream", Size: 10}
	terms.AddNested("sum_n", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"by_stream": terms})

	buckets := res.Aggs["by_stream"].Buckets
	require.Len(t, buckets, 2)
	// Ties on doc_count break by key ascending: alpha first.
	assert.Equal(t, "alpha", buckets[0].Key)
	assert.EqualValues(t, 6, buckets[0].DocCount)
	assert.InEpsilon(t, 21, buckets[0].Aggs["sum_n"].Value, 1e-9)
	assert.Equal(t, "beta", buckets[1].Key)
	assert.InEpsilon(t, 57, buckets[1].Aggs["sum_n"].Value, 1e-9)
}

func conformanceNestedTerms(t *testing.T, b Backend) {
	streams := &orm.TermsAggregation{Field: "stream", Size: 10}
	sevs := &orm.TermsAggregation{Field: "severity", Size: 10}
	streams.AddNested("by_sev", sevs)
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"streams": streams})

	buckets := res.Aggs["streams"].Buckets
	require.Len(t, buckets, 2)
	// h%3==0 → hours 0,3 → error; alpha error count 2.
	var alphaSevs []orm.Bucket
	for _, b2 := range buckets {
		if b2.Key == "alpha" {
			alphaSevs = b2.Aggs["by_sev"].Buckets
		}
	}
	require.NotEmpty(t, alphaSevs)
	var errCount, infoCount int64
	for _, sb := range alphaSevs {
		switch sb.Key {
		case "error":
			errCount = sb.DocCount
		case "info":
			infoCount = sb.DocCount
		}
	}
	assert.EqualValues(t, 2, errCount)
	assert.EqualValues(t, 4, infoCount)
}

func conformanceDateHistogram(t *testing.T, b Backend) {
	// Docs only at hours 0,1,3,5-ish (fixture has all 6) — use a filtered
	// subset to exercise zero fill: hours 0 and 3 → gap at 1,2.
	docs := []Doc{
		{"id": "z1", "ts": tsAt(0), "stream": "alpha", "n": 1.0},
		{"id": "z2", "ts": tsAt(0), "stream": "beta", "n": 2.0},
		{"id": "z3", "ts": tsAt(3), "stream": "alpha", "n": 4.0},
	}
	dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dh.AddNested("sum_n", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	res := runAgg(t, b, docs, map[string]orm.Aggregation{"over_time": dh})

	buckets := res.Aggs["over_time"].Buckets
	require.Len(t, buckets, 4, "hours 0..3 with zero fill at 1,2")
	assert.Equal(t, "2026-08-13T00:00:00", buckets[0].Key)
	assert.EqualValues(t, 2, buckets[0].DocCount)
	assert.InEpsilon(t, 3, buckets[0].Aggs["sum_n"].Value, 1e-9)
	assert.EqualValues(t, 0, buckets[1].DocCount, "zero-filled bucket")
	assert.EqualValues(t, 0, buckets[2].DocCount, "zero-filled bucket")
	assert.Equal(t, "2026-08-13T03:00:00", buckets[3].Key)
	assert.InEpsilon(t, 4, buckets[3].Aggs["sum_n"].Value, 1e-9)
	// Numeric epoch keys present.
	assert.True(t, buckets[0].KeyRaw.(int64) > 0)
}

func conformanceDateRange(t *testing.T, b Backend) {
	dr := &orm.DateRangeAggregation{Field: "ts", Ranges: []interface{}{
		map[string]interface{}{"from": "2026-08-13T00:00:00Z", "to": "2026-08-13T02:00:00Z", "key": "early"},
		map[string]interface{}{"from": "2026-08-13T02:00:00Z", "key": "late"},
	}}
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"ranges": dr})
	buckets := res.Aggs["ranges"].Buckets
	require.Len(t, buckets, 2)
	assert.EqualValues(t, 4, buckets[0].DocCount) // hours 0,1 × 2 streams
	assert.EqualValues(t, 8, buckets[1].DocCount)
}

func conformanceFilter(t *testing.T, b Backend) {
	filter := &orm.FilterAggregation{Query: map[string]interface{}{
		"term": map[string]interface{}{"severity": "error"},
	}}
	filter.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"errors": filter})

	buckets := res.Aggs["errors"].Buckets
	require.Len(t, buckets, 1)
	assert.EqualValues(t, 4, buckets[0].DocCount) // hours 0,3 × 2 streams
	// n sums: (h0: alpha 1 + beta 7) + (h3: alpha 4 + beta 10) = 22
	assert.InEpsilon(t, 22, buckets[0].Aggs["total"].Value, 1e-9)
}

func conformanceTopHits(t *testing.T, b Backend) {
	th := &orm.TopHitsAggregation{Sorts: []orm.Sort{{Field: "n", SortType: orm.DESC}}}
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"latest": th})
	node := res.Aggs["latest"]
	require.NotNil(t, node.TopHit)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(*node.TopHit, &doc))
	assert.Equal(t, "beta-f", doc["id"]) // highest n = 12 (hour 5 beta)
}

func conformancePercentiles(t *testing.T, b Backend) {
	p := &orm.PercentilesAggregation{Field: "n", Percents: []float64{50, 100}}
	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"p": p})
	values := res.Aggs["p"].Values
	require.NotEmpty(t, values)
	// 12 values 1..12 → p50 ∈ [6,7], p100 = 12 (backends may approximate).
	assert.InDelta(t, 6.5, values["50"], 1.0)
	assert.InDelta(t, 12, values["100"], 1e-9)
}

func conformancePipelines(t *testing.T, b Backend) {
	docs := []Doc{
		{"id": "p0", "ts": tsAt(0), "n": 10.0},
		{"id": "p1", "ts": tsAt(1), "n": 25.0},
		{"id": "p2", "ts": tsAt(2), "n": 20.0},
		{"id": "p3", "ts": tsAt(3), "n": 40.0},
	}
	dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dh.AddNested("sum_n", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	dh.AddNested("derivative", &orm.DerivativeAggregation{BucketsPath: "sum_n"})
	dh.AddNested("ratio", &orm.BucketScriptAggregation{
		BucketsPath: map[string]string{"a": "sum_n", "b": "sum_n"},
		Script:      "params.a / params.b",
	})
	sum := &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "over_time>sum_n"}
	mb := &orm.MaxBucketAggregation{BucketsPath: "over_time>sum_n"}

	res := runAgg(t, b, docs, map[string]orm.Aggregation{"over_time": dh, "total": sum, "peak": mb})
	buckets := res.Aggs["over_time"].Buckets
	require.Len(t, buckets, 4)
	if d := buckets[0].Aggs["derivative"]; d != nil {
		assert.False(t, d.ValueSet, "first bucket has no derivative")
	}
	assert.InEpsilon(t, 15, buckets[1].Aggs["derivative"].Value, 1e-9)
	assert.InEpsilon(t, -5, buckets[2].Aggs["derivative"].Value, 1e-9)
	assert.InEpsilon(t, 1, buckets[1].Aggs["ratio"].Value, 1e-9)
	assert.InEpsilon(t, 95, res.Aggs["total"].Value, 1e-9) // 10+25+20+40
	assert.InEpsilon(t, 40, res.Aggs["peak"].Value, 1e-9)
}

func conformanceDeepChain(t *testing.T, b Backend) {
	// terms(stream) → date_histogram(1h) → sum(n); sum_bucket per stream.
	streams := &orm.TermsAggregation{Field: "stream", Size: 10}
	dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dh.AddNested("bytes", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	streams.AddNested("dates", dh)
	streams.AddNested("total", &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "dates>bytes"})

	res := runAgg(t, b, fixtureDocs(), map[string]orm.Aggregation{"streams": streams})
	buckets := res.Aggs["streams"].Buckets
	require.Len(t, buckets, 2)
	for _, b2 := range buckets {
		require.Len(t, b2.Aggs["dates"].Buckets, 6)
	}
	// alpha total 21, beta total 57.
	totals := map[string]float64{}
	for _, b2 := range buckets {
		totals[b2.Key] = b2.Aggs["total"].Value
	}
	assert.InEpsilon(t, 21, totals["alpha"], 1e-9)
	assert.InEpsilon(t, 57, totals["beta"], 1e-9)
}

func conformanceEmptySet(t *testing.T, b Backend) {
	// Match-nothing filter scopes an empty aggregation set.
	ctx, cleanup := b.Setup(t, fixtureDocs())
	t.Cleanup(cleanup)
	qb := orm.NewQuery().Filter(orm.TermQuery("stream", "nope"))
	sum := &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"}
	qb.SetAggs("total", sum)
	res, err := b.Aggregate(ctx, qb)
	require.NoError(t, err)
	require.NotNil(t, res.Aggs["total"])
	assert.False(t, res.Aggs["total"].ValueSet, "sum over empty set has no value")
}

// RunParity executes the same spec against two backends and deep-compares
// the typed results (float epsilon; percentiles skipped — approximation
// strategies legitimately differ).
func RunParity(t *testing.T, a, b Backend) {
	check := func(t *testing.T, res *orm.AggregationResult) map[string]float64 {
		flattened := map[string]float64{}
		for name, node := range res.Aggs {
			flattenNode("."+name, node, flattened)
		}
		return flattened
	}
	docs := fixtureDocs()
	terms := &orm.TermsAggregation{Field: "stream", Size: 10}
	terms.AddNested("sum_n", &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"})
	dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
	dh.AddNested("count", &orm.MetricAggregation{Type: orm.MetricCount, Field: "n"})
	aggs := map[string]orm.Aggregation{"by_stream": terms, "over_time": dh}

	aCtx, aCleanup := a.Setup(t, docs)
	t.Cleanup(aCleanup)
	qb1 := orm.NewQuery()
	qb1.SetAggregations(aggs)
	aRes, err := a.Aggregate(aCtx, qb1)
	require.NoError(t, err)

	bCtx, bCleanup := b.Setup(t, docs)
	t.Cleanup(bCleanup)
	qb2 := orm.NewQuery()
	qb2.SetAggregations(aggs)
	bRes, err := b.Aggregate(bCtx, qb2)
	require.NoError(t, err)

	fa, fb := check(t, aRes), check(t, bRes)
	assert.Equal(t, len(fa), len(fb), "same flattened result size")
	for k, v := range fa {
		bv, ok := fb[k]
		if !ok {
			t.Errorf("key %q missing on backend b", k)
			continue
		}
		if math.Abs(v-bv) > 1e-6 {
			t.Errorf("key %q: a=%v b=%v", k, v, bv)
		}
	}
}

func flattenNode(prefix string, node *orm.AggNode, out map[string]float64) {
	if node == nil {
		return
	}
	if node.ValueSet {
		out[prefix+".value"] = node.Value
	}
	for i, b := range node.Buckets {
		bp := prefix + "[" + b.Key + "]"
		out[bp+".doc_count"] = float64(b.DocCount)
		for subName, subNode := range b.Aggs {
			flattenNode(bp+"."+subName, subNode, out)
		}
		_ = i
	}
}

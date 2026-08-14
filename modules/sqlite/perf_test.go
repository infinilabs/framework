/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

// Performance benchmarks at logpilot-realistic scale (PatternStats-shaped):
// boot-time schema ensure, dashboard aggregations, and the remaining
// full-table fetches. Run with:
//
//	go test ./modules/sqlite/ -run XXX -bench BenchmarkPerf -benchtime 1x -v
//
// (benchtime 1x: these benchmarks measure cold-ish single runs over a large
// fixture, not steady-state micro-ops.)

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"infini.sh/framework/core/orm"
)

// perfStat mirrors logpilot's PatternStats shape.
type perfStat struct {
	orm.ORMObjectBase
	StreamID    string    `json:"stream_id" elastic_mapping:"stream_id:{type:keyword}" sqlite_composite:"stream_id,pattern_id,bucket_start__epoch,count"`
	PatternID   string    `json:"pattern_id" elastic_mapping:"pattern_id:{type:keyword}" sqlite_composite:"pattern_id,bucket_start"`
	BucketStart time.Time `json:"bucket_start" elastic_mapping:"bucket_start:{type:date}"`
	Count       int64     `json:"count" elastic_mapping:"count:{type:long}"`
}

// perfStatSearch isolates the (stream,time) walk composite from the
// aggregation covering composite — both plans must coexist per query shape.
type perfStatSearch struct {
	orm.ORMObjectBase
	StreamID    string    `json:"stream_id" elastic_mapping:"stream_id:{type:keyword}" sqlite_composite:"stream_id,bucket_start__epoch"`
	PatternID   string    `json:"pattern_id" elastic_mapping:"pattern_id:{type:keyword}"`
	BucketStart time.Time `json:"bucket_start" elastic_mapping:"bucket_start:{type:date}"`
	Count       int64     `json:"count" elastic_mapping:"count:{type:long}"`
}

// perfPattern mirrors Pattern (has a text field → FTS triggers + backfill).
type perfPattern struct {
	orm.ORMObjectBase
	StreamID string `json:"stream_id" elastic_mapping:"stream_id:{type:keyword}"`
	Template string `json:"template" elastic_mapping:"template:{type:text}"`
}

const perfRows = 500_000

func seedPerfStats(b *testing.B) *SQLiteORM {
	b.Helper()
	handler := &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(b.TempDir(), "perf.db"),
	}}
	if err := handler.Open(); err != nil {
		b.Fatal(err)
	}
	if err := handler.RegisterSchemaWithName(perfStat{}, "perf_stats"); err != nil {
		b.Fatal(err)
	}
	if err := handler.RegisterSchemaWithName(perfPattern{}, "perf_patterns"); err != nil {
		b.Fatal(err)
	}

	tx, err := handler.DB.Begin()
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO perf_stats (id, raw) VALUES (?, ?)")
	if err != nil {
		b.Fatal(err)
	}
	base := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	patterns := 200
	for i := 0; i < perfRows; i++ {
		day := i % 7
		minute := i % 1440
		ts := base.Add(time.Duration(day)*24*time.Hour + time.Duration(minute)*time.Minute)
		raw := fmt.Sprintf(`{"id":"p%07d","stream_id":"s%d","pattern_id":"pat-%03d","bucket_start":%q,"count":%d}`,
			i, i%4, i%patterns, ts.Format(time.RFC3339), 1+i%97)
		if _, err := stmt.Exec(fmt.Sprintf("p%07d", i), raw); err != nil {
			b.Fatal(err)
		}
	}
	stmt.Close()
	// A modest pattern table (FTS-bearing).
	pstmt, err := tx.Prepare("INSERT INTO perf_patterns (id, raw) VALUES (?, ?)")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < patterns; i++ {
		raw := fmt.Sprintf(`{"id":"pat-%03d","stream_id":"s%d","template":"error in module %d while processing request"}`, i, i%4, i)
		if _, err := pstmt.Exec(fmt.Sprintf("pat-%03d", i), raw); err != nil {
			b.Fatal(err)
		}
	}
	pstmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	// Bulk-loaded after registration — refresh planner stats so index
	// choices reflect real cardinalities (production: PRAGMA optimize runs
	// on close / periodically).
	if _, err := handler.DB.Exec("PRAGMA optimize"); err != nil {
		b.Fatal(err)
	}
	return handler
}

func perfCtx(handler *SQLiteORM) *orm.Context {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &perfStat{})
	return ctx
}

// BenchmarkPerfBootEnsure measures re-running ensureFlattenedTable on an
// existing populated database — the per-boot schema path every open pays.
func BenchmarkPerfBootEnsure(b *testing.B) {
	handler := seedPerfStats(b)
	defer handler.Close()

	// The pattern table's FTS backfill anti-join is the suspect.
	schema := buildTableSchema("perf_patterns", perfPattern{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ensureFlattenedTable(handler.DB, schema); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPerfDashboardAggs measures the PatternOverview-shaped tree:
// terms(stream) → terms(pattern) → date_histogram(1h) → sum.
func BenchmarkPerfDashboardAggs(b *testing.B) {
	handler := seedPerfStats(b)
	defer handler.Close()
	ctx := perfCtx(handler)

	streams := &orm.TermsAggregation{Field: "stream_id", Size: 1000}
	patterns := &orm.TermsAggregation{Field: "pattern_id", Size: 10000}
	dh := &orm.DateHistogramAggregation{Field: "bucket_start", Interval: "1h"}
	dh.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "count"})
	patterns.AddNested("trend", dh)
	streams.AddNested("patterns", patterns)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Realistic window: PatternOverview aggregates the last 24h.
		qb := orm.NewQuery().Filter(orm.Range("bucket_start").Gte("2026-08-12T00:00:00Z"))
		qb.SetAggs("streams", streams)
		if _, err := handler.Aggregate(ctx, qb); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPerfHourlyTrend measures the Trends-shaped single series:
// offset 1h date_histogram + sum over the full table.
func BenchmarkPerfHourlyTrend(b *testing.B) {
	handler := seedPerfStats(b)
	defer handler.Close()
	ctx := perfCtx(handler)

	now := time.Date(2026, 8, 14, 10, 37, 12, 0, time.UTC)
	dh := &orm.DateHistogramAggregation{
		Field:    "bucket_start",
		Interval: "1h",
		Offset:   now.Sub(now.Truncate(time.Hour)),
	}
	dh.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "count"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb := orm.NewQuery()
		qb.SetAggs("trend", dh)
		if _, err := handler.Aggregate(ctx, qb); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPerfFullFetch measures the retired-but-still-present pattern:
// SearchV2 Size(10000) + its COUNT companion (two scans).
func BenchmarkPerfFullFetch(b *testing.B) {
	handler := seedPerfStats(b)
	defer handler.Close()
	ctx := perfCtx(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb := orm.NewQuery().Filter(orm.Range("bucket_start").Gte("2026-08-13T00:00:00Z")).Size(10000)
		if _, err := handler.SearchV2(ctx, qb); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPerfFilteredSearch measures a typical UI list query: term filter
// + sort by created DESC + size 50.
func BenchmarkPerfFilteredSearch(b *testing.B) {
	handler := seedPerfStats(b)
	defer handler.Close()

	// Same data registered with the (stream,time) composite only — the
	// walk-ordered plan a filtered search wants.
	if err := handler.RegisterSchemaWithName(perfStatSearch{}, "perf_search"); err != nil {
		b.Fatal(err)
	}
	tx, err := handler.DB.Begin()
	if err != nil {
		b.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO perf_search (id, raw) SELECT id, raw FROM perf_stats"); err != nil {
		b.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	sctx := orm.NewContext()
	orm.WithModel(sctx, &perfStatSearch{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb := orm.NewQuery().
			Filter(orm.TermQuery("stream_id", "s1")).
			SortBy(orm.Sort{Field: "bucket_start", SortType: orm.DESC}).
			Size(50)
		if _, err := handler.SearchV2(sctx, qb); err != nil {
			b.Fatal(err)
		}
	}
}

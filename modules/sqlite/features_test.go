/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	sqliteOrm "infini.sh/framework/modules/sqlite/orm"
)

// ftsDoc is a model exercising FTS (text) + flattened scalars together.
type ftsDoc struct {
	orm.ORMObjectBase
	Title string `json:"title,omitempty" elastic_mapping:"title: { type: text }"`
	Body  string `json:"body,omitempty" elastic_mapping:"body: { type: text }"`
	Tags  string `json:"tags,omitempty" elastic_mapping:"tags: { type: keyword }"`
	N     int    `json:"n,omitempty" elastic_mapping:"n: { type: integer }"`
}

func openFTSTestDB(t *testing.T) (*SQLiteORM, func()) {
	t.Helper()
	handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: filepath.Join(t.TempDir(), "fts.db")}}
	require.NoError(t, handler.Open())
	require.NoError(t, handler.RegisterSchemaWithName(ftsDoc{}, "fts_docs"))
	return handler, func() { handler.Close() }
}

func (h *SQLiteORM) saveFtsDoc(t *testing.T, id, title, body, tags string, n int) {
	t.Helper()
	doc := ftsDoc{Title: title, Body: body, Tags: tags, N: n}
	doc.ID = id
	require.NoError(t, h.Save(nil, &doc))
}

func decodeAggs(t *testing.T, res *orm.SearchResult) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(res.Payload.([]byte), &m))
	return m
}

func TestFTS_MatchQueries(t *testing.T) {
	handler, cleanup := openFTSTestDB(t)
	defer cleanup()

	handler.saveFtsDoc(t, "1", "hello world", "sqlite rocks", "a", 1)
	handler.saveFtsDoc(t, "2", "goodbye world", "postgres rocks", "b", 2)
	handler.saveFtsDoc(t, "3", "unrelated", "nothing here", "a", 3)

	ctx := orm.NewContext()
	orm.WithModel(ctx, &ftsDoc{})

	t.Run("match single word", func(t *testing.T) {
		res, err := handler.SearchV2(ctx, orm.NewQuery().Filter(orm.MatchQuery("title", "hello")))
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "1", items[0].ID)
	})

	t.Run("match any word (OR semantics)", func(t *testing.T) {
		res, err := handler.SearchV2(ctx, orm.NewQuery().Filter(orm.MatchQuery("title", "hello goodbye")))
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("match phrase keeps order", func(t *testing.T) {
		qb := orm.NewQuery().Filter(&orm.Clause{Field: "body", Operator: orm.QueryMatchPhrase, Value: "sqlite rocks"})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "1", items[0].ID)
	})

	t.Run("update resyncs index", func(t *testing.T) {
		handler.saveFtsDoc(t, "1", "changed title", "sqlite rocks", "a", 1)
		res, err := handler.SearchV2(ctx, orm.NewQuery().Filter(orm.MatchQuery("title", "hello")))
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Empty(t, items, "old term must be gone after update")
	})

	t.Run("delete resyncs index", func(t *testing.T) {
		doc := ftsDoc{}
		doc.ID = "2"
		require.NoError(t, handler.Delete(nil, &doc))
		res, err := handler.SearchV2(ctx, orm.NewQuery().Filter(orm.MatchQuery("title", "goodbye")))
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Empty(t, items, "deleted doc must leave the index")
	})

	t.Run("term on keyword unaffected", func(t *testing.T) {
		res, err := handler.SearchV2(ctx, orm.NewQuery().Filter(orm.TermQuery("tags", "a")))
		require.NoError(t, err)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
}

func TestPagination_FromWithoutSize(t *testing.T) {
	handler, cleanup := openFTSTestDB(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		handler.saveFtsDoc(t, fmt.Sprintf("p%d", i), "page", "body", "t", i)
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &ftsDoc{})

	// Regression: OFFSET without LIMIT used to emit invalid SQL.
	res, err := handler.SearchV2(ctx, orm.NewQuery().From(2))
	require.NoError(t, err)
	items, _, err := decodeLocal[ftsDoc](res)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestUnsupportedOperators_WarnAndDegrade(t *testing.T) {
	handler, cleanup := openFTSTestDB(t)
	defer cleanup()
	handler.saveFtsDoc(t, "n1", "nested", "body", "t", 1)

	ctx := orm.NewContext()
	orm.WithModel(ctx, &ftsDoc{})

	for _, op := range []orm.QueryType{orm.QuerySemantic, orm.QueryHybrid, orm.QueryNested} {
		qb := orm.NewQuery().Filter(&orm.Clause{Field: "title", Operator: op, Value: "x"})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err, "%s must not error (grace period)", op)
		items, _, err := decodeLocal[ftsDoc](res)
		require.NoError(t, err)
		assert.Empty(t, items, "%s matches nothing", op)
	}
}

func TestCompositeIndex(t *testing.T) {
	type compositeModel struct {
		orm.ORMObjectBase
		Status string `json:"status,omitempty" elastic_mapping:"status: { type: keyword }" sqlite_composite:"status,n"`
		N      int    `json:"n,omitempty" elastic_mapping:"n: { type: integer }"`
	}
	handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: filepath.Join(t.TempDir(), "comp.db")}}
	require.NoError(t, handler.Open())
	defer handler.Close()
	require.NoError(t, handler.RegisterSchemaWithName(compositeModel{}, "comp_docs"))

	idx := sqliteIndexDDL(t, handler.DB, "comp_docs")
	found := false
	for name, ddl := range idx {
		if strings.HasPrefix(name, "ixcc_") && containsAll(ddl, `"status"`, `"n"`) {
			found = true
		}
	}
	assert.True(t, found, "composite index on (status, n) should exist; got %v", idx)

	// The composite actually serves two-field filters.
	for i := 0; i < 100; i++ {
		m := compositeModel{Status: fmt.Sprintf("s%d", i%4), N: i}
		m.ID = fmt.Sprintf("c%d", i)
		require.NoError(t, handler.Save(nil, &m))
	}
	schema := lookupTableSchema("comp_docs")
	require.NotNil(t, schema)
	qb := orm.NewQuery().
		Filter(orm.TermQuery("status", "s1")).
		Filter(orm.Range("n").Gte(50))
	qb.Build()
	where, args := sqliteOrm.BuildWhereClause(qb, schema.resolver())
	require.NotEmpty(t, where)
	detail := queryPlanDetail(t, handler.DB, "EXPLAIN QUERY PLAN SELECT id FROM comp_docs WHERE "+where, args...)
	joined := fmt.Sprint(detail)
	assert.Contains(t, joined, "ixcc_", "plan should use the composite index; got: %s", joined)

	var n int
	require.NoError(t, handler.DB.QueryRow("SELECT COUNT(*) FROM comp_docs WHERE "+where, args...).Scan(&n))
	assert.Equal(t, 12, n) // s1: i%4==1, n>=50 → i ∈ {53,57,...,97}
}

// containsAll reports whether s contains every substring.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestAggregations(t *testing.T) {
	handler, cleanup := openFTSTestDB(t)
	defer cleanup()

	now := "2026-08-13T10:00:00Z"
	for i := 0; i < 10; i++ {
		doc := ftsDoc{
			Title: "t",
			Body:  "b",
			Tags:  fmt.Sprintf("tag%d", i%2),
			N:     i + 1,
		}
		doc.ID = fmt.Sprintf("a%d", i)
		doc.Created = mustTime(now)
		require.NoError(t, handler.Save(nil, &doc))
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &ftsDoc{})

	t.Run("terms + nested metric", func(t *testing.T) {
		terms := &orm.TermsAggregation{Field: "tags", Size: 10}
		terms.AddNested("avg_n", &orm.MetricAggregation{Type: orm.MetricAvg, Field: "n"})
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{"by_tag": terms})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err)
		out := decodeAggs(t, res)
		aggs := out["aggregations"].(map[string]interface{})
		byTag := aggs["by_tag"].(map[string]interface{})
		buckets := byTag["buckets"].([]interface{})
		require.Len(t, buckets, 2)
		b0 := buckets[0].(map[string]interface{})
		assert.Equal(t, float64(5), b0["doc_count"])
		sub := b0["avg_n"].(map[string]interface{})
		assert.Equal(t, float64(5), sub["value"]) // tag0: (1+3+5+7+9)/5; ties on doc_count break by key asc
	})

	t.Run("metrics", func(t *testing.T) {
		sum := &orm.MetricAggregation{Type: orm.MetricSum, Field: "n"}
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{"total": sum})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err)
		out := decodeAggs(t, res)
		total := out["aggregations"].(map[string]interface{})["total"].(map[string]interface{})
		assert.Equal(t, float64(55), total["value"])
	})

	t.Run("date_histogram hourly", func(t *testing.T) {
		dh := &orm.DateHistogramAggregation{Field: "created", Interval: "1h"}
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{"over_time": dh})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err)
		out := decodeAggs(t, res)
		buckets := out["aggregations"].(map[string]interface{})["over_time"].(map[string]interface{})["buckets"].([]interface{})
		require.Len(t, buckets, 1)
		b := buckets[0].(map[string]interface{})
		assert.Equal(t, float64(10), b["doc_count"])
		assert.Equal(t, "2026-08-13T10:00:00", b["key_as_string"])
	})

	t.Run("date_range", func(t *testing.T) {
		dr := &orm.DateRangeAggregation{Field: "created", Ranges: []interface{}{
			map[string]interface{}{"from": "2026-08-13T09:00:00Z", "to": "2026-08-13T11:00:00Z"},
			map[string]interface{}{"from": "2026-08-14T00:00:00Z"},
		}}
		qb := orm.NewQuery()
		qb.SetAggregations(map[string]orm.Aggregation{"ranges": dr})
		res, err := handler.SearchV2(ctx, qb)
		require.NoError(t, err)
		out := decodeAggs(t, res)
		buckets := out["aggregations"].(map[string]interface{})["ranges"].(map[string]interface{})["buckets"].([]interface{})
		require.Len(t, buckets, 2)
		assert.Equal(t, float64(10), buckets[0].(map[string]interface{})["doc_count"])
		assert.Equal(t, float64(0), buckets[1].(map[string]interface{})["doc_count"])
	})
}

func mustTime(s string) *time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &tm
}

func decodeLocal[T any](res *orm.SearchResult) ([]T, int64, error) {
	return elastic.DecodeHits[T](res)
}

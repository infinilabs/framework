/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"path/filepath"
	"testing"

	"infini.sh/framework/core/aggregate/aggstest"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// conformModel covers the suite's fixture fields with proper mappings so
// generated columns and FTS apply during conformance runs.
type conformModel struct {
	orm.ORMObjectBase
	TS       string `json:"ts,omitempty" elastic_mapping:"ts: { type: date }"`
	Stream   string `json:"stream,omitempty" elastic_mapping:"stream: { type: keyword }"`
	Severity string `json:"severity,omitempty" elastic_mapping:"severity: { type: keyword }"`
	N        int    `json:"n,omitempty" elastic_mapping:"n: { type: integer }"`
}

// sqliteAggstestBackend adapts SQLiteORM to the conformance suite.
type sqliteAggstestBackend struct {
	handler *SQLiteORM
}

func (s *sqliteAggstestBackend) Setup(t *testing.T, docs []aggstest.Doc) (*orm.Context, func()) {
	t.Helper()
	s.handler = &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(t.TempDir(), "conformance.db"),
	}}
	if err := s.handler.Open(); err != nil {
		t.Fatal(err)
	}
	if err := s.handler.RegisterSchemaWithName(conformModel{}, "conform_docs"); err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		raw := util.MustToJSONBytes(d)
		id, _ := d["id"].(string)
		if _, err := s.handler.DB.Exec("INSERT INTO conform_docs (id, raw) VALUES (?, ?)", id, raw); err != nil {
			t.Fatal(err)
		}
	}
	ctx := orm.NewContext()
	orm.WithModel(ctx, &conformModel{})
	return ctx, func() { s.handler.Close() }
}

func (s *sqliteAggstestBackend) Aggregate(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.AggregationResult, error) {
	return s.handler.Aggregate(ctx, qb)
}

// TestAggConformance_SQLite runs the shared aggregation conformance suite.
// Elastic wires the same suite behind the `integration` build tag.
func TestAggConformance_SQLite(t *testing.T) {
	aggstest.RunConformance(t, &sqliteAggstestBackend{})
}

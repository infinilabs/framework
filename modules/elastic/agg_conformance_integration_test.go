//go:build integration

/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"testing"

	"infini.sh/framework/core/aggregate/aggstest"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/elastic/common"
)

// elasticAggstestBackend adapts ElasticORM to the aggregation conformance
// suite. Requires a live cluster (the system cluster from elastic.* config):
//
//	go test -tags integration ./modules/elastic/ -run TestAggConformance_Elastic
type elasticAggstestBackend struct {
	handler *ElasticORM
	index   string
}

func (e *elasticAggstestBackend) Setup(t *testing.T, docs []aggstest.Doc) (*orm.Context, func()) {
	t.Helper()
	client, err := common.GetElasticClient(elastic.GlobalSystemElasticsearchID)
	if err != nil || client == nil {
		t.Skipf("no live elasticsearch available: %v", err)
	}
	e.handler = &ElasticORM{Client: client}
	e.index = "aggstest-conformance"

	// Create the index with explicit mappings: blind indexing lets dynamic
	// mapping turn keyword-ish fields into text, and aggregations on text
	// fields are rejected by ES/Easysearch by default.
	_ = client.DeleteIndex(e.index)
	if err := client.CreateIndex(e.index, map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"ts":       map[string]interface{}{"type": "date"},
				"stream":   map[string]interface{}{"type": "keyword"},
				"severity": map[string]interface{}{"type": "keyword"},
				"n":        map[string]interface{}{"type": "integer"},
			},
		},
	}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	for _, d := range docs {
		id, _ := d["id"].(string)
		if _, err := client.Index(e.index, "", id, d, ""); err != nil {
			t.Fatalf("seed doc: %v", err)
		}
	}
	if err := client.Refresh(e.index); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	t.Cleanup(func() {
		_ = client.DeleteIndex(e.index)
	})

	ctx := orm.NewContext()
	orm.WithModel(ctx, &aggstestDoc{})
	orm.WithIndices(ctx, e.index)
	return ctx, func() {}
}

// aggstestDoc maps the suite fixture fields for index resolution.
type aggstestDoc struct {
	orm.ORMObjectBase
	TS       string `json:"ts,omitempty" elastic_mapping:"ts: { type: date }"`
	Stream   string `json:"stream,omitempty" elastic_mapping:"stream: { type: keyword }"`
	Severity string `json:"severity,omitempty" elastic_mapping:"severity: { type: keyword }"`
	N        int    `json:"n,omitempty" elastic_mapping:"n: { type: integer }"`
}

func (e *elasticAggstestBackend) Aggregate(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.AggregationResult, error) {
	return e.handler.Aggregate(ctx, qb)
}

func TestAggConformance_Elastic(t *testing.T) {
	aggstest.RunConformance(t, &elasticAggstestBackend{})
}

// TestAggParity_SQLite_Elastic lives in the sqlite package
// (aggregate_parity_integration_test.go) — it needs both backends and the
// sqlite package is the cycle-free meeting point.

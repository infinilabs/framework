//go:build integration

/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"testing"
	"time"

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

	for _, d := range docs {
		id, _ := d["id"].(string)
		if _, err := client.Index(e.index, "", id, d, ""); err != nil {
			t.Fatalf("seed doc: %v", err)
		}
	}
	time.Sleep(1500 * time.Millisecond) // refresh window
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

// TestAggParity_SQLite_Elastic deep-compares the two backends (needs sqlite
// plus a live cluster). Enable with the `integration` tag.
func TestAggParity_SQLite_Elastic(t *testing.T) {
	t.Skip("wired in the logpilot/framework CI once a live cluster fixture exists; see plan P6.1")
}

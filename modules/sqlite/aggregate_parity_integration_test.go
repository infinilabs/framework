package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"infini.sh/framework/core/aggregate/aggstest"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/orm/ormtest"
	elasticmod "infini.sh/framework/modules/elastic"
	"infini.sh/framework/modules/elastic/common"
)


// elasticAggstestBackend adapts ElasticORM to the conformance suite for the
// parity run (the canonical elastic-side copy lives in modules/elastic;
// duplicated here because parity needs both backends and only this package
// can import elastic without a cycle).
type elasticAggstestBackend struct {
	handler *elasticmod.ElasticORM
	index   string
}

func (e *elasticAggstestBackend) Setup(t *testing.T, docs []aggstest.Doc) (*orm.Context, func()) {
	t.Helper()
	client, err := common.GetElasticClient(elastic.GlobalSystemElasticsearchID)
	if err != nil || client == nil {
		t.Skipf("no live elasticsearch available: %v", err)
	}
	e.handler = &elasticmod.ElasticORM{Client: client}
	e.index = "aggstest-parity"

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
	t.Cleanup(func() { _ = client.DeleteIndex(e.index) })

	ctx := orm.NewContext()
	orm.WithIndices(ctx, e.index)
	return ctx, func() {}
}

func (e *elasticAggstestBackend) Aggregate(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.AggregationResult, error) {
	return e.handler.Aggregate(ctx, qb)
}

// TestAggParity_SQLite_Elastic deep-compares the two backends against the
// shared conformance suite. CI wires it via the integration workflow's
// ES_ENDPOINT fixture (the system cluster is seeded by the elastic
// package's integration TestMain); skips when no cluster is reachable.
func TestAggParity_SQLite_Elastic(t *testing.T) {
	seedParityORM(t)
	if err := ormtest.SeedSystemCluster(); err != nil {
		t.Skipf("no live cluster fixture: %v", err)
	}
	aggstest.RunParity(t, &sqliteAggstestBackend{}, &elasticAggstestBackend{})
}


// seedParityORM registers the sqlite ORM handler over a temp database for
// the parity run's cluster-record persistence.
func seedParityORM(t *testing.T) {
	t.Helper()
	handler := &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(os.TempDir(), "orm-parity-test.db"),
	}}
	if err := handler.Open(); err != nil {
		t.Fatal(err)
	}
	if err := handler.RegisterSchemaWithName(elastic.ElasticsearchConfig{}, "cluster"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recover() }()
	orm.Register("sqlite-parity", handler)
}

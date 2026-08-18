//go:build integration

/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/orm/ormtest"
	"infini.sh/framework/modules/elastic/common"
)

// TestContract_Elastic runs the shared backend contract suite against a live
// cluster (the system cluster from the elastic.* config). Enable with:
//
//	go test -tags integration ./modules/elastic/ -run TestContract_Elastic
func TestContract_Elastic(t *testing.T) {
	client, err := common.GetElasticClient(elastic.GlobalSystemElasticsearchID)
	if err != nil || client == nil {
		t.Skipf("no live elasticsearch available for contract tests: %v", err)
	}
	// Fresh index with explicit mappings: drop leftovers from a previous run
	// (fixed IDs would hit version conflicts) and prevent dynamic mapping from
	// turning keyword fields into text (aggregations on text are rejected).
	_ = client.DeleteIndex("contractmodel")
	if err := client.CreateIndex("contractmodel", map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"name":   map[string]interface{}{"type": "keyword"},
				"status": map[string]interface{}{"type": "keyword"},
				"body":   map[string]interface{}{"type": "text"},
				"age":    map[string]interface{}{"type": "integer"},
				"created": map[string]interface{}{"type": "date"},
			},
		},
	}); err != nil {
		t.Fatalf("create contractmodel index: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteIndex("contractmodel") })
	ormtest.RunContractTests(t, func() orm.ORM {
		return &ElasticORM{Client: client}
	})
}

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
	ormtest.RunContractTests(t, func() orm.ORM {
		return &ElasticORM{Client: client}
	})
}

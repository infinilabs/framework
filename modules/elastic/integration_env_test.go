//go:build integration

/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"os"
	"path/filepath"
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/orm/ormtest"
	"infini.sh/framework/modules/sqlite"
)

// TestMain bootstraps the ORM handler + system-cluster record from CI's ES_*
// variables so the integration suites resolve a live cluster via
// common.GetElasticClient(GlobalSystemElasticsearchID). Without ES_ENDPOINT
// the suites skip gracefully (developer laptops).
func TestMain(m *testing.M) {
	_ = seedORMBackend()
	if true {
		if err := ormtest.SeedSystemCluster(); err != nil && err != ormtest.ErrNoEndpoint {
			println("integration bootstrap failed (tests will skip):", err.Error())
		}
	}
	os.Exit(m.Run())
}

// seedORMBackend registers a sqlite handler over a temp database (the
// cluster record must be persistable somewhere; sqlite keeps the fixture
// self-contained).
func seedORMBackend() error {
	handler := &sqlite.SQLiteORM{
		Config: sqlite.SQLiteConfig{
			Enabled: true,
			DBPath:  filepath.Join(os.TempDir(), "orm-integration-test.db"),
		},
	}
	if err := handler.Open(); err != nil {
		return err
	}
	if err := handler.RegisterSchemaWithName(elastic.ElasticsearchConfig{}, "cluster"); err != nil {
		return err
	}
	defer func() { _ = recover() }() // duplicate registration is fine
	orm.Register("sqlite-integration", handler)
	return nil
}

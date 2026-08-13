/* Copyright © INFINI LTD. All rights reserved. */

package easysearch

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

// Module owns cluster management (the /easysearch/ REST API) as a clean,
// ORM-backed data layer, decoupled from the elasticsearch module's live-client
// and health machinery. Apps opt in by registering it.
//
// It does NOT register live ES clients or read the in-memory metadata
// registry — cluster records become live clients when the elasticsearch module
// loads them (LoadClustersFromORM) on its own boot/reload cycle. Unifying live
// registration is a later refactor.
type Module struct {
	moduleConfig moduleConfig
}

type moduleConfig struct {
	Enabled bool `json:"enabled" config:"enabled"`
}

func (m *Module) Name() string { return "easysearch" }

func (m *Module) Setup() {

	exists, err := env.ParseConfig("easysearch", &m.moduleConfig)
	if exists && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	if !m.moduleConfig.Enabled {
		return
	}

	// Register the cluster schema. This module owns it now (it was previously
	// registered inside modules/elastic). The table is materialized later by
	// the ORM backend's InitSchema() (e.g. sqlite), which runs after every
	// module's Setup().
	if err := orm.RegisterSchemaWithIndexName(elastic.ElasticsearchConfig{}, "cluster"); err != nil {
		log.Warnf("easysearch: register cluster schema: %v", err)
	}
	registerClusterAPI()
}

func (m *Module) Start() error {

	if !m.moduleConfig.Enabled {
		return nil
	}

	return nil
}
func (m *Module) Stop() error { return nil }

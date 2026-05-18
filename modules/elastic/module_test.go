package elastic

import (
	"testing"

	coreElastic "infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
)

func TestLoadESBasedElasticConfigSkipsWhenSystemClusterUnavailable(t *testing.T) {
	previous := global.Lookup(coreElastic.GlobalSystemElasticsearchID)
	defer global.Register(coreElastic.GlobalSystemElasticsearchID, previous)

	global.Register(coreElastic.GlobalSystemElasticsearchID, "")

	configs := loadESBasedElasticConfig()
	if len(configs) != 0 {
		t.Fatalf("expected no remote configs when system cluster id is unavailable, got %d", len(configs))
	}
}

func TestElasticModuleStartSkipsSystemClusterDependentInitBeforeSetup(t *testing.T) {
	previousSystemID := global.Lookup(coreElastic.GlobalSystemElasticsearchID)
	defer global.Register(coreElastic.GlobalSystemElasticsearchID, previousSystemID)

	previousModuleConfig := moduleConfig
	defer func() {
		moduleConfig = previousModuleConfig
	}()

	previousOrmInited := ormInited
	defer func() {
		ormInited = previousOrmInited
	}()

	global.Register(coreElastic.GlobalSystemElasticsearchID, "")

	moduleConfig = getDefaultConfig()
	moduleConfig.ORMConfig.Enabled = true
	moduleConfig.StoreConfig.Enabled = true
	moduleConfig.RemoteConfigEnabled = true
	ormInited = false

	module := &ElasticModule{}
	if err := module.Start(); err != nil {
		t.Fatalf("expected elastic module start to succeed before setup, got %v", err)
	}
}

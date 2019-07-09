/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elastic

import (
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/elastic"
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/errors"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/kv"
	"github.com/infinitbyte/framework/core/orm"
	"github.com/infinitbyte/framework/modules/elastic/adapter"
	"strings"
)

func (module ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = ModuleConfig{
		Elasticsearch: "default",
	}
)

func getDefaultConfig() ModuleConfig {
	return defaultConfig
}

type ModuleConfig struct {
	IndexerEnabled bool   `config:"indexer_enabled"`
	StoreEnabled   bool   `config:"store_enabled"`
	ORMEnabled     bool   `config:"orm_enabled"`
	Elasticsearch  string `config:"elasticsearch"`
}

var m = map[string]elastic.ElasticsearchConfig{}

func loadElasticConfig() {

	var configs []elastic.ElasticsearchConfig
	exist, err := env.ParseConfig("elasticsearch", &configs)
	if err != nil {
		panic(err)
	}
	if exist {
		for _, v := range configs {
			if v.ID == "" {
				if v.Name == "" {
					panic(errors.Errorf("invalid elasticsearch config, %v", v))
				}
				v.ID = v.Name
			}
			m[v.ID] = v

		}
	}
}

func initElasticInstances() {

	for k, esConfig := range m {

		var client elastic.API

		esVersion, err := adapter.ClusterVersion(&esConfig)
		if err != nil {
			panic(err)
			return
		}
		if global.Env().IsDebug {
			log.Debug("elasticsearch version: ", esVersion.Version.Number)
		}

		if strings.HasPrefix(esVersion.Version.Number, "7.") {
			api := new(adapter.ESAPIV7)
			api.Config = &esConfig
			client = api
		} else if strings.HasPrefix(esVersion.Version.Number, "6.") {
			api := new(adapter.ESAPIV6)
			api.Config = &esConfig
			client = api
		} else if strings.HasPrefix(esVersion.Version.Number, "5.") {
			api := new(adapter.ESAPIV5)
			api.Config = &esConfig
			client = api
		} else {
			api := new(adapter.ESAPIV0)
			api.Config = &esConfig
			client = api
		}
		elastic.RegisterInstance(k, client)
	}

}

func (module ElasticModule) Setup(cfg *config.Config) {

	loadElasticConfig()

	initElasticInstances()

	moduleConfig := getDefaultConfig()
	cfg.Unpack(&moduleConfig)

	client := elastic.GetClient(moduleConfig.Elasticsearch)

	if moduleConfig.ORMEnabled {
		handler := ElasticORM{Client: client}
		handler.Client.Init()
		orm.Register("elastic", handler)
	}

	if moduleConfig.StoreEnabled {
		handler := ElasticStore{Client: client}
		kv.Register("elastic", handler)
	}

	if moduleConfig.IndexerEnabled {
		module.indexer = &ElasticIndexer{client: client, indexChannel: "index"}
	}

}

func (module ElasticModule) Stop() error {
	if module.indexer != nil {
		module.indexer.Stop()
	}
	return nil

}

func (module ElasticModule) Start() error {

	if module.indexer != nil {
		module.indexer.Start()
	}
	return nil

}

type ElasticModule struct {
	indexer *ElasticIndexer
}

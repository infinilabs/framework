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
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/task"
	"infini.sh/framework/modules/elastic/adapter"
	"strings"
)

func (module ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = ModuleConfig{
		Elasticsearch: "default",
		InitTemplate:  false,
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
	InitTemplate   bool   `config:"init_template"`
}

var indexer *ElasticIndexer

var m = map[string]elastic.ElasticsearchConfig{}

func loadElasticConfig() {

	var configs []elastic.ElasticsearchConfig
	exist, err := env.ParseConfig("elasticsearch", &configs)
	if exist && err != nil {
		panic(err)
	}

	if exist {
		for _, v := range configs {
			if !v.Enabled {
				log.Debug("elasticsearch ", v.Name, " is not enabled")
				continue
			}
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
		if !esConfig.Enabled {
			log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
			continue
		}

		var ver string
		if esConfig.Version == "" || esConfig.Version == "auto" {
			esVersion, err := adapter.ClusterVersion(&esConfig)
			if err != nil {
				panic(err)
				return
			}
			ver = esVersion.Version.Number
			esConfig.Version = ver
		} else {
			ver = esConfig.Version
		}

		if global.Env().IsDebug {
			log.Debug("elasticsearch version: ", ver)
		}

		if strings.HasPrefix(ver, "8.") {
			api := new(adapter.ESAPIV8)
			api.Config = esConfig
			api.Version = ver
			client = api
		} else if strings.HasPrefix(ver, "7.") {
			api := new(adapter.ESAPIV7)
			api.Config = esConfig
			api.Version = ver
			client = api
		} else if strings.HasPrefix(ver, "6.") {
			api := new(adapter.ESAPIV6)
			api.Config = esConfig
			api.Version = ver
			client = api
		} else if strings.HasPrefix(ver, "5.") {
			api := new(adapter.ESAPIV5)
			api.Config = esConfig
			api.Version = ver
			client = api
		} else if strings.HasPrefix(ver, "2.") {
			api := new(adapter.ESAPIV2)
			api.Config = esConfig
			api.Version = ver
			client = api
		} else {
			api := new(adapter.ESAPIV0)
			api.Config = esConfig
			api.Version = ver
			client = api
		}
		elastic.RegisterInstance(k, esConfig, client)
	}

}

func (module ElasticModule) Init() {

	loadElasticConfig()

	initElasticInstances()

}

func (module ElasticModule) Setup(cfg *config.Config) {

	module.Init()

	moduleConfig := getDefaultConfig()
	if !cfg.Enabled(false) {
		return
	}

	err := cfg.Unpack(&moduleConfig)
	if err != nil {
		panic(err)
	}

	client := elastic.GetClient(moduleConfig.Elasticsearch)
	if moduleConfig.InitTemplate {
		client.Init()
	}

	if moduleConfig.ORMEnabled {
		handler := ElasticORM{Client: client}
		orm.Register("elastic", handler)
	}

	if moduleConfig.StoreEnabled {
		handler := ElasticStore{Client: client}
		kv.Register("elastic", handler)
	}

	if moduleConfig.IndexerEnabled {
		indexer = &ElasticIndexer{client: client, indexChannel: "index"}
	}

}

func (module ElasticModule) Stop() error {
	if indexer != nil {
		indexer.Stop()
	}
	return nil

}

func discovery() {
	all := elastic.GetAllConfigs()
	for _, cfg := range all {
		if cfg.Discovery.Enabled {
			client := elastic.GetClient(cfg.Name)

			nodes, err := client.GetNodes()
			if err != nil {
				log.Error(err)
				continue
			}
			if len(nodes.Nodes) <= 0 {
				continue
			}

			oldMetadata := elastic.GetMetadata(cfg.Name)
			newMetadata := elastic.ElasticsearchMetadata{}

			//Nodes
			//if util.ContainsAnyInArray("nodes", cfg.Discovery.Modules) {
			var nodesChanged = false
			var oldNodesTopologyVersion = 0
			if oldMetadata == nil {
				nodesChanged = true
			} else {
				oldNodesTopologyVersion = oldMetadata.NodesTopologyVersion
				newMetadata.NodesTopologyVersion=oldNodesTopologyVersion
				newMetadata.Nodes=oldMetadata.Nodes

				if len(nodes.Nodes) != len(oldMetadata.Nodes) {
					nodesChanged = true
				} else {
					for k, v := range nodes.Nodes {
						v1, ok := oldMetadata.Nodes[k]
						if ok {
							if v.Http.PublishAddress != v1.Http.PublishAddress {
								nodesChanged = true
							}
						} else {
							nodesChanged = true
							break
						}
					}
				}
			}

			if nodesChanged{
				newMetadata.NodesTopologyVersion = oldNodesTopologyVersion + 1
				newMetadata.Nodes = nodes.Nodes
			}

			var indicesChanged bool
			//Indices
			indices,err:=client.GetIndices()
			if err!=nil{
				panic(err)
			}
			if indices!=nil{
				//TODO check if that changed or skip replace
				newMetadata.Indices=*indices
				indicesChanged=true
			}
			var shardsChanged bool

			//Shards
			shards,err:=client.GetPrimaryShards()
			if err!=nil{
				panic(err)
			}
			if shards!=nil{
				//TODO check if that changed or skip replace
				newMetadata.PrimaryShards=*shards
				shardsChanged=true
			}

			if nodesChanged ||indicesChanged||shardsChanged {
				log.Trace("elasticsearch newMetadata updated,", nodes.Nodes)
				elastic.SetMetadata(cfg.Name, &newMetadata)
			}

		}
	}
}

func (module ElasticModule) Start() error {

	t := task.ScheduleTask{
		Description: "discovery nodes topology",
		Type:        "interval",
		Interval:    "10s",
		Task:        discovery,
	}
	task.RegisterScheduleTask(t)

	discovery()

	if indexer != nil {
		indexer.Start()
	}
	return nil

}

type ElasticModule struct {
}

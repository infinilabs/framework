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
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/api"
	. "infini.sh/framework/modules/elastic/common"
	"sync"
)

func (module *ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = ModuleConfig{
		RemoteConfigEnabled: false,
		HealthCheckConfig: CheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		NodeAvailabilityCheckConfig: CheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		MetadataRefresh: CheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		ORMConfig: ORMConfig{
			Enabled:      false,
			InitTemplate: true,
			IndexPrefix:  ".infini-",
		},
		StoreConfig: StoreConfig{
			Enabled: false,
		},
		ClusterSettingsCheckConfig: CheckConfig{
			Enabled: true,
			Interval: "20s",
		},
		ClientTimeout: "60s",
	}
)

func getDefaultConfig() ModuleConfig {
	return defaultConfig
}

var m = map[string]elastic.ElasticsearchConfig{}

func loadFileBasedElasticConfig() {
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
			v.Source = "file"
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

func loadESBasedElasticConfig() {
	configs := []elastic.ElasticsearchConfig{}
	query := elastic.SearchRequest{From: 0, Size: 1000} //TODO handle clusters beyond 1000
	result, err := elastic.GetClient(moduleConfig.Elasticsearch).Search(orm.GetIndexName(elastic.ElasticsearchConfig{}), &query)
	//log.Errorf("%v %v",query,result)
	if err != nil {
		log.Error(err)
		return
	}

	if len(result.Hits.Hits) > 0 {
		for _, v1 := range result.Hits.Hits {
			cfg := elastic.ElasticsearchConfig{}
			bytes := util.MustToJSONBytes(v1.Source)
			util.MustFromJSONBytes(bytes, &cfg)
			cfg.ID = v1.ID
			cfg.Discovery.Enabled = true
			configs = append(configs, cfg)
		}

		for _, v := range configs {
			if !v.Enabled {
				log.Debug("elasticsearch ", v.Name, " is not enabled")
				continue
			}
			v.Source = "elastic"
			if v.ID == "" {
				if v.Name == "" {
					log.Errorf("invalid elasticsearch config, %v", v)
					continue
				}
				v.ID = v.Name
			}
			m[v.ID] = v
		}
	}

	log.Infof("loading [%v] remote elasticsearch configs",len(result.Hits.Hits) )

}

func initElasticInstances() {
	for k, esConfig := range m {

		log.Trace("init elasticsearch ", esConfig.Name,esConfig.Enabled)

		if !esConfig.Enabled {
			log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
			continue
		}
		client, err := InitClientWithConfig(esConfig)
		if err != nil {
			log.Error("elasticsearch ", esConfig.Name, err)
			continue
		}
		elastic.RegisterInstance(k, esConfig, client)
	}
}

var moduleConfig = ModuleConfig{}

func (module *ElasticModule) Setup(cfg *config.Config) {

	loadFileBasedElasticConfig()

	initElasticInstances()

	moduleConfig = getDefaultConfig()

	exists, err := env.ParseConfig("elastic", &moduleConfig)

	if exists && err != nil {
		panic(err)
	}

	if moduleConfig.ORMConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		if moduleConfig.ORMConfig.InitTemplate {
			client.InitDefaultTemplate(moduleConfig.ORMConfig.TemplateName, moduleConfig.ORMConfig.IndexPrefix)
		}
		handler := ElasticORM{Client: client, Config: moduleConfig.ORMConfig}
		orm.Register("elastic", handler)

		err = orm.RegisterSchemaWithIndexName(elastic.ElasticsearchConfig{}, "cluster")
		if err != nil {
			panic(err)
		}
		err = orm.RegisterSchemaWithIndexName(elastic.NodeConfig{}, "node")
		if err != nil {
			panic(err)
		}
		err = orm.RegisterSchemaWithIndexName(elastic.IndexConfig{}, "index")
		if err != nil {
			panic(err)
		}

		err = orm.RegisterSchemaWithIndexName(event.Event{}, "metrics")
		if err != nil {
			panic(err)
		}
		err = orm.RegisterSchemaWithIndexName(event.Activity{}, "activities")
		if err != nil {
			panic(err)
		}
	}

	if moduleConfig.StoreConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		module.storeHandler= &ElasticStore{Client: client, Config: moduleConfig.StoreConfig}
		kv.Register("elastic", module.storeHandler)
	}

	api.Init(moduleConfig)
}

func (module *ElasticModule) Stop() error {
	return nil
}

func nodeAvailabilityCheck() {

	task2 := task.ScheduleTask{
		Description: "check for elasticsearch node availability",
		Type:        "interval",
		Interval:    "10s",
		Task: func(ctx context.Context) {
			elastic.WalkHosts(func(key, value interface{}) bool {
				k := key.(string)

				if value == nil {
					return true
				}

				v, ok := value.(*elastic.NodeAvailable)
				if ok {
					log.Trace("check availability for node: " + k)
					avail := util.TestTCPAddress(k)
					if global.Env().IsDebug{
						log.Debugf("availability for node [%v] : %v",k,avail)
					}

					if avail {
						v.ReportSuccess()
					} else {
						v.ReportFailure()
					}
					log.Tracef("node [%v], connection available: [%v] [%v]", k, avail, v.IsAvailable())
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)
}

func (module *ElasticModule)clusterStateRefresh() {
	var mutexMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch state refresh",
		Type:        "interval",
		Interval:    moduleConfig.MetadataRefresh.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				log.Trace("walk metadata: ",key)

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init meta refresh task: [%v] [%v] [%v] [%v]",key,v.ID,v.Name,v.Enabled)

				if ok {
					if !v.Enabled{
						return true
					}
					if busy, ok := mutexMap.Load(v.ID); ok && busy == true{
						log.Warnf("cluster state refresh of cluster %s is busy", v.Name)
						return true
					}
					mutexMap.Store(v.ID, true)
					go func(clusterID string) {
						module.updateClusterState(clusterID)
						mutexMap.Store(clusterID, false)
					}(v.ID)
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)

}

func (module *ElasticModule) Start() error {

	if moduleConfig.RemoteConfigEnabled {
		loadESBasedElasticConfig()
	}

	initElasticInstances()

	if module.storeHandler!=nil{
		err:=module.storeHandler.Open()
		if err!=nil{
			panic(err)
		}
	}
	elastic.WalkConfigs(func(key, value interface{}) bool {
		cfg1, ok := value.(*elastic.ElasticsearchConfig)
		if ok && cfg1 != nil {
			log.Tracef("init task walk configs: %v", cfg1.Name)

			go clusterHealthCheck( cfg1.ID, true)
		}
		return true
	})

	if moduleConfig.HealthCheckConfig.Enabled {
		healthCheckLoading := sync.Map{}
		t := task.ScheduleTask{
			Description: "cluster health check",
			Type:        "interval",
			Interval:    moduleConfig.HealthCheckConfig.Interval,
			Task: func(ctx context.Context) {
				elastic.WalkConfigs(func(key, value interface{}) bool {
					cfg1, ok := value.(*elastic.ElasticsearchConfig)
					if ok && cfg1 != nil {
						log.Tracef("init task walk configs: %v", cfg1.Name)

						if busy, ok := healthCheckLoading.Load(cfg1.ID); ok && busy == true{
							log.Debugf("previous health check of cluster %v is still running", cfg1.Name)
							return true
						}
						healthCheckLoading.Store(cfg1.ID, true)
						go func(clusterID string) {
							clusterHealthCheck( clusterID, false)
							healthCheckLoading.Store(clusterID, false)
						}(cfg1.ID)
					}
					return true
				})
			},
		}

		task.RegisterScheduleTask(t)
	}

	if moduleConfig.NodeAvailabilityCheckConfig.Enabled {
		nodeAvailabilityCheck()
	}

	log.Tracef("metadata refresh enabled:%v",moduleConfig.MetadataRefresh.Enabled)

	if moduleConfig.MetadataRefresh.Enabled {
		//refresh cluster state
		module.clusterStateRefresh()

		//refresh nodes
		task2 := task.ScheduleTask{
			Description: fmt.Sprintf("elasticsearch nodes discovery"),
			Type:        "interval",
			Interval:    "60s",
			Task: func(ctx context.Context) {
				elastic.WalkMetadata(func(key, value interface{}) bool {
					if value==nil{
						return true
					}

					v,ok:=value.(*elastic.ElasticsearchMetadata)
					if ok{
						if v.Config.Discovery.Enabled{
							module.updateNodeInfo(v)
						}
					}
					return true
				})
			},
		}
		task.RegisterScheduleTask(task2)


		////refresh indices
		//task2 = task.ScheduleTask{
		//	Description: fmt.Sprintf("elasticsearch indices discovery"),
		//	Type:        "interval",
		//	Interval:    "30s",
		//	Task: func() {
		//		elastic.WalkMetadata(func(key, value interface{}) bool {
		//			if value==nil{
		//				return true
		//			}
		//			v,ok:=value.(*elastic.ElasticsearchMetadata)
		//			if ok{
		//				updateIndices(v)
		//			}
		//			return true
		//		})
		//	},
		//}
		//task.RegisterScheduleTask(task2)

		//refresh index alias
		task2 = task.ScheduleTask{
			Description: fmt.Sprintf("elasticsearch alias discovery"),
			Type:        "interval",
			Interval:    "30s",
			Task: func(ctx context.Context) {
				elastic.WalkMetadata(func(key, value interface{}) bool {
					if value==nil{
						return true
					}
					v,ok:=value.(*elastic.ElasticsearchMetadata)
					if ok{
						updateAliases(v)
					}
					return true
				})
			},
		}
		task.RegisterScheduleTask(task2)

		////refresh primary_shards
		//task2 = task.ScheduleTask{
		//	Description: fmt.Sprintf("elasticsearch shards discovery"),
		//	Type:        "interval",
		//	Interval:    "30s",
		//	Task: func() {
		//		elastic.WalkMetadata(func(key, value interface{}) bool {
		//			if value==nil{
		//				return true
		//			}
		//			v,ok:=value.(*elastic.ElasticsearchMetadata)
		//			if ok{
		//				updateShards(v)
		//			}
		//			return true
		//		})
		//	},
		//}
		//task.RegisterScheduleTask(task2)
	}

	if moduleConfig.ClusterSettingsCheckConfig.Enabled {
		module.clusterSettingsRefresh()
	}
	return nil

}

func (module *ElasticModule)clusterSettingsRefresh() {
	var mutexMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch settings refresh",
		Type:        "interval",
		Interval:    moduleConfig.ClusterSettingsCheckConfig.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				log.Trace("walk metadata: ",key)

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init settings refresh task: [%v] [%v] [%v] [%v]",key,v.ID,v.Name,v.Enabled)

				if ok {
					if !v.Enabled{
						return true
					}
					if busy, ok := mutexMap.Load(v.ID); ok && busy == true{
						log.Warnf("cluster settings collecting for cluster %s is still running", v.Name)
						return true
					}
					mutexMap.Store(v.ID, true)
					go func(clusterID string) {
						module.updateClusterSettings(clusterID)
						mutexMap.Store(clusterID, false)
					}(v.ID)
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)

}

type ElasticModule struct {
	storeHandler *ElasticStore
}

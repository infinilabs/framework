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
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/api"
	. "infini.sh/framework/modules/elastic/common"
	"math"
	"runtime"
	"sync"
	"time"
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
			Enabled:  true,
			Interval: "20s",
		},
		ClientTimeout: "60s",
	}
)

func getDefaultConfig() ModuleConfig {
	return defaultConfig
}

func loadFileBasedElasticConfig() []elastic.ElasticsearchConfig {
	var configs []elastic.ElasticsearchConfig
	exist, err := env.ParseConfig("elasticsearch", &configs)
	if exist && err != nil {
		panic(err)
	}

	if exist {
		for _, v := range configs {
			v.Source = "file"
			if !v.Enabled {
				log.Debug("elasticsearch ", v.Name, " is not enabled")
				continue
			}
		}
	}
	return configs
}

func loadESBasedElasticConfig() []elastic.ElasticsearchConfig {
	configs := []elastic.ElasticsearchConfig{}
	query := elastic.SearchRequest{From: 0, Size: 1000} //TODO handle clusters beyond 1000
	result, err := elastic.GetClient(moduleConfig.Elasticsearch).Search(orm.GetIndexName(elastic.ElasticsearchConfig{}), &query)
	if err != nil {
		log.Error(err)
		return configs
	}

	if len(result.Hits.Hits) > 0 {
		for _, v1 := range result.Hits.Hits {
			cfg := elastic.ElasticsearchConfig{}
			bytes := util.MustToJSONBytes(v1.Source)
			util.MustFromJSONBytes(bytes, &cfg)
			cfg.ID = v1.ID
			configs = append(configs, cfg)
		}
	}

	log.Infof("loading [%v] remote elasticsearch configs", len(result.Hits.Hits))
	return configs
}

func initElasticInstances(m []elastic.ElasticsearchConfig, source string) {
	for _, esConfig := range m {
		esConfig.Source = source
		if esConfig.ID == "" && esConfig.Name != "" {
			esConfig.ID = esConfig.Name
		}
		log.Trace("init elasticsearch ", esConfig.Name, ", enabled:", esConfig.Enabled)
		if !esConfig.Enabled {
			log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
			continue
		}
		client, err := InitClientWithConfig(esConfig)
		if err != nil {
			log.Error("elasticsearch ", esConfig.Name, err)
			continue
		}
		elastic.RegisterInstance(esConfig, client)
	}
}

var moduleConfig = ModuleConfig{}

func (module *ElasticModule) Setup(cfg *config.Config) {

	moduleConfig = getDefaultConfig()

	exists, err := env.ParseConfig("elastic", &moduleConfig)

	if exists && err != nil {
		panic(err)
	}
	m := loadFileBasedElasticConfig()
	initElasticInstances(m, "file")

	if moduleConfig.ORMConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		handler := ElasticORM{Client: client, Config: moduleConfig.ORMConfig}
		orm.Register("elastic", handler)
	}

	if moduleConfig.StoreConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		module.storeHandler = &ElasticStore{Client: client, Config: moduleConfig.StoreConfig}
		kv.Register("elastic", module.storeHandler)
	}

	api.Init(moduleConfig)
}

func (module *ElasticModule) Stop() error {
	return nil
}

func nodeAvailabilityCheck() {
	availabilityMap := sync.Map{}
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
					if v.ClusterID==""{
						return true
					}

					cfg := elastic.GetConfig(v.ClusterID)
					if !cfg.Enabled || (cfg.MetadataConfigs !=nil && !cfg.MetadataConfigs.NodeAvailabilityCheck.Enabled){
						return true
					}

					if startTime, ok := availabilityMap.Load(k); ok {
						elapsed := time.Since(startTime.(time.Time))
						interval := moduleConfig.NodeAvailabilityCheckConfig.Interval
						if cfg.MetadataConfigs != nil && cfg.MetadataConfigs.NodeAvailabilityCheck.Interval != "" {
							interval = cfg.MetadataConfigs.NodeAvailabilityCheck.Interval
						}
						if time.Since(startTime.(time.Time)) > util.GetDurationOrDefault(interval, 10*time.Second)*2 {
							log.Warnf("check availability for node [%s] is still running, elapsed: %v, skip waiting", v.Host, elapsed.String())
						} else {
							log.Warnf("check availability for node [%s] is still running, elapsed: %v", v.Host, elapsed.String())
							return true
						}
					}
					availabilityMap.Store(k, time.Now())

					log.Trace("check availability for node: " + k)
					avail := util.TestTCPAddress(k,10*time.Second)
					if global.Env().IsDebug {
						log.Tracef("availability for node [%v] : %v", k, avail)
					}

					if avail {
						v.ReportSuccess()
					} else {
						v.ReportFailure()
					}
					log.Tracef("node [%v], connection available: [%v] [%v]", k, avail, v.IsAvailable())
					availabilityMap.Delete(k)
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)
}

func (module *ElasticModule) clusterStateRefresh() {
	module.stateMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch state refresh",
		Type:        "interval",
		Interval:    moduleConfig.MetadataRefresh.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				log.Trace("walk metadata: ", key)

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init meta refresh task: [%v] [%v] [%v] [%v]", key, v.ID, v.Name, v.Enabled)

				if ok {
					if !v.Enabled || (v.MetadataConfigs !=nil && !v.MetadataConfigs.MetadataRefresh.Enabled){
						return true
					}

					if startTime, ok := module.stateMap.Load(v.ID); ok {
						elapsed := time.Since(startTime.(time.Time))
						interval := moduleConfig.MetadataRefresh.Interval
						if v.MetadataConfigs != nil && v.MetadataConfigs.MetadataRefresh.Interval != "" {
							interval = v.MetadataConfigs.MetadataRefresh.Interval
						}
						intervalD :=  util.GetDurationOrDefault(interval, 10*time.Second)
						if time.Since(startTime.(time.Time)) > intervalD *2 {
							log.Warnf("refresh cluster state for cluster [%s] is still running, elapsed: %v, skip waiting", v.Name, elapsed.String())
						} else {
							duration := elapsed - intervalD
							abd := math.Abs(duration.Seconds())
							if abd> 3 {
								log.Warnf("refresh cluster state for cluster [%s] is still running, elapsed: %v", v.Name, elapsed.String())
								return true
							}
						}
					}
					module.stateMap.Store(v.ID, time.Now())
					go func(clusterID string) {
						module.updateClusterState(clusterID)
						module.stateMap.Delete(clusterID)
					}(v.ID)
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)
}

func (module *ElasticModule) Start() error {

	if moduleConfig.ORMConfig.Enabled {
		if moduleConfig.ORMConfig.InitTemplate {
			client := elastic.GetClient(moduleConfig.Elasticsearch)
			client.InitDefaultTemplate(moduleConfig.ORMConfig.TemplateName, moduleConfig.ORMConfig.IndexPrefix)
		}

		err := orm.RegisterSchemaWithIndexName(elastic.ElasticsearchConfig{}, "cluster")
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
	if moduleConfig.RemoteConfigEnabled {
		m := loadESBasedElasticConfig()
		initElasticInstances(m, "elastic")
	}

	if module.storeHandler != nil {
		err := module.storeHandler.Open()
		if err != nil {
			panic(err)
		}
	}

	//init elasticsearch
	elastic.WalkConfigs(func(key, value interface{}) bool {
		cfg1, ok := value.(*elastic.ElasticsearchConfig)
		if ok && cfg1 != nil {
			log.Tracef("init elasticsearch config: %v", cfg1.Name)
			metadata := elastic.GetMetadata(cfg1.ID)
			if metadata != nil {
				module.updateNodeInfo(metadata, true, cfg1.Discovery.Enabled)
			}
			go module.clusterHealthCheck(cfg1.ID, true)
		}
		return true
	})

	if moduleConfig.HealthCheckConfig.Enabled {
		module.healthMap = sync.Map{}
		t := task.ScheduleTask{
			Description: "cluster health check",
			Type:        "interval",
			Interval:    moduleConfig.HealthCheckConfig.Interval,
			Task: func(ctx context.Context) {
				elastic.WalkConfigs(func(key, value interface{}) bool {
					cfg1, ok := value.(*elastic.ElasticsearchConfig)
					if ok && cfg1 != nil {
						if !cfg1.Enabled || (cfg1.MetadataConfigs !=nil && !cfg1.MetadataConfigs.HealthCheck.Enabled){
							return true
						}

						log.Tracef("init cluster health check for: %v", cfg1.Name)

						if startTime, ok := module.healthMap.Load(cfg1.ID); ok {
							elapsed := time.Since(startTime.(time.Time))
							interval := moduleConfig.HealthCheckConfig.Interval
							if cfg1.MetadataConfigs != nil && cfg1.MetadataConfigs.HealthCheck.Interval != "" {
								interval = cfg1.MetadataConfigs.HealthCheck.Interval
							}
							tinterval := util.GetDurationOrDefault(interval, 10*time.Second)
							if elapsed > tinterval*2 {
								log.Warnf("health check for cluster [%s] is still running, elapsed: %v, skip waiting", cfg1.Name, elapsed.String())
							} else if math.Abs((elapsed - tinterval).Seconds()) > 3{
								log.Warnf("health check for cluster [%s] is still running, elapsed: %v", cfg1.Name, elapsed.String())
								return true
							}
						}
						module.healthMap.Store(cfg1.ID, time.Now())
						go func(clusterID string) {
							module.clusterHealthCheck(clusterID, false)
							module.healthMap.Delete(clusterID)
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

	log.Tracef("metadata refresh enabled:%v", moduleConfig.MetadataRefresh.Enabled)

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
					if value == nil {
						return true
					}

					v, ok := value.(*elastic.ElasticsearchMetadata)
					if ok {
						module.updateNodeInfo(v, false, v.Config.Discovery.Enabled)
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
					if value == nil {
						return true
					}
					v, ok := value.(*elastic.ElasticsearchMetadata)
					if ok {
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

	config.NotifyOnConfigSectionChange("elasticsearch", func(pCfg, cCfg *config.Config) {

		defer func() {
			if !global.Env().IsDebug {
				if r := recover(); r != nil {
					var v string
					switch r.(type) {
					case error:
						v = r.(error).Error()
					case runtime.Error:
						v = r.(runtime.Error).Error()
					case string:
						v = r.(string)
					}
					log.Error("error on apply config change,", v)
				}
			}
		}()

		if cCfg != nil {
			//TODO diff previous and current config
			var newConfig []elastic.ElasticsearchConfig
			err := cCfg.Unpack(&newConfig)
			if err != nil {
				log.Error(err)
				return
			}
			initElasticInstances(newConfig, "file")
		}
	})

	return nil

}

func (module *ElasticModule) clusterSettingsRefresh() {
	module.settingsMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch settings refresh",
		Type:        "interval",
		Interval:    moduleConfig.ClusterSettingsCheckConfig.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				log.Trace("walk metadata: ", key)

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init settings refresh task: [%v] [%v] [%v] [%v]", key, v.ID, v.Name, v.Enabled)

				if ok {
					if !v.Enabled || (v.MetadataConfigs !=nil && !v.MetadataConfigs.ClusterSettingsCheck.Enabled) {
						return true
					}
					if startTime, ok := module.settingsMap.Load(v.ID); ok {
						elapsed := time.Since(startTime.(time.Time))
						interval := moduleConfig.ClusterSettingsCheckConfig.Interval
						if v.MetadataConfigs != nil && v.MetadataConfigs.ClusterSettingsCheck.Interval != "" {
							interval = v.MetadataConfigs.ClusterSettingsCheck.Interval
						}

						if time.Since(startTime.(time.Time)) > util.GetDurationOrDefault(interval, 10*time.Second)*2 {
							log.Warnf("collect cluster settings for cluster [%s] is still running, elapsed: %v, skip waiting", v.Name, elapsed.String())
						} else {
							log.Warnf("collect cluster settings for cluster [%s] is still running, elapsed: %v", v.Name, elapsed.String())
							return true
						}
					}
					module.settingsMap.Store(v.ID, time.Now())
					go func(clusterID string) {
						module.updateClusterSettings(clusterID)
						module.settingsMap.Delete(clusterID)
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
	settingsMap  sync.Map
	stateMap     sync.Map
	healthMap    sync.Map
}

// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"context"
	"fmt"
	"github.com/rubyniu105/framework/core/model"
	"math"
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/credential"
	"github.com/rubyniu105/framework/core/elastic"
	"github.com/rubyniu105/framework/core/env"
	"github.com/rubyniu105/framework/core/event"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/kv"
	"github.com/rubyniu105/framework/core/orm"
	"github.com/rubyniu105/framework/core/task"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/modules/elastic/common"
)

func (module *ElasticModule) Name() string {
	return "elasticsearch"
}

var (
	defaultConfig = common.ModuleConfig{
		RemoteConfigEnabled: false,
		HealthCheckConfig: common.CheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		NodeAvailabilityCheckConfig: common.CheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		MetadataRefresh: common.CheckConfig{
			Enabled:  false,
			Interval: "10s",
		},
		ORMConfig: common.ORMConfig{
			Enabled:                 false,
			InitTemplate:            true,
			SkipInitDefaultTemplate: false,
			InitSchema:              true,
			IndexPrefix:             ".infini_",
		},
		StoreConfig: common.StoreConfig{
			Enabled: false,
		},
		ClusterSettingsCheckConfig: common.CheckConfig{
			Enabled:  false,
			Interval: "20s",
		},
		ClientTimeout: "60s",
	}
)

func getDefaultConfig() common.ModuleConfig {
	return defaultConfig
}

func loadFileBasedElasticConfig() []elastic.ElasticsearchConfig {
	var configs []elastic.ElasticsearchConfig
	exist, err := env.ParseConfig("elasticsearch", &configs)
	if exist && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	if exist {
		for _, v := range configs {
			v.Source = elastic.ElasticsearchConfigSourceFile
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
	esClient := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
	result, err := esClient.Search(orm.GetIndexName(elastic.ElasticsearchConfig{}), &query)
	if err != nil {
		log.Error(err)
		return configs
	}
	credentialIDsM := map[string]struct{}{}
	credentialIDs := []string{}

	if len(result.Hits.Hits) > 0 {
		for _, v1 := range result.Hits.Hits {
			cfg := elastic.ElasticsearchConfig{}
			bytes := util.MustToJSONBytes(v1.Source)
			util.MustFromJSONBytes(bytes, &cfg)
			cfg.ID = v1.ID
			configs = append(configs, cfg)
			if cfg.CredentialID != "" {
				if _, ok := credentialIDsM[cfg.CredentialID]; !ok {
					credentialIDs = append(credentialIDs, cfg.CredentialID)
					credentialIDsM[cfg.CredentialID] = struct{}{}
				}
			}
		}
	}
	if len(credentialIDs) > 0 {
		query.Query = &elastic.Query{
			BoolQuery: &elastic.BoolQuery{
				Must: []interface{}{
					util.MapStr{
						"terms": util.MapStr{
							"_id": credentialIDs,
						},
					},
				},
			}}
		searchRes, err := esClient.Search(orm.GetIndexName(credential.Credential{}), &query)
		if err != nil {
			log.Error(err)
			return configs
		}
		if len(searchRes.Hits.Hits) > 0 {
			credentials := map[string]*credential.Credential{}
			for _, v1 := range searchRes.Hits.Hits {
				cred := credential.Credential{}
				bytes := util.MustToJSONBytes(v1.Source)
				util.MustFromJSONBytes(bytes, &cred)
				cred.ID = v1.ID
				credentials[cred.ID] = &cred
			}
			for i, cfg := range configs {
				if cfg.CredentialID != "" {
					if v, ok := credentials[cfg.CredentialID]; ok {
						if v.Type == credential.BasicAuth {
							obj, err := v.Decode()
							if err != nil {
								log.Error(err)
								continue
							}
							if basicAuth, ok := obj.(model.BasicAuth); ok {
								configs[i].BasicAuth = &basicAuth
							}

						}
					}
				}
			}
		}

	}

	log.Infof("loading [%v] remote configs", len(result.Hits.Hits))
	return configs
}

func initElasticInstances(m []elastic.ElasticsearchConfig, source string) {
	for _, esConfig := range m {
		esConfig.Source = source
		common.InitElasticInstanceWithoutMetadata(esConfig)
	}
}

var moduleConfig = common.ModuleConfig{}

func (module *ElasticModule) Setup() {

	moduleConfig = getDefaultConfig()

	exists, err := env.ParseConfig("elastic", &moduleConfig)
	if exists && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}
	if exists {
		if moduleConfig.Elasticsearch != "" {
			global.Register(elastic.GlobalSystemElasticsearchID, moduleConfig.Elasticsearch)
		}
	}

	m := loadFileBasedElasticConfig()
	initElasticInstances(m, elastic.ElasticsearchConfigSourceFile)
}

func (module *ElasticModule) Stop() error {
	return nil
}

func nodeAvailabilityCheck() {
	availabilityMap := sync.Map{}
	task2 := task.ScheduleTask{
		Description: "check for elasticsearch node availability",
		Type:        "interval",
		Interval:    moduleConfig.NodeAvailabilityCheckConfig.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkHosts(func(key, value interface{}) bool {
				if global.ShuttingDown() {
					return true
				}

				k := key.(string)

				if value == nil {
					return true
				}

				v, ok := value.(*elastic.NodeAvailable)
				if ok {
					if v.ClusterID == "" {
						return true
					}

					cfg := elastic.GetConfig(v.ClusterID)
					if !cfg.Enabled || (cfg.MetadataConfigs != nil && !cfg.MetadataConfigs.NodeAvailabilityCheck.Enabled) {
						return true
					}

					if v.IsDead() {
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
					avail := util.TestTCPAddress(k, 10*time.Second)
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

func (module *ElasticModule) registerClusterStateRefreshTask() {
	module.stateMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch state refresh",
		Type:        "interval",
		Interval:    moduleConfig.MetadataRefresh.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				log.Trace("walk metadata: ", key)
				if global.ShuttingDown() {
					return true
				}

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init meta refresh task: [%v] [%v] [%v] [%v]", key, v.ID, v.Name, v.Enabled)

				if ok {
					if !v.Enabled || (v.MetadataConfigs != nil && !v.MetadataConfigs.MetadataRefresh.Enabled) {
						return true
					}

					if startTime, ok := module.stateMap.Load(v.ID); ok {
						elapsed := time.Since(startTime.(time.Time))
						interval := moduleConfig.MetadataRefresh.Interval
						if v.MetadataConfigs != nil && v.MetadataConfigs.MetadataRefresh.Interval != "" {
							interval = v.MetadataConfigs.MetadataRefresh.Interval
						}
						intervalD := util.GetDurationOrDefault(interval, 10*time.Second)
						if time.Since(startTime.(time.Time)) > intervalD*2 {
							log.Warnf("refresh cluster state for cluster [%s] is still running, elapsed: %v, skip waiting", v.Name, elapsed.String())
						} else {
							duration := elapsed - intervalD
							abd := math.Abs(duration.Seconds())
							if abd > 3 {
								log.Warnf("refresh cluster state for cluster [%s] is still running, elapsed: %v", v.Name, elapsed.String())
								return true
							}
						}
					}
					module.stateMap.Store(v.ID, time.Now())

					task.RunWithContext("refresh_cluster_state", func(ctx context.Context) error {
						clusterID := task.MustGetString(ctx, "id")
						module.updateClusterState(clusterID, false)
						module.stateMap.Delete(clusterID)
						return nil
					}, context.WithValue(context.Background(), "id", v.ID))
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)
}

var schemaInited bool

func InitSchema() {
	if schemaInited {
		return
	}
	if !moduleConfig.ORMConfig.InitSchema {
		return
	}

	//TODO move to dedicated module
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

	//init schemas
	err = orm.InitSchema()
	if err != nil {
		panic(err)
	}

	schemaInited = true
}

var ormInited bool

func (module *ElasticModule) Start() error {

	if moduleConfig.ORMConfig.Enabled {
		client := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
		handler := ElasticORM{Client: client, Config: moduleConfig.ORMConfig}
		orm.Register("elastic", &handler)
	}

	if moduleConfig.StoreConfig.Enabled {
		client := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
		module.storeHandler = &ElasticStore{Client: client, Config: moduleConfig.StoreConfig}
		kv.Register("elastic", module.storeHandler)
	}

	if moduleConfig.ORMConfig.Enabled {
		if !ormInited {
			//init template
			InitTemplate(false)
			//register schema
			InitSchema()
			ormInited = true
		}
	}

	if moduleConfig.RemoteConfigEnabled {
		m := loadESBasedElasticConfig()
		initElasticInstances(m, elastic.ElasticsearchConfigSourceElasticsearch)
	}

	if module.storeHandler != nil {
		err := module.storeHandler.Open()
		if err != nil {
			panic(err)
		}
	}

	if !moduleConfig.SkipInitMetadataOnStart {
		//init elasticsearch
		elastic.WalkConfigs(func(key, value interface{}) bool {
			if global.ShuttingDown() {
				return true
			}

			log.Trace("init cluster: ", key, ",", util.MustToJSON(value))

			cfg1, ok := value.(*elastic.ElasticsearchConfig)
			if ok && cfg1 != nil {
				log.Tracef("init elasticsearch config: %v", cfg1.Name)
				metadata := elastic.GetMetadata(cfg1.ID)
				if metadata != nil {
					//update nodes
					module.updateNodeInfo(metadata, true, cfg1.Discovery.Enabled)

					//update alias
					updateAliases(metadata, true)

					//update
					module.updateClusterState(cfg1.ID, true)
				}

				task.RunWithContext("cluster_health_check", func(ctx context.Context) error {
					id := task.MustGetString(ctx, "id")
					module.clusterHealthCheck(id, true)
					return nil
				}, context.WithValue(context.Background(), "id", cfg1.ID))
			}
			return true
		})
	}
	if moduleConfig.HealthCheckConfig.Enabled {
		module.healthMap = sync.Map{}
		t := task.ScheduleTask{
			Description: "cluster health check",
			Type:        "interval",
			Interval:    moduleConfig.HealthCheckConfig.Interval,
			Task: func(ctx context.Context) {
				elastic.WalkConfigs(func(key, value interface{}) bool {
					if global.ShuttingDown() {
						return true
					}
					cfg1, ok := value.(*elastic.ElasticsearchConfig)
					if ok && cfg1 != nil {
						if !cfg1.Enabled || (cfg1.MetadataConfigs != nil && !cfg1.MetadataConfigs.HealthCheck.Enabled) {
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
							} else if math.Abs((elapsed - tinterval).Seconds()) > 3 {
								log.Warnf("health check for cluster [%s] is still running, elapsed: %v", cfg1.Name, elapsed.String())
								return true
							}
						}
						module.healthMap.Store(cfg1.ID, time.Now())

						task.RunWithContext("refresh_cluster_health", func(ctx context.Context) error {
							clusterID := task.MustGetString(ctx, "id")
							module.clusterHealthCheck(clusterID, false)
							module.healthMap.Delete(clusterID)
							return nil
						}, context.WithValue(context.Background(), "id", cfg1.ID))
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
		module.registerClusterStateRefreshTask()

		//refresh nodes
		task2 := task.ScheduleTask{
			Description: fmt.Sprintf("elasticsearch nodes discovery"),
			Type:        "interval",
			Interval:    "60s",
			Task: func(ctx context.Context) {
				module.refreshAllClusterMetadata()
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
				module.refreshAllClusterAlias(false)
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
		module.registerClusterSettingsRefreshTask()
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
			initElasticInstances(newConfig, elastic.ElasticsearchConfigSourceFile)
		}
	})

	return nil

}

func (module *ElasticModule) registerClusterSettingsRefreshTask() {
	module.settingsMap = sync.Map{}
	task2 := task.ScheduleTask{
		Description: "elasticsearch settings refresh",
		Type:        "interval",
		Interval:    moduleConfig.ClusterSettingsCheckConfig.Interval,
		Task: func(ctx context.Context) {
			elastic.WalkConfigs(func(key, value interface{}) bool {
				if global.ShuttingDown() {
					return true
				}

				log.Trace("walk metadata: ", key)

				if value == nil {
					return true
				}
				v, ok := value.(*elastic.ElasticsearchConfig)
				log.Tracef("init settings refresh task: [%v] [%v] [%v] [%v]", key, v.ID, v.Name, v.Enabled)

				if ok {
					if !v.Enabled || (v.MetadataConfigs != nil && !v.MetadataConfigs.ClusterSettingsCheck.Enabled) {
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
					task.RunWithContext("refresh_cluster_settings", func(ctx context.Context) error {
						clusterID := task.MustGetString(ctx, "id")
						module.updateClusterSettings(clusterID)
						module.settingsMap.Delete(clusterID)
						return nil
					}, context.WithValue(context.Background(), "id", v.ID))
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)

}

func (module *ElasticModule) refreshAllClusterMetadata() {
	elastic.WalkMetadata(func(key, value interface{}) bool {
		if global.ShuttingDown() {
			return true
		}

		if value == nil {
			return true
		}

		v, ok := value.(*elastic.ElasticsearchMetadata)
		if ok {
			module.updateNodeInfo(v, false, v.Config.Discovery.Enabled)
		}
		return true
	})
}

func (module *ElasticModule) refreshAllClusterAlias(force bool) {
	elastic.WalkMetadata(func(key, value interface{}) bool {
		if value == nil {
			return true
		}
		v, ok := value.(*elastic.ElasticsearchMetadata)
		if ok {
			updateAliases(v, force)
		}
		return true
	})
}

type ElasticModule struct {
	storeHandler *ElasticStore
	settingsMap  sync.Map
	stateMap     sync.Map
	healthMap    sync.Map
}

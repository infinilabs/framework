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

package elastic

import (
	"context"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type ElasticsearchMetric struct {
	Enabled       bool `config:"enabled"`
	IsAgentMode   bool `config:"agent_mode"`
	ClusterHealth bool `config:"cluster_health"`

	ClusterStats bool `config:"cluster_stats"`
	NodeStats    bool `config:"node_stats"`

	IndexStats    bool `config:"index_stats"`
	AllIndexStats bool `config:"all_index_stats"`

	IndexPrimaryStats bool `config:"index_primary_stats"`
	IndexTotalStats   bool `config:"index_total_stats"`

	ClusterState bool   `config:"cluster_state"`
	NodeInfo     bool   `config:"node_info"`
	Interval     string `config:"interval"`
	onSaveEvent  func(item *event.Event) error
	taskIDs      sync.Map
}

//元数据定期快照
//GET /_nodes/_local        #获取元数据信息，根据变更来判断是否存储信息快照
//GET /_cluster/state/version,nodes,master_node,routing_table

//统计信息
//GET _cluster/stats
//GET _stats    #集群级别统计信息，已处理
//GET /_nodes/_local/stats  #节点级别统计信息，统计信息

func New(cfg *config.Config, saveEvent func(item *event.Event) error) (*ElasticsearchMetric, error) {
	me := &ElasticsearchMetric{
		onSaveEvent:       saveEvent,
		ClusterHealth:     true,
		ClusterStats:      true,
		NodeStats:         true,
		IndexStats:        true,
		AllIndexStats:     true,
		IndexPrimaryStats: true,
		IndexTotalStats:   true,
		ClusterState:      true,
		Interval:          "10s",
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	return me, nil
}

func getMonitorConfigs(v *elastic.ElasticsearchMetadata) *elastic.MonitorConfig {
	if v.Config.MonitorConfigs == nil {
		return &elastic.MonitorConfig{
			ClusterStats: elastic.TaskConfig{
				Enabled:  true,
				Interval: "10s",
			},
			NodeStats: elastic.TaskConfig{
				Enabled:  true,
				Interval: "10s",
			},
			ClusterHealth: elastic.TaskConfig{
				Enabled:  true,
				Interval: "10s",
			},
			IndexStats: elastic.TaskConfig{
				Enabled:  true,
				Interval: "10s",
			},
		}
	}
	validateMonitorConfig(&v.Config.MonitorConfigs.ClusterStats)
	validateMonitorConfig(&v.Config.MonitorConfigs.ClusterHealth)
	validateMonitorConfig(&v.Config.MonitorConfigs.NodeStats)
	validateMonitorConfig(&v.Config.MonitorConfigs.IndexStats)

	return v.Config.MonitorConfigs
}
func validateMonitorConfig(monitorConfig *elastic.TaskConfig) {
	if monitorConfig.Enabled {
		duration, _ := time.ParseDuration(monitorConfig.Interval)
		if duration < time.Second*10 {
			monitorConfig.Interval = "10s"
		}
	}
}

func (m *ElasticsearchMetric) Collect() error {
	if !m.Enabled {
		return nil
	}
	elastic.WalkMetadata(func(key, value interface{}) bool {
		log.Debug("collecting metrics for: ", key)
		k := key.(string)
		if value == nil {
			log.Error("nil value:", key)
			return true
		}
		v, ok := value.(*elastic.ElasticsearchMetadata)
		if !ok {
			return true
		}
		m.InitialCollectTask(k, v)
		return true
	})
	return nil
}

// RemoveAllCollectTasks remove all running collect tasks
func (m *ElasticsearchMetric) RemoveAllCollectTasks() {
	m.taskIDs.Range(func(key, value any) bool {
		m.RemoveTask(key.(string))
		return true
	})
}

// RemoveTask remove a running collect task by taskID
func (m *ElasticsearchMetric) RemoveTask(taskID string) {
	task.DeleteTask(taskID)
	m.taskIDs.Delete(taskID)
}

// RemoveTasksByClusterID removes running collect tasks associated with a clusterID
func (m *ElasticsearchMetric) RemoveTasksByClusterID(clusterID string) {
	var (
		clusterHealthTaskID = fmt.Sprintf("collect-cluster_health_%s", clusterID)
		clusterStatsTaskID  = fmt.Sprintf("collect-cluster_stats_%s", clusterID)
		nodeStatsTaskID     = fmt.Sprintf("collect-node_stats_%s", clusterID)
		indexStatsTaskID    = fmt.Sprintf("collect-index_stats_%s", clusterID)
	)
	for _, taskID := range []string{clusterHealthTaskID, clusterStatsTaskID, nodeStatsTaskID, indexStatsTaskID} {
		m.RemoveTask(taskID)
	}
}

func (m *ElasticsearchMetric) InitialCollectTask(k string, v *elastic.ElasticsearchMetadata) bool {
	//just skip if elastic metric was not enabled
	if !m.Enabled {
		return false
	}
	var (
		clusterHealthTaskID = fmt.Sprintf("collect-cluster_health_%s", k)
		clusterStatsTaskID  = fmt.Sprintf("collect-cluster_stats_%s", k)
		nodeStatsTaskID     = fmt.Sprintf("collect-node_stats_%s", k)
		indexStatsTaskID    = fmt.Sprintf("collect-index_stats_%s", k)
	)
	//clear old collect tasks if exists
	for _, taskID := range []string{clusterHealthTaskID, clusterStatsTaskID, nodeStatsTaskID, indexStatsTaskID} {
		if _, ok := m.taskIDs.Load(taskID); ok {
			m.RemoveTask(taskID)
		}
	}
	if !v.Config.Monitored || !v.Config.Enabled {
		log.Debugf("cluster [%v] NOT (enabled[%v] or monitored[%v] or not available[%v]), skip collect", v.Config.Name, v.Config.Enabled, v.Config.Monitored, v.IsAvailable())
		return true
	}
	if global.Env().IsDebug {
		log.Debugf("run monitoring task for elasticsearch: %v - %v", k, v.Config.Name)
	}

	var err error
	monitorConfigs := getMonitorConfigs(v)
	if m.ClusterHealth && monitorConfigs.ClusterHealth.Enabled {
		log.Debugf("collect cluster health: %s, endpoint: %s\n", k, v.Config.GetAnyEndpoint())
		var clusterHealthMetricTask = task.ScheduleTask{
			ID:          clusterHealthTaskID,
			Description: fmt.Sprintf("monitoring cluster health metric  for cluster %s", k),
			Type:        "interval",
			Singleton:   true,
			Interval:    monitorConfigs.ClusterHealth.Interval,
			Task: func(ctx context.Context) {
				if !v.IsAvailable() {
					log.Debugf("cluster [%v] is not available, skip collect cluster health metric", v.Config.Name)
					return
				}
				err = m.CollectClusterHealth(k, v)
				if err != nil {
					log.Error(err)
				}
			},
		}
		taskID := task.RegisterScheduleTask(clusterHealthMetricTask)
		m.taskIDs.Store(taskID, struct{}{})
	}

	//cluster stats
	if m.ClusterStats && monitorConfigs.ClusterStats.Enabled {
		log.Debugf("collect cluster state: %s, endpoint: %s\n", k, v.Config.GetAnyEndpoint())
		var clusterStatsMetricTask = task.ScheduleTask{
			ID:          clusterStatsTaskID,
			Description: fmt.Sprintf("monitoring cluster stats metric for cluster %s", k),
			Type:        "interval",
			Singleton:   true,
			Interval:    monitorConfigs.ClusterStats.Interval,
			Task: func(ctx context.Context) {
				if !v.IsAvailable() {
					log.Debugf("cluster [%v] is not available, skip collect cluster stats metric", v.Config.Name)
					return
				}
				err = m.CollectClusterState(k, v)
				if err != nil {
					log.Error(err)
				}
			},
		}
		taskID := task.RegisterScheduleTask(clusterStatsMetricTask)
		m.taskIDs.Store(taskID, struct{}{})
	}

	//nodes stats
	if m.NodeStats && monitorConfigs.NodeStats.Enabled {
		var nodeStatsMetricTask = task.ScheduleTask{
			ID:          nodeStatsTaskID,
			Description: fmt.Sprintf("monitoring node stats metric for cluster %s", k),
			Type:        "interval",
			Interval:    monitorConfigs.NodeStats.Interval,
			Singleton:   true,
			Task: func(ctx context.Context) {
				if !v.IsAvailable() {
					log.Debugf("cluster [%v] is not available, skip collect node stats metric", v.Config.Name)
					return
				}
				var (
					shards []elastic.CatShardResponse
				)

				client := elastic.GetClient(k)
				shards, err = client.CatShards()
				if err != nil {
					log.Debug(v.Config.Name, " get shards info error: ", err)
				}
				shardInfos := map[string]map[string]interface{}{}
				indexInfos := map[string]map[string]bool{}
				for _, item := range shards {
					if item.State == "UNASSIGNED" {
						continue
					}
					if _, ok := shardInfos[item.NodeID]; !ok {
						shardInfos[item.NodeID] = map[string]interface{}{
							"shard_count":    0,
							"replicas_count": 0,
							"indices_count":  0,
							"shards":         []interface{}{},
						}
					}
					if _, ok := indexInfos[item.NodeID]; !ok {
						indexInfos[item.NodeID] = map[string]bool{}
					}
					if item.ShardType == "p" {
						shardInfos[item.NodeID]["shard_count"] = shardInfos[item.NodeID]["shard_count"].(int) + 1
					} else {
						shardInfos[item.NodeID]["replicas_count"] = shardInfos[item.NodeID]["replicas_count"].(int) + 1
					}
					shardInfos[item.NodeID]["shards"] = append(shardInfos[item.NodeID]["shards"].([]interface{}), item)
					indexInfos[item.NodeID][item.Index] = true
				}
				if len(shardInfos) > 50 || len(shards) > 5000 {
					log.Warnf("cluster [%v] has over 50 nodes or 5000 shards. use the agent for metrics collection: https://github.com/infinilabs/agent.", v.Config.Name)
				}

				host := v.GetActiveHost()
				//published host is not a valid host
				if host != "" && !elastic.IsHostDead(host) && elastic.IsHostAvailable(host) {
					//host not dead and is not available, skip collecting
					stats := client.GetNodesStats("", host, "")
					if stats.ErrorObject != nil {
						log.Errorf("error on get node stats: %v %v", host, stats.ErrorObject)
					} else {
						for nodeID, nodeStats := range stats.Nodes {
							if _, ok := shardInfos[nodeID]; ok {
								shardInfos[nodeID]["indices_count"] = len(indexInfos[nodeID])
							}
							m.SaveNodeStats(v, nodeID, nodeStats, shardInfos[nodeID])
						}
					}
				} else {
					log.Debugf("host [%v] is not available, skip metrics collecting", host)
				}

			},
		}
		taskID := task.RegisterScheduleTask(nodeStatsMetricTask)
		m.taskIDs.Store(taskID, struct{}{})
	}

	//indices stats
	if (m.AllIndexStats || m.IndexStats) && monitorConfigs.IndexStats.Enabled {
		var indexStatsMetricTask = task.ScheduleTask{
			ID:          indexStatsTaskID,
			Description: fmt.Sprintf("monitoring index stats metric for cluster %s", k),
			Type:        "interval",
			Interval:    monitorConfigs.ClusterStats.Interval,
			Singleton:   true,
			Task: func(ctx context.Context) {
				if !v.IsAvailable() {
					log.Debugf("cluster [%v] is not available, skip collect index stats metric", v.Config.Name)
					return
				}
				var (
					shards []elastic.CatShardResponse
				)

				client := elastic.GetClient(k)
				shards, err = client.CatShards()
				if err != nil {
					log.Debug(v.Config.Name, " get shards info error: ", err)
					//return true
				}
				if (v.Health != nil && v.Health.NumberOfNodes > 50) || len(shards) > 5000 {
					log.Warnf("cluster [%v] has over 50 nodes or 5000 shards. use the agent for metrics collection: https://github.com/infinilabs/agent.", v.Config.Name)
				}
				indexStats, err := client.GetStats()
				if err != nil {
					log.Error(v.Config.Name, " get indices stats error: ", err)
					return
				}

				if indexStats != nil {
					var indexInfos *map[string]elastic.IndexInfo
					shardInfos := map[string][]elastic.CatShardResponse{}

					if v.IsAvailable() {
						indexInfos, err = client.GetIndices("")
						if err != nil {
							log.Error(v.Config.Name, " get indices info error: ", err)
						}

						for _, item := range shards {
							if _, ok := shardInfos[item.Index]; !ok {
								shardInfos[item.Index] = []elastic.CatShardResponse{
									item,
								}
							} else {
								shardInfos[item.Index] = append(shardInfos[item.Index], item)
							}
						}
					}

					if m.AllIndexStats {
						m.SaveIndexStats(v, "_all", "_all", indexStats.All.Primaries, indexStats.All.Total, nil, nil)
					}

					if m.IndexStats {
						for x, y := range indexStats.Indices {
							var indexInfo elastic.IndexInfo
							var shardInfo []elastic.CatShardResponse
							if indexInfos != nil {
								indexInfo = (*indexInfos)[x]
							}
							if shardInfos != nil {
								shardInfo = shardInfos[x]
							}
							m.SaveIndexStats(v, y.Uuid, x, y.Primaries, y.Total, &indexInfo, shardInfo)
						}
					}
				}
			},
		}
		taskID := task.RegisterScheduleTask(indexStatsMetricTask)
		m.taskIDs.Store(taskID, struct{}{})
	} else {
		if global.Env().IsDebug {
			log.Debugf("elasticsearch: %v - %v, no index info was found, skip index metrics collect", k, v.Config.Name)
		}
	}
	return true
}

func (m *ElasticsearchMetric) SaveNodeStats(v *elastic.ElasticsearchMetadata, nodeID string, f interface{}, shardInfo interface{}) error {
	//remove adaptive_selection
	x, ok := f.(map[string]interface{})
	if !ok {
		log.Errorf("invalid node stats for [%v] [%v]", v.Config.ID, nodeID)
		return errors.New("invalid node stats")
	}

	if ok {
		delete(x, "adaptive_selection")
		delete(x, "ingest")
		util.MapStr(x).Delete("indices.segments.max_unsafe_auto_id_timestamp")
		x["shard_info"] = shardInfo
	}
	nodeName := x["name"]
	nodeIP := x["ip"]
	nodeAddress := x["transport_address"]
	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "node_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":        v.Config.ID,
				"cluster_uuid":      v.Config.ClusterUUID,
				"node_id":           nodeID,
				"node_name":         nodeName,
				"ip":                nodeIP,
				"transport_address": nodeAddress,
			},
		},
	}
	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"node_stats": x,
		},
	}

	return m.onSaveEvent(&item)
}

func (m *ElasticsearchMetric) SaveIndexStats(v *elastic.ElasticsearchMetadata, indexID, indexName string, primary, total elastic.IndexLevelStats, info *elastic.IndexInfo, shardInfo []elastic.CatShardResponse) error {
	newIndexID := fmt.Sprintf("%s:%s", v.Config.ID, indexName)
	if indexID == "_all" {
		newIndexID = indexID
	}
	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "index_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   v.Config.ID,
				"cluster_uuid": v.Config.ClusterUUID,

				"index_id":   newIndexID,
				"index_uuid": indexID,
				"index_name": indexName,
			},
		},
	}

	mtr := util.MapStr{}
	if m.IndexPrimaryStats {
		mtr["primaries"] = primary
		mtr["total"] = total
		mtr["index_info"] = info
		mtr["shard_info"] = shardInfo
	}

	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"index_stats": mtr,
		},
	}

	return m.onSaveEvent(&item)
}

func (m *ElasticsearchMetric) CollectClusterHealth(k string, v *elastic.ElasticsearchMetadata) error {

	log.Trace("collecting custer health metrics for :", k)

	client := elastic.GetClient(k)
	//add context to control timeout for metric collecting,
	//since next metric collecting round will be triggered after this one
	monitorCfg := getMonitorConfigs(v)
	du, _ := time.ParseDuration(monitorCfg.ClusterHealth.Interval)
	ctx, cancel := context.WithTimeout(context.Background(), du)
	defer cancel()
	var (
		health *elastic.ClusterHealth
		err    error
	)
	health, err = client.ClusterHealthSpecEndpoint(ctx, v.Config.GetAnyEndpoint(), "indices")
	if err != nil {
		log.Error(v.Config.Name, " get cluster health error: ", err)
		return err
	}

	indicesHealth := health.Indices
	health.Indices = nil
	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "cluster_health",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   v.Config.ID,
				"cluster_uuid": v.Config.ClusterUUID,
			},
		},
	}
	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"cluster_health": health,
		},
	}

	err = m.onSaveEvent(&item)
	if err != nil {
		return err
	}
	for indexName, healthInfo := range indicesHealth {
		item = event.Event{
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name:     "index_health",
				Datatype: "snapshot",
				Labels: util.MapStr{
					"cluster_id":   v.Config.ID,
					"cluster_uuid": v.Config.ClusterUUID,
					"index_name":   indexName,
				},
			},
		}
		item.Fields = util.MapStr{
			"elasticsearch": util.MapStr{
				"index_health": healthInfo,
			},
		}
		err = m.onSaveEvent(&item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *ElasticsearchMetric) CollectClusterState(k string, v *elastic.ElasticsearchMetadata) error {

	log.Trace("collecting custer state metrics for :", k)

	client := elastic.GetClient(k)

	var stats *elastic.ClusterStats
	//add context to control timeout for metric collecting,
	//since next metric collecting round will be triggered after this one
	monitorCfg := getMonitorConfigs(v)
	du, _ := time.ParseDuration(monitorCfg.ClusterHealth.Interval)
	ctx, cancel := context.WithTimeout(context.Background(), du)
	defer cancel()
	var err error
	if m.IsAgentMode {
		stats, err = client.GetClusterStatsSpecEndpoint(ctx, "", v.Config.GetAnyEndpoint())
	} else {
		stats, err = client.GetClusterStats(ctx, "")
	}
	if err != nil {
		log.Error(v.Config.Name, " get cluster stats error: ", err)
		return err
	}

	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "cluster_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   v.Config.ID,
				"cluster_uuid": v.Config.ClusterUUID,
			},
		},
	}

	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"cluster_stats": stats,
		},
	}

	return m.onSaveEvent(&item)
}

func (m *ElasticsearchMetric) CollectNodeStats() {

}

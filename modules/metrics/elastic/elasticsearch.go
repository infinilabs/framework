package elastic

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"math"
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

var collectLoadingMap = sync.Map{}

func canDoMonitor(key string, subKey string, interval string, configName string) bool {
	if item, ok := collectLoadingMap.Load(key); ok {
		itemM := item.(map[string]time.Time)
		if startTime, ok := itemM[subKey]; ok && !startTime.IsZero() {
			elapsed := time.Since(startTime)
			intervalD := util.GetDurationOrDefault(interval, 10*time.Second)
			duration := elapsed - intervalD
			abd := math.Abs(duration.Seconds())
			if abd < 3 {
				return true
			} else if duration > intervalD {
				log.Warnf("collect metrics of %s for cluster %s is still running, elapsed: %v, skip waiting", subKey, configName, elapsed.String())
				return true
			} else {
				//log.Warnf("collect  metrics of %s for cluster %s is still running, elapsed: %v",subKey, configName, elapsed.String())
				return false
			}
		}
	}
	return true
}
func setLastMonitorTime(key string, subKey string, lastTime time.Time) {
	if item, ok := collectLoadingMap.Load(key); ok {
		itemM := item.(map[string]time.Time)
		//if lastTime.IsZero(){
		//	delete(itemM, subKey)
		//	if len(itemM) == 0 {
		//		collectLoadingMap.Delete(key)
		//	}
		//}
		if t, ok := itemM[subKey]; ok {
			if lastTime.Before(t) {
				return
			}
		}
		itemM[subKey] = lastTime
		collectLoadingMap.Store(key, itemM)
		return
	}
	collectLoadingMap.Store(key, map[string]time.Time{
		subKey: lastTime,
	})
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
	if m.IsAgentMode {
		return m.CollectOfAgentMode()
	}

	collectStartTime := time.Now()
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
		m.DoCollect(k, v, collectStartTime)
		return true
	})
	return nil
}

func (m *ElasticsearchMetric) CollectOfAgentMode() error {
	collectStartTime := time.Now()
	elastic.WalkMetadata(func(key, value interface{}) bool {
		log.Debug("collecting metrics for: ", key)
		k := key.(string)
		v, ok := value.(*elastic.ElasticsearchMetadata)
		if !ok {
			return true
		}
		if v.Config.Endpoint == "" {
			return true
		}
		go m.DoCollect(k, v, collectStartTime)
		return true
	})
	return nil
}

func (m *ElasticsearchMetric) DoCollect(k string, v *elastic.ElasticsearchMetadata, collectStartTime time.Time) bool {

	if !v.Config.Monitored || !v.Config.Enabled {
		log.Debugf("cluster [%v] NOT (enabled[%v] or monitored[%v] or not available[%v]), skip collect", v.Config.Name, v.Config.Enabled, v.Config.Monitored, v.IsAvailable())
		collectLoadingMap.Delete(k)
		return true
	}
	if global.Env().IsDebug {
		log.Debugf("run monitoring task for elasticsearch: %v - %v", k, v.Config.Name)
	}

	//collectLoadingMap.Store(key, time.Now())
	//defer collectLoadingMap.Delete(key)

	var err error
	monitorConfigs := getMonitorConfigs(v)
	if m.ClusterHealth && v.IsAvailable() && monitorConfigs.ClusterHealth.Enabled {
		if canDoMonitor(k, "cluster_health", monitorConfigs.ClusterHealth.Interval, v.Config.Name) {
			t1 := time.Now()
			setLastMonitorTime(k, "cluster_health", collectStartTime)
			log.Debugf("collect cluster health: %s, endpoint: %s\n", k, v.Config.GetAnyEndpoint())
			err = m.CollectClusterHealth(k, v)
			log.Trace("time of CollectClusterHealth:", time.Since(t1).String())
			if err != nil {
				log.Error(err)
			}
		}

	}

	//cluster stats
	if m.ClusterStats && v.IsAvailable() && monitorConfigs.ClusterStats.Enabled {
		if canDoMonitor(k, "cluster_stats", monitorConfigs.ClusterStats.Interval, v.Config.Name) {
			t1 := time.Now()
			setLastMonitorTime(k, "cluster_stats", collectStartTime)
			log.Debugf("collect cluster state: %s, endpoint: %s\n", k, v.Config.GetAnyEndpoint())
			err = m.CollectClusterState(k, v)
			log.Trace("time of CollectClusterState:", time.Since(t1).String())
			if err != nil {
				log.Error(err)
			}
		}

	}

	var (
		shards []elastic.CatShardResponse
	)

	client := elastic.GetClient(k)

	if ((m.NodeStats && monitorConfigs.NodeStats.Enabled) || (m.IndexStats && monitorConfigs.IndexStats.Enabled)) && v.IsAvailable() {
		t1 := time.Now()
		if m.IsAgentMode {
			shards, err = client.CatShardsSpecEndpoint(v.Config.GetAnyEndpoint())
		} else {
			shards, err = client.CatShards()
		}

		log.Trace("time of CatShards:", time.Since(t1).String())

		if err != nil {
			log.Debug(v.Config.Name, " get shards info error: ", err)
			//return true
		}
	}

	//nodes stats
	if m.NodeStats && monitorConfigs.NodeStats.Enabled && v.IsAvailable() {
		if v.Config.Discovery.Enabled && v.Nodes == nil {
			if global.Env().IsDebug {
				log.Debugf("elasticsearch: %v - %v, no nodes info was found, skip nodes metrics collect", k, v.Config.Name)
			}
			return true
		}
		if canDoMonitor(k, "node_stats", monitorConfigs.NodeStats.Interval, v.Config.Name) {
			setLastMonitorTime(k, "node_stats", collectStartTime)

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

			//get node stats per each node
			if v.Nodes != nil {
				for nodeID, y := range *v.Nodes {
					//get node level stats
					nodeHost := y.GetHttpPublishHost()

					var host string
					//published host is not a valid host
					if elastic.IsHostDead(nodeHost) {
						host = v.GetActivePreferredHost(nodeHost)
					} else {
						//the  node is online
						if elastic.IsHostAvailable(nodeHost) {
							host = nodeHost
						} else {
							//host not dead and is not available, skip collecting
							log.Debugf("host [%v] is not available, skip metrics collecting", nodeHost)
							continue
						}
					}
					log.Debugf("collect nodes stats, endpoint: %s", host)
					stats := client.GetNodesStats(nodeID, host,"")

					log.Trace(y.GetHttpPublishHost(), " => ", host, stats.ErrorObject)

					if stats.ErrorObject != nil {
						log.Errorf("get node stats of %s error: %v", y.Name, stats.ErrorObject)
						continue
					}
					if _, ok := shardInfos[nodeID]; ok {
						shardInfos[nodeID]["indices_count"] = len(indexInfos[nodeID])
					}
					m.SaveNodeStats(v, nodeID, stats.Nodes[nodeID], shardInfos[nodeID])
				}
			} else {
				host := v.GetActiveHost()
				//published host is not a valid host
				if host != "" && !elastic.IsHostDead(host) && elastic.IsHostAvailable(host) {
					//host not dead and is not available, skip collecting
					stats := client.GetNodesStats("", host,"")
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
			}
		}
	}

	//indices stats
	if v.IsAvailable() && (m.AllIndexStats || m.IndexStats) && monitorConfigs.IndexStats.Enabled {
		if canDoMonitor(k, "index_stats", monitorConfigs.IndexStats.Interval, v.Config.Name) {
			setLastMonitorTime(k, "index_stats", collectStartTime)
			indexStats, err := client.GetStats()
			if err != nil {
				log.Error(v.Config.Name, " get indices stats error: ", err)
				return true
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

		} else {
			if global.Env().IsDebug {
				log.Debugf("elasticsearch: %v - %v, no index info was found, skip index metrics collect", k, v.Config.Name)
			}
		}
	}
	return true
}

func (m *ElasticsearchMetric) SaveNodeStats(v *elastic.ElasticsearchMetadata, nodeID string, f interface{}, shardInfo interface{})error {
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

func (m *ElasticsearchMetric) SaveIndexStats(v *elastic.ElasticsearchMetadata, indexID, indexName string, primary, total elastic.IndexLevelStats, info *elastic.IndexInfo, shardInfo []elastic.CatShardResponse) error{
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
	var health *elastic.ClusterHealth
	var err error
	if m.IsAgentMode {
		health, err = client.ClusterHealthSpecEndpoint(nil, v.Config.GetAnyEndpoint(), "")
	} else {
		health, err = client.ClusterHealth(nil)
	}
	if err != nil {
		log.Error(v.Config.Name, " get cluster health error: ", err)
		return err
	}

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

	return m.onSaveEvent(&item)
}

func (m *ElasticsearchMetric) CollectClusterState(k string, v *elastic.ElasticsearchMetadata) error {

	log.Trace("collecting custer state metrics for :", k)

	client := elastic.GetClient(k)

	var stats *elastic.ClusterStats
	var err error
	if m.IsAgentMode {
		stats, err = client.GetClusterStatsSpecEndpoint(nil,"", v.Config.GetAnyEndpoint())
	} else {
		stats, err = client.GetClusterStats(nil, "")
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
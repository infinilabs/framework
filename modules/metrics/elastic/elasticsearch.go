package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type Metric struct {
	Enabled   bool `config:"enabled"`

	ClusterHealth bool `config:"cluster_health"`

	ClusterStats bool `config:"cluster_stats"`
	NodeStats bool `config:"node_stats"`

	IndexStats    bool `config:"index_stats"`
	AllIndexStats bool `config:"all_index_stats"`

	IndexPrimaryStats bool `config:"index_primary_stats"`
	IndexTotalStats   bool `config:"index_total_stats"`

	ClusterState bool `config:"cluster_state"`
	NodeInfo bool `config:"node_info"`
	Interval string `config:"interval"`
}

//元数据定期快照
//GET /_nodes/_local        #获取元数据信息，根据变更来判断是否存储信息快照
//GET /_cluster/state/version,nodes,master_node,routing_table

//统计信息
//GET _cluster/stats
//GET _stats    #集群级别统计信息，已处理
//GET /_nodes/_local/stats  #节点级别统计信息，统计信息


func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		ClusterHealth:      true,
		ClusterStats:      true,
		NodeStats:         true,
		IndexStats:        true,
		AllIndexStats:     true,
		IndexPrimaryStats: true,
		IndexTotalStats:   true,
		ClusterState:      true,
		Interval: "10s",
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	return me, nil
}

var collectLoadingMap = sync.Map{}
func (m *Metric) Collect() error {

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
		if ok {
			if !v.Config.Monitored || !v.Config.Enabled {
				log.Debugf("cluster [%v] NOT (enabled[%v] or monitored[%v] or not available[%v]), skip collect", v.Config.Name, v.Config.Enabled, v.Config.Monitored, v.IsAvailable())
				return true
			}
			if global.Env().IsDebug {
				log.Debugf("run monitoring task for elasticsearch: %v - %v", k, v.Config.Name)
			}
			if startTime, ok := collectLoadingMap.Load(key); ok {
				elapsed := time.Since(startTime.(time.Time))
				if time.Since(startTime.(time.Time)) > util.GetDurationOrDefault(m.Interval, 10*time.Second)*2 {
					log.Warnf("collect metrics for cluster %s is still running, elapsed: %v, skip waiting", v.Config.Name, elapsed.String())
				} else {
					log.Warnf("collect metrics for cluster %s is still running, elapsed: %v", v.Config.Name, elapsed.String())
					return true
				}
			}

			collectLoadingMap.Store(key, time.Now())
			defer collectLoadingMap.Delete(key)

			var err error
			if m.ClusterHealth && v.IsAvailable() {
				t1 := time.Now()
				err = m.CollectClusterHealth(k, v)
				log.Trace("time of CollectClusterHealth:", time.Since(t1).String())
				if err != nil {
					log.Error(err)
				}
			}

			//cluster stats
			if m.ClusterStats && v.IsAvailable() {
				t1 := time.Now()
				err = m.CollectClusterState(k, v)
				log.Trace("time of CollectClusterState:", time.Since(t1).String())
				if err != nil {
					log.Error(err)
				}
			}

			var (
				shards []elastic.CatShardResponse
			)

			client := elastic.GetClient(k)

			if (m.NodeStats || m.IndexStats) && v.IsAvailable() {
				t1 := time.Now()
				shards, err = client.CatShards()
				log.Trace("time of CatShards:", time.Since(t1).String())

				if err != nil {
					log.Error(v.Config.Name, " get shards info error: ", err)
					return true
				}
			}

			//nodes stats
			if m.NodeStats {
				if v.Config.Discovery.Enabled &&  v.Nodes == nil {
					if global.Env().IsDebug{
						log.Debugf("elasticsearch: %v - %v, no nodes info was found, skip nodes metrics collect",k,v.Config.Name)
					}
					return true
				}
				shardInfos := map[string]map[string]int{}
				indexInfos := map[string]map[string]bool{}
				for _, item := range shards {
					if item.State == "UNASSIGNED" {
						continue
					}
					if _, ok := shardInfos[item.NodeID]; !ok {
						shardInfos[item.NodeID] = map[string] int{
							"shard_count": 0,
							"replicas_count": 0,
							"indices_count": 0,
						}
					}
					if _, ok := indexInfos[item.NodeID]; !ok {
						indexInfos[item.NodeID] = map[string]bool {}
					}
					if item.ShardType == "p" {
						shardInfos[item.NodeID]["shard_count"]++
					}else{
						shardInfos[item.NodeID]["replicas_count"]++
					}
					indexInfos[item.NodeID][item.Index] = true
				}

				//get node stats per each node
				if v.Nodes!=nil{
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

						stats := client.GetNodesStats(nodeID, host)

						log.Trace(y.GetHttpPublishHost(), " => ", host, stats.ErrorObject)

						if stats.ErrorObject != nil {
							log.Errorf("get node stats of %s error: %v", y.Name, stats.ErrorObject)
							continue
						}
						if _, ok := shardInfos[nodeID]; ok {
							shardInfos[nodeID]["indices_count"] = len(indexInfos[nodeID])
						}
						m.SaveNodeStats(v.Config.ID, nodeID, stats.Nodes[nodeID], shardInfos[nodeID])
					}
				}else{
					host := v.GetActiveHost()
					//published host is not a valid host
					if !elastic.IsHostDead(host) && elastic.IsHostAvailable(host) {
						//host not dead and is not available, skip collecting
						stats := client.GetNodesStats("", host)
						if stats.ErrorObject != nil {
							log.Errorf("error on get node stats: %v %v", host, stats.ErrorObject)
						} else {
							for nodeID, nodeStats := range stats.Nodes {
								if _, ok := shardInfos[nodeID]; ok {
									shardInfos[nodeID]["indices_count"] = len(indexInfos[nodeID])
								}
								m.SaveNodeStats(v.Config.ID, nodeID, nodeStats, shardInfos[nodeID])
							}
						}
					} else {
						log.Debugf("host [%v] is not available, skip metrics collecting", host)
					}
				}
			}

			//indices stats
			if v.IsAvailable() && (m.AllIndexStats || m.IndexStats) {
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
							return true
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
						m.SaveIndexStats(v.Config.ID, "_all", "_all", indexStats.All.Primaries, indexStats.All.Total, nil, nil)
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
							m.SaveIndexStats(v.Config.ID, y.Uuid, x, y.Primaries, y.Total, &indexInfo, shardInfo)
						}
					}
				}else{
					if global.Env().IsDebug{
						log.Debugf("elasticsearch: %v - %v, no index info was found, skip index metrics collect",k,v.Config.Name)
					}
				}
			}

		}
		return true
	})

	return nil
}

func (m *Metric) SaveNodeStats( clusterId,nodeID string, f interface{}, shardInfo interface{}){
		//remove adaptive_selection
		x,ok:=f.(map[string]interface{})
		if !ok{
			log.Errorf("invalid node stats for [%v] [%v]", clusterId, nodeID)
			return
		}

		if ok{
			delete(x,"adaptive_selection")
			delete(x,"ingest")
			util.MapStr(x).Delete("indices.segments.max_unsafe_auto_id_timestamp")
			x["shard_info"] = shardInfo
		}
		nodeName:=x["name"]
		nodeIP:=x["ip"]
		nodeAddress:=x["transport_address"]
		item := event.Event{
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name:     "node_stats",
				Datatype: "snapshot",
				Labels: util.MapStr{
					"cluster_id":   clusterId,
					//"cluster_uuid": stats.ClusterUUID,
					"node_id":   nodeID,
					"node_name": nodeName,
					"ip": nodeIP,
					"transport_address":   nodeAddress,
				},
			},
		}
		item.Fields = util.MapStr{
			"elasticsearch": util.MapStr{
				"node_stats": x,
			},
		}
		err := event.Save(item)
		if err !=nil {
			log.Error(err)
		}
}

func (m *Metric) SaveIndexStats(clusterId, indexID, indexName string, primary, total elastic.IndexLevelStats, info *elastic.IndexInfo, shardInfo []elastic.CatShardResponse) {
	newIndexID := fmt.Sprintf("%s:%s", clusterId, indexName)
	if indexID == "_all" {
		newIndexID = indexID
	}
	//indexIDKey := []byte(newIndexID)
	//statusBytes, err := kv.GetValue(elastic.KVElasticIndexHealthStatus, indexIDKey)
	//var lastHealthStatus string
	//if statusBytes != nil {
	//	lastHealthStatus = string(statusBytes)
	//}
	//
	//if err == nil  && info != nil  {
	//	newStatusBytes := []byte(info.Health )
	//	if lastHealthStatus != "" {
	//		if !bytes.Equal(newStatusBytes, statusBytes) {
	//			kv.AddValue(elastic.KVElasticIndexHealthStatus, indexIDKey, newStatusBytes)
	//			queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexHealthStatus)
	//			if queueConfig.Labels == nil {
	//				queueConfig.Labels = map[string]interface{}{
	//					"type":     "metadata",
	//					"name":     "index_health_change",
	//					"category": "elasticsearch",
	//				}
	//			}
	//			ev := event.Event{
	//				Metadata: event.EventMetadata{
	//					Category: "elasticsearch",
	//					Name:     "index_health_change",
	//					Datatype: "snapshot",
	//					Labels: util.MapStr{
	//						"cluster_id": clusterId,
	//						"index_id":   newIndexID,
	//						"index_uuid": indexID,
	//						"index_name": indexName,
	//						"type":       "metadata",
	//						"from":       lastHealthStatus,
	//						"to":         info.Health,
	//					},
	//				},
	//			}
	//			err = queue.Push(queueConfig, util.MustToJSONBytes(ev))
	//			if err != nil {
	//				panic(err)
	//			}
	//		}
	//	}else{
	//		kv.AddValue(elastic.KVElasticIndexHealthStatus, indexIDKey, newStatusBytes)
	//	}
	//}
	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "index_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   clusterId,
				//"cluster_uuid": clusterId,

				"index_id": newIndexID,
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

	event.Save(item)
}

func (m *Metric) CollectClusterHealth(k string, v *elastic.ElasticsearchMetadata) error {

	log.Trace("collecting custer health metrics for :", k)

	client := elastic.GetClient(k)
	health, err := client.ClusterHealth()
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
				"cluster_id": v.Config.ID,
			},
		},
	}
	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"cluster_health": health,
		},
	}
	return event.Save(item)
}

func (m *Metric) CollectClusterState(k string, v *elastic.ElasticsearchMetadata) error {

	log.Trace("collecting custer state metrics for :", k)

	client := elastic.GetClient(k)

	stats, err := client.GetClusterStats("")
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
				"cluster_id": v.Config.ID,
			},
		},
	}

	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"cluster_stats": stats,
		},
	}

	return event.Save(item)
}

func (m *Metric) CollectNodeStats() {

}

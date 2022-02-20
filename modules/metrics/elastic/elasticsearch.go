package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
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
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	return me, nil
}

func (m *Metric) Collect() error {

	if !m.Enabled {
		return nil
	}

	elastic.WalkMetadata(func(key, value interface{}) bool {
		log.Trace("walking:",key)
		k := key.(string)
		if value == nil {
			log.Error("nil value:",key)
			return true
		}
		v, ok := value.(*elastic.ElasticsearchMetadata)
		if ok {
			if !v.Config.Monitored || !v.Config.Enabled || !v.IsAvailable() {
				log.Debugf("cluster [%v] not enabled[%v] or monitored[%v] or not available[%v], skip collect",v.Config.Name,v.Config.Enabled,v.Config.Monitored,v.IsAvailable())
				return true
			}
			if global.Env().IsDebug{
				log.Debugf("run monitoring task for elasticsearch: %v - %v",k,v.Config.Name)
			}

			client := elastic.GetClient(k)
			//var clusterUUID string
			//TODO fetch cluster_uuid?

			if m.ClusterHealth{
				health,err:=client.ClusterHealth()
				if err != nil {
					log.Error(v.Config.Name, " get cluster health error: ", err)
					return true
				}

				item := event.Event{
					Metadata: event.EventMetadata{
						Category: "elasticsearch",
						Name:     "cluster_health",
						Datatype: "snapshot",
						Labels: util.MapStr{
							"cluster_id":   v.Config.ID,
						},
					},
				}
				item.Fields = util.MapStr{
					"elasticsearch": util.MapStr{
						"cluster_health": health,
					},
				}
				event.Save(item)
			}

			//cluster stats
			if m.ClusterStats{
				stats,err:=client.GetClusterStats("")
				if err != nil {
					log.Error(v.Config.Name, " get cluster stats error: ", err)
					return true
				}

				item := event.Event{
					Metadata: event.EventMetadata{
						Category: "elasticsearch",
						Name:     "cluster_stats",
						Datatype: "snapshot",
						Labels: util.MapStr{
							"cluster_id":   v.Config.ID,
						},
					},
				}
				item.Fields = util.MapStr{
					"elasticsearch": util.MapStr{
						"cluster_stats": stats,
					},
				}
				event.Save(item)
			}

			//nodes stats
			if m.NodeStats {
				if v.Nodes == nil {
					if global.Env().IsDebug{
						log.Debugf("elasticsearch: %v - %v, no nodes info was found, skip nodes metrics collect",k,v.Config.Name)
					}
					return true
				}
				for nodeID, y := range *v.Nodes {
					//get node level stats
					stats := client.GetNodesStats(nodeID,y.GetHttpPublishHost())

					m.SaveNodeStats(v.Config.ID,stats)

				}
			}

			//indices stats
			if m.AllIndexStats || m.IndexStats {
				indexStats, err := client.GetStats()
				if err != nil {
					log.Error(v.Config.Name, " get indices stats error: ", err)
					return true
				}
				indexInfos, err := client.GetIndices("")
				if err != nil {
					log.Error(v.Config.Name, " get indices info error: ", err)
					return true
				}
				shardInfos, err := client.GetPrimaryShards()
				if err != nil {
					log.Error(v.Config.Name, " get shards info error: ", err)
					return true
				}
				if indexStats != nil {

					if m.AllIndexStats {
						m.SaveIndexStats(v.Config.ID,  "_all", "_all", indexStats.All.Primaries, indexStats.All.Total, nil, nil)
					}

					if m.IndexStats {
						for x, y := range indexStats.Indices {
							indexInfo :=  (*indexInfos)[x]
							shardInfo := (*shardInfos)[x]
							m.SaveIndexStats(v.Config.ID,  y.Uuid, x, y.Primaries, y.Total, &indexInfo, shardInfo)
						}
					}
				}else{
					if global.Env().IsDebug{
						log.Debugf("elasticsearch: %v - %v, no index info was found, skip index metrics collect",k,v.Config.Name)
					}
				}
			}

			//cluster state
			//nodes info


		}
		return true
	})

	return nil
}

func (m *Metric) SaveNodeStats( clusterId string, stats *elastic.NodesStats){
	for e,f:=range stats.Nodes{
		//remove adaptive_selection
		x,ok:=f.(map[string]interface{})
		if ok{
			delete(x,"adaptive_selection")
			delete(x,"ingest")
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
					"node_id":   e,
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
		event.Save(item)
	}
}

func (m *Metric) SaveIndexStats(clusterId, indexID, indexName string, primary, total elastic.IndexLevelStats, info *elastic.IndexInfo, shardInfo map[int]elastic.ShardInfo) {

	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "index_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   clusterId,
				//"cluster_uuid": clusterId,

				"index_id":   util.StringDefault(indexID, indexName),
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

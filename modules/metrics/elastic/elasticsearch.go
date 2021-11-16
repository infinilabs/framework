package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

type Metric struct {
	Enabled   bool `config:"enabled"`

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
		k := key.(string)
		if value == nil {
			return true
		}
		v, ok := value.(*elastic.ElasticsearchMetadata)
		if ok {
			if !v.Config.Monitored || !v.Config.Enabled || !v.IsAvailable() {
				return true
			}
			log.Tracef("run monitoring task for elasticsearch: " + k)
			client := elastic.GetClient(k)
			var clusterUUID string
			//TODO fetch cluster_uuid?

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
					return true
				}
				for x, y := range *v.Nodes {
					//get node level stats
					//TODO routing request to specify host
					stats := client.GetNodesStats(x)
					for e,f:=range stats.Nodes{
						item := event.Event{
							Metadata: event.EventMetadata{
								Category: "elasticsearch",
								Name:     "node_stats",
								Datatype: "snapshot",
								Labels: util.MapStr{
									"cluster_id":   v.Config.ID,
									//"cluster_uuid": stats.ClusterUUID,
									"node_id":   e,
									"node_name": y.Name,
									"ip": y.Ip,
									"transport_address":   y.TransportAddress,
								},
							},
						}
						item.Fields = util.MapStr{
							"elasticsearch": util.MapStr{
								"node_stats": f,
							},
						}
						event.Save(item)
					}
				}
			}

			//indices stats
			if m.AllIndexStats || m.IndexStats {
				indexStats, err := client.GetStats()
				if err != nil {
					log.Error(v.Config.Name, " get indices stats error: ", err)
					return true
				}
				if indexStats != nil {

					if m.AllIndexStats {
						m.saveIndexStats(v.Config.ID, clusterUUID, "_all", "_all", indexStats.All.Primaries, indexStats.All.Total)
					}

					if m.IndexStats {
						for x, y := range indexStats.Indices {
							m.saveIndexStats(v.Config.ID, clusterUUID, y.Uuid, x, y.Primaries, y.Total)
						}
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

func (m *Metric) saveIndexStats(clusterUUID, clusterId, uuid, name string, primary, total elastic.IndexLevelStats) {

	item := event.Event{
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name:     "index_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   clusterUUID,
				//"cluster_uuid": clusterId,

				"index_id":   util.StringDefault(uuid,name),
				"index_name": name,
			},
		},
	}

	mtr := util.MapStr{}
	if m.IndexPrimaryStats {
		mtr["primaries"] = primary
		mtr["total"] = total
	}

	item.Fields = util.MapStr{
		"elasticsearch": util.MapStr{
			"index_stats": mtr,
		},
	}

	event.Save(item)
}

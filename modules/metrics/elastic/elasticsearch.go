package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/metrics"
	"infini.sh/framework/core/util"
)

type Metric struct {
	Enabled   bool `config:"enabled"`
	NodeStats bool `config:"node_stats"`

	IndexStats    bool `config:"index_stats"`
	AllIndexStats bool `config:"all_index_stats"`

	IndexPrimaryStats bool `config:"index_primary_stats"`
	IndexTotalStats   bool `config:"index_total_stats"`
}

//GET _cluster/stats
//GET _stats
//GET /_nodes/_local
//GET /_nodes/_local/stats
//GET　/_nodes/_all/stats
//GET /_cluster/state/version,nodes,master_node,routing_table

func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		NodeStats:         true,
		IndexStats:        true,
		AllIndexStats:     true,
		IndexPrimaryStats: true,
		IndexTotalStats:   true,
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

			//nodes
			if m.NodeStats {
				if v.Nodes == nil {
					return true
				}
				for x, y := range *v.Nodes {
					stats := client.GetClusterStats(x)
					clusterUUID=stats.ClusterUUID
					item := metrics.MetricEvent{
						Metadata: metrics.EventMetadata{
							Category: "elasticsearch",
							Name:     "node_stats",
							Datatype: "snapshot",
							Labels: util.MapStr{
								"cluster_id":   v.Config.ID,
								"cluster_uuid": stats.ClusterUUID,

								"node_id":   x,
								"node_name": y.Name,
								"node_ip":   y.Ip,
							},
						},
					}
					item.Metric = util.MapStr{
						"elasticsearch": util.MapStr{
							"node_stats": stats,
						},
					}
					metrics.Save(item)
				}
			}

			//indices
			if m.AllIndexStats || m.IndexStats {
				indexStats, err := client.GetStats()
				if err != nil {
					log.Error(v.Config.Name, " get cluster stats error: ", err)
					return true
				}
				if indexStats != nil {

					if m.AllIndexStats {
						m.saveIndexStats(v.Config.ID,clusterUUID, "_all", "_all", indexStats.All.Primaries, indexStats.All.Total)
					}

					if m.IndexStats {
						for x, y := range indexStats.Indices {
							m.saveIndexStats(v.Config.ID,clusterUUID, y.Uuid, x, y.Primaries, y.Total)
						}
					}
				}

			}
		}
		return true
	})

	return nil
}

func (m *Metric) saveIndexStats(clusterUUID,clusterId, uuid, name string, primary, total elastic.IndexLevelStats) {

	item := metrics.MetricEvent{
		Metadata: metrics.EventMetadata{
			Category: "elasticsearch",
			Name:     "index_stats",
			Datatype: "snapshot",
			Labels: util.MapStr{
				"cluster_id":   clusterUUID,
				"cluster_uuid": clusterId,

				"index_id":   uuid,
				"index_name": name,
			},
		},
	}

	mtr:=util.MapStr{}
	if m.IndexPrimaryStats{
		mtr["primaries"]=primary
		mtr["total"]=total
	}

	item.Metric = util.MapStr{
		"elasticsearch": util.MapStr{
			"index_stats": mtr,
		},
	}

	metrics.Save(item)
}

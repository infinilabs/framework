package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/metrics"
	"infini.sh/framework/core/util"
)

type Metric struct {
	Enabled    bool     `config:"enabled"`
	NodeStats bool `config:"node_stats"`
}

func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		NodeStats:      true,
	}

	err:=cfg.Unpack(&me)
	if err!=nil{
		panic(err)
	}

	return me, nil
}

func (m *Metric) Collect() error {

	if !m.Enabled{
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

			if v.Nodes == nil {
				return true
			}
			for x, y := range *v.Nodes {
				stats := client.GetClusterStats(x)

				//indexStats, err := client.GetStats()
				//if err != nil {
				//	log.Error(v.Config.Name, " get cluster stats error: ", err)
				//	return true
				//}

				item := metrics.MetricEvent{
					Metadata: metrics.EventMetadata{
						Category: "elasticsearch",
						Name:     "node_stats",
						Datatype: "snapshot",
						Labels: util.MapStr{
							"cluster_id": v.Config.ID,
							"node_id": x,
							"node_name": y.Name,
							"node_ip": y.Ip,
						},
					},
				}
				item.MetricData = util.MapStr{
					"elasticsearch": util.MapStr{
						"node_stats": stats,
					},
				}
				//if indexStats != nil {
				//
				//	//replace . to _dot_
				//	for k, v := range indexStats.Indices {
				//		if util.PrefixStr(k, ".") {
				//			delete(indexStats.Indices, k)
				//			indexStats.Indices[strings.Replace(k, ".", "_dot_", 1)] = v
				//		}
				//	}
				//
				//	item.IndexStats = indexStats
				//}
				metrics.Save(item)
			}

		}
		return true
	})

	return nil
}

package api

import (
	"fmt"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
)

func (h *APIHandler) getQueueMetrics(clusterID string, bucketSize int, min, max int64, nodeName string, top int) map[string]*common.MetricItem{
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	var must = []util.MapStr{
		{
			"term":util.MapStr{
				"metadata.labels.cluster_id":util.MapStr{
					"value": clusterID,
				},
			},
		},
		{
			"term": util.MapStr{
				"metadata.category": util.MapStr{
					"value": "elasticsearch",
				},
			},
		},
		{
			"term": util.MapStr{
				"metadata.name": util.MapStr{
					"value": "node_stats",
				},
			},
		},
	}
	if nodeName != "" {
		top = 1
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"metadata.labels.transport_address": util.MapStr{
					"value": nodeName,
				},
			},
		})
	}

	query:=map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": must,
			"filter": []util.MapStr{
				{
					"range": util.MapStr{
						"timestamp": util.MapStr{
							"gte": min,
							"lte": max,
						},
					},
				},
			},
		},
	}
	searchThreadsMetric := newMetricItem("search_threads", 1, SystemGroupKey)
	searchThreadsMetric.AddAxi("Search Threads Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems := []GroupMetricItem{
		{
			Key: "search_threads",
			Field: "payload.elasticsearch.node_stats.thread_pool.search.threads",
			ID: util.GetUUID(),
			IsDerivative: false,
			MetricItem: searchThreadsMetric,
			FormatType: "num",
			Units: "",
		},
	}
	//Get Thread Pool queue
	aggs:=map[string]interface{}{}

	for _,metricItem:=range queueMetricItems{
		aggs[metricItem.ID]=util.MapStr{
			"max":util.MapStr{
				"field": metricItem.Field,
			},
		}
		if metricItem.Field2 != "" {
			aggs[metricItem.ID + "_field2"]=util.MapStr{
				"max":util.MapStr{
					"field": metricItem.Field2,
				},
			}
		}

		if metricItem.IsDerivative{
			aggs[metricItem.ID+"_deriv"]=util.MapStr{
				"derivative":util.MapStr{
					"buckets_path": metricItem.ID,
				},
			}
			if metricItem.Field2 != "" {
				aggs[metricItem.ID + "_field2_deriv"]=util.MapStr{
					"derivative":util.MapStr{
						"buckets_path": metricItem.ID + "_field2",
					},
				}
			}
		}
	}

	query["size"]=0
	query["aggs"]= util.MapStr{
		"group_by_level": util.MapStr{
			"terms": util.MapStr{
				"field": "metadata.labels.transport_address",
				"size":  top,
			},
			"aggs": util.MapStr{
				"dates": util.MapStr{
					"date_histogram":util.MapStr{
						"field": "timestamp",
						"fixed_interval": bucketSizeStr,
					},
					"aggs":aggs,
				},
			},
		},
	}
	return h.getMetrics(query, queueMetricItems, bucketSize)
}
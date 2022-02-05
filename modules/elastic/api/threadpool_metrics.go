package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
)

const (
	ThreadPool_GetGroupKey = "thread_pool_get"
	ThreadPool_SearchGroupKey = "thread_pool_search"
	ThreadPool_FlushGroupKey = "thread_pool_flush"
	ThreadPool_RefreshGroupKey = "thread_pool_refresh"
	ThreadPool_WriteGroupKey = "thread_pool_write"
	ThreadPool_ForceMergeGroupKey = "thread_pool_force_merge"
	ThreadPool_IndexGroupKey = "thread_pool_index"
	ThreadPool_BulkGroupKey = "thread_pool_bulk"
)

func (h *APIHandler) getThradPoolMetrics(clusterID string, bucketSize int, min, max int64, nodeName string, top int) map[string]*common.MetricItem{
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
	}else{
		nodeNames, err := h.getTopNodeName(clusterID, top, 15)
		if err != nil {
			log.Error(err)
		}
		if len(nodeNames) > 0 {
			must = append(must, util.MapStr{
				"terms": util.MapStr{
					"metadata.labels.transport_address": nodeNames,
				},
			})
		}
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
	searchThreadsMetric := newMetricItem("search_threads", 1, ThreadPool_SearchGroupKey)
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
	searchQueueMetric := newMetricItem("search_queue", 1, ThreadPool_SearchGroupKey)
	searchQueueMetric.AddAxi("Search Queue Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "search_queue",
		Field: "payload.elasticsearch.node_stats.thread_pool.search.queue",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: searchQueueMetric,
		FormatType: "num",
		Units: "",
	})
	searchActiveMetric := newMetricItem("search_active", 1, ThreadPool_SearchGroupKey)
	searchActiveMetric.AddAxi("Search Active Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "search_active",
		Field: "payload.elasticsearch.node_stats.thread_pool.search.active",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: searchActiveMetric,
		FormatType: "num",
		Units: "",
	})
	searchRejectedMetric := newMetricItem("search_rejected", 1, ThreadPool_SearchGroupKey)
	searchRejectedMetric.AddAxi("Search Rejected Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "search_rejected",
		Field: "payload.elasticsearch.node_stats.thread_pool.search.rejected",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: searchRejectedMetric,
		FormatType: "num",
		Units: "rejected/s",
	})

	getThreadsMetric := newMetricItem("get_threads", 1, ThreadPool_GetGroupKey)
	getThreadsMetric.AddAxi("Get Threads Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "get_threads",
		Field: "payload.elasticsearch.node_stats.thread_pool.get.threads",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: getThreadsMetric,
		FormatType: "num",
		Units: "",
	})
	getQueueMetric := newMetricItem("get_queue", 1, ThreadPool_GetGroupKey)
	getQueueMetric.AddAxi("Get Queue Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "get_queue",
		Field: "payload.elasticsearch.node_stats.thread_pool.get.queue",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: getQueueMetric,
		FormatType: "num",
		Units: "",
	})
	getActiveMetric := newMetricItem("get_active", 1, ThreadPool_GetGroupKey)
	getActiveMetric.AddAxi("Get Active Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "get_active",
		Field: "payload.elasticsearch.node_stats.thread_pool.get.active",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: getActiveMetric,
		FormatType: "num",
		Units: "",
	})
	getRejectedMetric := newMetricItem("get_rejected", 1, ThreadPool_GetGroupKey)
	getRejectedMetric.AddAxi("Get Rejected Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "get_rejected",
		Field: "payload.elasticsearch.node_stats.thread_pool.get.rejected",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: getRejectedMetric,
		FormatType: "num",
		Units: "rejected/s",
	})

	flushThreadsMetric := newMetricItem("flush_threads", 1, ThreadPool_FlushGroupKey)
	flushThreadsMetric.AddAxi("Flush Threads Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "flush_threads",
		Field: "payload.elasticsearch.node_stats.thread_pool.flush.threads",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: flushThreadsMetric,
		FormatType: "num",
		Units: "",
	})
	flushQueueMetric := newMetricItem("flush_queue", 1, ThreadPool_FlushGroupKey)
	flushQueueMetric.AddAxi("Get Queue Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "flush_queue",
		Field: "payload.elasticsearch.node_stats.thread_pool.flush.queue",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: flushQueueMetric,
		FormatType: "num",
		Units: "",
	})
	flushActiveMetric := newMetricItem("flush_active", 1, ThreadPool_FlushGroupKey)
	flushActiveMetric.AddAxi("Flush Active Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "flush_active",
		Field: "payload.elasticsearch.node_stats.thread_pool.flush.active",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: flushActiveMetric,
		FormatType: "num",
		Units: "",
	})
	flushRejectedMetric := newMetricItem("flush_rejected", 1, ThreadPool_FlushGroupKey)
	flushRejectedMetric.AddAxi("Flush Rejected Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "flush_rejected",
		Field: "payload.elasticsearch.node_stats.thread_pool.flush.rejected",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: flushRejectedMetric,
		FormatType: "num",
		Units: "rejected/s",
	})

	majorVersion := elastic.GetMetadata(clusterID).GetMajorVersion()
	
	if majorVersion < 6 {
		indexThreadsMetric := newMetricItem("index_threads", 1, ThreadPool_IndexGroupKey)
		indexThreadsMetric.AddAxi("Index Threads Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "index_threads",
			Field:        "payload.elasticsearch.node_stats.thread_pool.index.threads",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   indexThreadsMetric,
			FormatType:   "num",
			Units:        "",
		})
		indexQueueMetric := newMetricItem("index_queue", 1, ThreadPool_IndexGroupKey)
		indexQueueMetric.AddAxi("Index Queue Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "index_queue",
			Field:        "payload.elasticsearch.node_stats.thread_pool.index.queue",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   indexQueueMetric,
			FormatType:   "num",
			Units:        "",
		})
		indexActiveMetric := newMetricItem("index_active", 1, ThreadPool_IndexGroupKey)
		indexActiveMetric.AddAxi("Index Active Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "index_active",
			Field:        "payload.elasticsearch.node_stats.thread_pool.index.active",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   indexActiveMetric,
			FormatType:   "num",
			Units:        "",
		})
		indexRejectedMetric := newMetricItem("index_rejected", 1, ThreadPool_IndexGroupKey)
		indexRejectedMetric.AddAxi("Index Rejected Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "index_rejected",
			Field:        "payload.elasticsearch.node_stats.thread_pool.index.rejected",
			ID:           util.GetUUID(),
			IsDerivative: true,
			MetricItem:   indexRejectedMetric,
			FormatType:   "num",
			Units:        "rejected/s",
		})

		bulkThreadsMetric := newMetricItem("bulk_threads", 1, ThreadPool_BulkGroupKey)
		bulkThreadsMetric.AddAxi("Bulk Threads Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "bulk_threads",
			Field:        "payload.elasticsearch.node_stats.thread_pool.bulk.threads",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   bulkThreadsMetric,
			FormatType:   "num",
			Units:        "",
		})
		bulkQueueMetric := newMetricItem("bulk_queue", 1, ThreadPool_BulkGroupKey)
		bulkQueueMetric.AddAxi("Bulk Queue Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "bulk_queue",
			Field:        "payload.elasticsearch.node_stats.thread_pool.bulk.queue",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   bulkQueueMetric,
			FormatType:   "num",
			Units:        "",
		})
		bulkActiveMetric := newMetricItem("bulk_active", 1, ThreadPool_BulkGroupKey)
		bulkActiveMetric.AddAxi("Bulk Active Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "bulk_active",
			Field:        "payload.elasticsearch.node_stats.thread_pool.bulk.active",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   bulkActiveMetric,
			FormatType:   "num",
			Units:        "",
		})
		bulkRejectedMetric := newMetricItem("bulk_rejected", 1, ThreadPool_BulkGroupKey)
		bulkRejectedMetric.AddAxi("Bulk Rejected Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "bulk_rejected",
			Field:        "payload.elasticsearch.node_stats.thread_pool.bulk.rejected",
			ID:           util.GetUUID(),
			IsDerivative: true,
			MetricItem:   bulkRejectedMetric,
			FormatType:   "num",
			Units:        "rejected/s",
		})
	}else {
		writeThreadsMetric := newMetricItem("write_threads", 1, ThreadPool_WriteGroupKey)
		writeThreadsMetric.AddAxi("Write Threads Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "write_threads",
			Field:        "payload.elasticsearch.node_stats.thread_pool.write.threads",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   writeThreadsMetric,
			FormatType:   "num",
			Units:        "",
		})
		writeQueueMetric := newMetricItem("write_queue", 1, ThreadPool_WriteGroupKey)
		writeQueueMetric.AddAxi("Write Queue Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "write_queue",
			Field:        "payload.elasticsearch.node_stats.thread_pool.write.queue",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   writeQueueMetric,
			FormatType:   "num",
			Units:        "",
		})
		writeActiveMetric := newMetricItem("write_active", 1, ThreadPool_WriteGroupKey)
		writeActiveMetric.AddAxi("Write Active Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "write_active",
			Field:        "payload.elasticsearch.node_stats.thread_pool.write.active",
			ID:           util.GetUUID(),
			IsDerivative: false,
			MetricItem:   writeActiveMetric,
			FormatType:   "num",
			Units:        "",
		})
		writeRejectedMetric := newMetricItem("write_rejected", 1, ThreadPool_WriteGroupKey)
		writeRejectedMetric.AddAxi("Write Rejected Count", "group1", common.PositionLeft, "num", "0.[0]", "0.[0]", 5, true)

		queueMetricItems = append(queueMetricItems, GroupMetricItem{
			Key:          "write_rejected",
			Field:        "payload.elasticsearch.node_stats.thread_pool.write.rejected",
			ID:           util.GetUUID(),
			IsDerivative: true,
			MetricItem:   writeRejectedMetric,
			FormatType:   "num",
			Units:        "rejected/s",
		})
	}
	refreshThreadsMetric := newMetricItem("refresh_threads", 1, ThreadPool_RefreshGroupKey)
	refreshThreadsMetric.AddAxi("Refresh Threads Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "refresh_threads",
		Field: "payload.elasticsearch.node_stats.thread_pool.refresh.threads",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: refreshThreadsMetric,
		FormatType: "num",
		Units: "",
	})
	refreshQueueMetric := newMetricItem("refresh_queue", 1, ThreadPool_RefreshGroupKey)
	refreshQueueMetric.AddAxi("Refresh Queue Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "refresh_queue",
		Field: "payload.elasticsearch.node_stats.thread_pool.refresh.queue",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: refreshQueueMetric,
		FormatType: "num",
		Units: "",
	})
	refreshActiveMetric := newMetricItem("refresh_active", 1, ThreadPool_RefreshGroupKey)
	refreshActiveMetric.AddAxi("Refresh Active Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "refresh_active",
		Field: "payload.elasticsearch.node_stats.thread_pool.refresh.active",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: refreshActiveMetric,
		FormatType: "num",
		Units: "",
	})
	refreshRejectedMetric := newMetricItem("refresh_rejected", 1, ThreadPool_RefreshGroupKey)
	refreshRejectedMetric.AddAxi("Refresh Rejected Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "refresh_rejected",
		Field: "payload.elasticsearch.node_stats.thread_pool.refresh.rejected",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: refreshRejectedMetric,
		FormatType: "num",
		Units: "rejected/s",
	})
	forceMergeThreadsMetric := newMetricItem("force_merge_threads", 1, ThreadPool_ForceMergeGroupKey)
	forceMergeThreadsMetric.AddAxi("Force Merge Threads Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "force_merge_threads",
		Field: "payload.elasticsearch.node_stats.thread_pool.force_merge.threads",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: forceMergeThreadsMetric,
		FormatType: "num",
		Units: "",
	})
	forceMergeQueueMetric := newMetricItem("force_merge_queue", 1, ThreadPool_ForceMergeGroupKey)
	forceMergeQueueMetric.AddAxi("Force Merge Queue Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "force_merge_queue",
		Field: "payload.elasticsearch.node_stats.thread_pool.force_merge.queue",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: forceMergeQueueMetric,
		FormatType: "num",
		Units: "",
	})
	forceMergeActiveMetric := newMetricItem("force_merge_active", 1, ThreadPool_ForceMergeGroupKey)
	forceMergeActiveMetric.AddAxi("Force Merge Active Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "force_merge_active",
		Field: "payload.elasticsearch.node_stats.thread_pool.force_merge.active",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: forceMergeActiveMetric,
		FormatType: "num",
		Units: "",
	})
	forceMergeRejectedMetric := newMetricItem("force_merge_rejected", 1, ThreadPool_ForceMergeGroupKey)
	forceMergeRejectedMetric.AddAxi("Force Merge Rejected Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	queueMetricItems = append(queueMetricItems, GroupMetricItem{
		Key: "force_merge_rejected",
		Field: "payload.elasticsearch.node_stats.thread_pool.force_merge.rejected",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: forceMergeRejectedMetric,
		FormatType: "num",
		Units: "rejected/s",
	})
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

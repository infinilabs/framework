package api

import (
	"fmt"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
)

func (h *APIHandler) getIndexMetrics(clusterID string, bucketSize int, min, max int64, indexName string, top int) map[string]*common.MetricItem{
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
					"value": "index_stats",
				},
			},
		},
	}
	if indexName != "" {
		top = 1
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"metadata.labels.index_name": util.MapStr{
					"value": indexName,
				},
			},
		})
	}

	query:=map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": must,
			"must_not": []util.MapStr{
				{
					"term": util.MapStr{
						"metadata.labels.index_name": util.MapStr{
							"value": "_all",
						},
					},
				},
			},
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
	//索引存储大小
	indexStorageMetric := newMetricItem("index_storage", 1, StorageGroupKey)
	indexStorageMetric.AddAxi("Index storage","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)

	indexMetricItems := []GroupMetricItem{
		{
			Key: "index_storage",
			Field: "payload.elasticsearch.index_stats.total.store.size_in_bytes",
			ID: util.GetUUID(),
			IsDerivative: false,
			MetricItem: indexStorageMetric,
			FormatType: "bytes",
			Units: "",
		},
	}
	//索引文档个数
	docCountMetric := newMetricItem("doc_count", 2, StorageGroupKey)
	docCountMetric.AddAxi("Doc count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "doc_count",
		Field: "payload.elasticsearch.index_stats.total.docs.count",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: docCountMetric,
		FormatType: "num",
		Units: "",
	})
	//查询次数
	queryTimesMetric := newMetricItem("query_times", 3, OperationGroupKey)
	queryTimesMetric.AddAxi("Query times","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "query_times",
		Field: "payload.elasticsearch.index_stats.total.search.query_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryTimesMetric,
		FormatType: "num",
		Units: "",
	})

	//Fetch次数
	fetchTimesMetric := newMetricItem("fetch_times", 4, OperationGroupKey)
	fetchTimesMetric.AddAxi("Fetch times","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "fetch_times",
		Field: "payload.elasticsearch.index_stats.total.search.fetch_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: fetchTimesMetric,
		FormatType: "num",
		Units: "",
	})
	//Merge次数
	mergeTimesMetric := newMetricItem("merge_times", 5, OperationGroupKey)
	mergeTimesMetric.AddAxi("Merge times","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "merge_times",
		Field: "payload.elasticsearch.index_stats.total.merges.total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: mergeTimesMetric,
		FormatType: "num",
		Units: "",
	})
	//Refresh次数
	refreshTimesMetric := newMetricItem("refresh_times", 6, OperationGroupKey)
	refreshTimesMetric.AddAxi("Refresh times","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "refresh_times",
		Field: "payload.elasticsearch.index_stats.total.refresh.total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: refreshTimesMetric,
		FormatType: "num",
		Units: "",
	})
	//写入速率
	indexingRateMetric := newMetricItem("indexing_rate", 7, OperationGroupKey)
	indexingRateMetric.AddAxi("Indexing rate","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "indexing_rate",
		Field: "payload.elasticsearch.index_stats.total.indexing.index_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexingRateMetric,
		FormatType: "num",
		Units: "",
	})
	//写入时延
	indexingLatencyMetric := newMetricItem("indexing_latency", 8, LatencyGroupKey)
	indexingLatencyMetric.AddAxi("Indexing latency","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "indexing_latency",
		Field: "payload.elasticsearch.index_stats.total.indexing.index_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexingLatencyMetric,
		FormatType: "num",
		Units: "",
	})

	//查询时延
	queryLatencyMetric := newMetricItem("query_latency", 9, LatencyGroupKey)
	queryLatencyMetric.AddAxi("Query latency","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "query_latency",
		Field: "payload.elasticsearch.index_stats.total.search.query_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryLatencyMetric,
		FormatType: "num",
		Units: "",
	})
	//fetch时延
	fetchLatencyMetric := newMetricItem("fetch_latency", 9, LatencyGroupKey)
	fetchLatencyMetric.AddAxi("Fetch latency","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "fetch_latency",
		Field: "payload.elasticsearch.index_stats.total.search.fetch_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: fetchLatencyMetric,
		FormatType: "num",
		Units: "",
	})

	//merge时延
	mergeLatencyMetric := newMetricItem("merge_latency", 10, LatencyGroupKey)
	mergeLatencyMetric.AddAxi("Merge latency","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "merge_latency",
		Field: "payload.elasticsearch.index_stats.total.merges.total_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: mergeLatencyMetric,
		FormatType: "num",
		Units: "",
	})
	//refresh时延
	refreshLatencyMetric := newMetricItem("refresh_latency", 11, LatencyGroupKey)
	refreshLatencyMetric.AddAxi("Refresh latency","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "refresh_latency",
		Field: "payload.elasticsearch.index_stats.total.refresh.total_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: refreshLatencyMetric,
		FormatType: "num",
		Units: "",
	})
	//queryCache
	queryCacheMetric := newMetricItem("query_cache", 12, CacheGroupKey)
	queryCacheMetric.AddAxi("Query cache","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "query_cache",
		Field: "payload.elasticsearch.index_stats.total.query_cache.memory_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: queryCacheMetric,
		FormatType: "bytes",
		Units: "",
	})
	//requestCache
	requestCacheMetric := newMetricItem("request_cache", 12, CacheGroupKey)
	requestCacheMetric.AddAxi("request cache","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "request_cache",
		Field: "payload.elasticsearch.index_stats.total.request_cache.memory_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: requestCacheMetric,
		FormatType: "bytes",
		Units: "",
	})
	//segment memory
	segmentMemoryMetric := newMetricItem("segment_memory", 13, MemoryGroupKey)
	segmentMemoryMetric.AddAxi("Segment memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "segment_memory",
		Field: "payload.elasticsearch.index_stats.total.segments.memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})

	//segment doc values memory
	docValuesMemoryMetric := newMetricItem("segment_doc_values_memory", 13, MemoryGroupKey)
	docValuesMemoryMetric.AddAxi("Segment Doc values Memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "segment_doc_values_memory",
		Field: "payload.elasticsearch.index_stats.total.segments.doc_values_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: docValuesMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})

	//segment terms memory
	termsMemoryMetric := newMetricItem("segment_terms_memory", 13, MemoryGroupKey)
	termsMemoryMetric.AddAxi("Segment Terms Memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "segment_terms_memory",
		Field: "payload.elasticsearch.index_stats.total.segments.terms_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: termsMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})

	//segment fields memory
	fieldsMemoryMetric := newMetricItem("segment_fields_memory", 13, MemoryGroupKey)
	fieldsMemoryMetric.AddAxi("Segment Fields Memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	indexMetricItems = append(indexMetricItems, GroupMetricItem{
		Key: "segment_fields_memory",
		Field: "payload.elasticsearch.index_stats.total.segments.stored_fields_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: fieldsMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})

	aggs:=map[string]interface{}{}

	for _,metricItem:=range indexMetricItems {
		aggs[metricItem.ID]=util.MapStr{
			"max":util.MapStr{
				"field": metricItem.Field,
			},
		}

		if metricItem.IsDerivative{
			aggs[metricItem.ID+"_deriv"]=util.MapStr{
				"derivative":util.MapStr{
					"buckets_path": metricItem.ID,
				},
			}
		}
	}

	query["size"]=0
	query["aggs"]= util.MapStr{
		"group_by_level": util.MapStr{
			"terms": util.MapStr{
				"field": "metadata.labels.index_name",
				"size":  top,
				"order": util.MapStr{
					"max_store": "desc",
				},
			},
			"aggs": util.MapStr{
				"dates": util.MapStr{
					"date_histogram":util.MapStr{
						"field": "timestamp",
						"fixed_interval": bucketSizeStr,
					},
					"aggs":aggs,
				},
				"max_store": util.MapStr{
					"max": util.MapStr{
						"field": "payload.elasticsearch.index_stats.total.store.size_in_bytes",
					},
				},
			},
		},
	}
	return h.getMetrics(query, indexMetricItems, bucketSize)

}


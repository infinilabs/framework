package api

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"sort"
	log "github.com/cihub/seelog"
	"strings"
	"time"
)

func (h *APIHandler) getNodeMetrics(clusterID string, bucketSize int, min, max int64, nodeName string, top int) map[string]*common.MetricItem{
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	nodeNames, err := h.getTopNodeName(clusterID, top, 15)
	if err != nil {
		log.Error(err)
	}
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
	cpuMetric := newMetricItem("cpu", 1, SystemGroupKey)
	cpuMetric.AddAxi("cpu","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)

	nodeMetricItems := []GroupMetricItem{
		{
			Key: "cpu",
			Field: "payload.elasticsearch.node_stats.process.cpu.percent",
			ID: util.GetUUID(),
			IsDerivative: false,
			MetricItem: cpuMetric,
			FormatType: "ratio",
			Units: "%",
		},
	}

	osCpuMetric := newMetricItem("os_cpu", 2, SystemGroupKey)
	osCpuMetric.AddAxi("OS CPU Percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "os_cpu",
		Field: "payload.elasticsearch.node_stats.os.cpu.percent",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: osCpuMetric,
		FormatType: "ratio",
		Units: "%",
	})

	osMemMetric := newMetricItem("os_used_mem", 2, SystemGroupKey)
	osMemMetric.AddAxi("OS Mem Used Percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "os_used_mem",
		Field: "payload.elasticsearch.node_stats.os.mem.used_percent",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: osMemMetric,
		FormatType: "ratio",
		Units: "%",
	})
	//swap usage
	osSwapMetric := newMetricItem("os_used_swap", 2, SystemGroupKey)
	osSwapMetric.AddAxi("OS Swap Used Percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "os_used_swap",
		Field: "payload.elasticsearch.node_stats.os.swap.used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		Field2: "payload.elasticsearch.node_stats.os.swap.total_in_bytes",
		Calc: func(value, value2 float64) float64 {
			return util.ToFixed((value / value2)*100, 2)
		},
		MetricItem: osSwapMetric,
		FormatType: "ratio",
		Units: "%",
	})
	openFileMetric := newMetricItem("open_file", 2, SystemGroupKey)
	openFileMetric.AddAxi("Open File Count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "open_file",
		Field: "payload.elasticsearch.node_stats.process.open_file_descriptors",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: openFileMetric,
		FormatType: "num",
		Units: "",
	})
	openFilePercentMetric := newMetricItem("open_file_percent", 2, SystemGroupKey)
	openFilePercentMetric.AddAxi("Open File Percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "open_file_percent",
		Field: "payload.elasticsearch.node_stats.process.open_file_descriptors",
		ID: util.GetUUID(),
		IsDerivative: false,
		Field2: "payload.elasticsearch.node_stats.process.max_file_descriptors",
		Calc: func(value, value2 float64) float64 {
			if value < 0 {
				return value
			}
			return util.ToFixed((value / value2)*100, 2)
		},
		MetricItem: openFilePercentMetric,
		FormatType: "ratio",
		Units: "%",
	})

	diskMetric := newMetricItem("disk", 2, SystemGroupKey)
	diskMetric.AddAxi("disk available percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)

	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "disk",
		Field: "payload.elasticsearch.node_stats.fs.total.total_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: diskMetric,
		FormatType: "ratio",
		Units: "%",
		Field2: "payload.elasticsearch.node_stats.fs.total.available_in_bytes",
		Calc: func(value, value2 float64) float64 {
			return util.ToFixed((value2 / value)*100, 2)
		},
	})
	// 索引速率
	indexMetric:=newMetricItem("indexing_rate", 1, OperationGroupKey)
	indexMetric.AddAxi("indexing rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "indexing_rate",
		Field: "payload.elasticsearch.node_stats.indices.indexing.index_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexMetric,
		FormatType: "num",
		Units: "doc/s",
	})
	// 索引延时
	indexLatencyMetric:=newMetricItem("indexing_latency", 1, LatencyGroupKey)
	indexLatencyMetric.AddAxi("indexing latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "indexing_latency",
		Field: "payload.elasticsearch.node_stats.indices.indexing.index_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})

	queryMetric:=newMetricItem("query_rate", 2, OperationGroupKey)
	queryMetric.AddAxi("query rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_rate",
		Field: "payload.elasticsearch.node_stats.indices.search.query_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryMetric,
		FormatType: "num",
		Units: "requests/s",
	})

	// 查询延时
	queryLatencyMetric:=newMetricItem("query_latency", 2, LatencyGroupKey)
	queryLatencyMetric.AddAxi("query latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_latency",
		Field: "payload.elasticsearch.node_stats.indices.search.query_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})

	fetchMetric:=newMetricItem("fetch_rate", 3, OperationGroupKey)
	fetchMetric.AddAxi("fetch rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "fetch_rate",
		Field: "payload.elasticsearch.node_stats.indices.search.fetch_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: fetchMetric,
		FormatType: "num",
		Units: "requests/s",
	})
	scrollMetric:=newMetricItem("scroll_rate", 4, OperationGroupKey)
	scrollMetric.AddAxi("scroll rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "scroll_rate",
		Field: "payload.elasticsearch.node_stats.indices.search.scroll_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: scrollMetric,
		FormatType: "num",
		Units: "requests/s",
	})

	refreshMetric:=newMetricItem("refresh_rate", 5, OperationGroupKey)
	refreshMetric.AddAxi("refresh rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "refresh_rate",
		Field: "payload.elasticsearch.node_stats.indices.refresh.total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: refreshMetric,
		FormatType: "num",
		Units: "requests/s",
	})
	flushMetric:=newMetricItem("flush_rate", 6, OperationGroupKey)
	flushMetric.AddAxi("flush rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "flush_rate",
		Field: "payload.elasticsearch.node_stats.indices.flush.total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: flushMetric,
		FormatType: "num",
		Units: "requests/s",
	})
	mergeMetric:=newMetricItem("merges_rate", 7, OperationGroupKey)
	mergeMetric.AddAxi("merges rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "merges_rate",
		Field: "payload.elasticsearch.node_stats.indices.merges.total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: mergeMetric,
		FormatType: "num",
		Units: "requests/s",
	})

	// fetch延时
	fetchLatencyMetric:=newMetricItem("fetch_latency", 3, LatencyGroupKey)
	fetchLatencyMetric.AddAxi("fetch latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "fetch_latency",
		Field: "payload.elasticsearch.node_stats.indices.search.fetch_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: fetchLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})
	// scroll 延时
	scrollLatencyMetric:=newMetricItem("scroll_latency", 4, LatencyGroupKey)
	scrollLatencyMetric.AddAxi("scroll latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "scroll_latency",
		Field: "payload.elasticsearch.node_stats.indices.search.scroll_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: scrollLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})

	// merge 延时
	mergeLatencyMetric:=newMetricItem("merge_latency", 7, LatencyGroupKey)
	mergeLatencyMetric.AddAxi("merge latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "merge_latency",
		Field: "payload.elasticsearch.node_stats.indices.merges.total_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: mergeLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})

	// refresh 延时
	refreshLatencyMetric:=newMetricItem("refresh_latency", 5, LatencyGroupKey)
	refreshLatencyMetric.AddAxi("refresh latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "refresh_latency",
		Field: "payload.elasticsearch.node_stats.indices.refresh.total_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: refreshLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})
	// flush 时延
	flushLatencyMetric:=newMetricItem("flush_latency", 6, LatencyGroupKey)
	flushLatencyMetric.AddAxi("flush latency","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "flush_latency",
		Field: "payload.elasticsearch.node_stats.indices.flush.total_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: flushLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})
	// Query Cache 内存占用大小
	queryCacheMetric:=newMetricItem("query_cache", 1, CacheGroupKey)
	queryCacheMetric.AddAxi("query cache","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_cache",
		Field: "payload.elasticsearch.node_stats.indices.query_cache.memory_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: queryCacheMetric,
		FormatType: "bytes",
		Units: "",
	})
	// Request Cache 内存占用大小
	requestCacheMetric:=newMetricItem("request_cache", 2, CacheGroupKey)
	requestCacheMetric.AddAxi("request cache","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "request_cache",
		Field: "payload.elasticsearch.node_stats.indices.request_cache.memory_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: requestCacheMetric,
		FormatType: "bytes",
		Units: "",
	})
	// Request Cache Hit
	requestCacheHitMetric:=newMetricItem("request_cache_hit", 6, CacheGroupKey)
	requestCacheHitMetric.AddAxi("request cache hit","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "request_cache_hit",
		Field: "payload.elasticsearch.node_stats.indices.request_cache.hit_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: requestCacheHitMetric,
		FormatType: "num",
		Units: "hits",
	})
	// Request Cache Miss
	requestCacheMissMetric:=newMetricItem("request_cache_miss", 8, CacheGroupKey)
	requestCacheMissMetric.AddAxi("request cache miss","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "request_cache_miss",
		Field: "payload.elasticsearch.node_stats.indices.request_cache.miss_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: requestCacheMissMetric,
		FormatType: "num",
		Units: "misses",
	})
	// Query Cache Count
	queryCacheCountMetric:=newMetricItem("query_cache_count", 4, CacheGroupKey)
	queryCacheCountMetric.AddAxi("query cache miss","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_cache_count",
		Field: "payload.elasticsearch.node_stats.indices.query_cache.cache_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryCacheCountMetric,
		FormatType: "num",
		Units: "",
	})
	// Query Cache Miss
	queryCacheHitMetric:=newMetricItem("query_cache_hit", 5, CacheGroupKey)
	queryCacheHitMetric.AddAxi("query cache hit","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_cache_hit",
		Field: "payload.elasticsearch.node_stats.indices.query_cache.hit_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryCacheHitMetric,
		FormatType: "num",
		Units: "hits",
	})

	//// Query Cache evictions
	//queryCacheEvictionsMetric:=newMetricItem("query_cache_evictions", 5, CacheGroupKey)
	//queryCacheEvictionsMetric.AddAxi("query cache evictions","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	//nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
	//	Key: "query_cache_evictions",
	//	Field: "payload.elasticsearch.node_stats.indices.query_cache.evictions",
	//	ID: util.GetUUID(),
	//	IsDerivative: true,
	//	MetricItem: queryCacheEvictionsMetric,
	//	FormatType: "num",
	//	Units: "evictions",
	//})

	// Query Cache Miss
	queryCacheMissMetric:=newMetricItem("query_cache_miss", 7, CacheGroupKey)
	queryCacheMissMetric.AddAxi("query cache miss","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_cache_miss",
		Field: "payload.elasticsearch.node_stats.indices.query_cache.miss_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryCacheMissMetric,
		FormatType: "num",
		Units: "misses",
	})

	// Fielddata内存占用大小
	fieldDataCacheMetric:=newMetricItem("fielddata_cache", 3, CacheGroupKey)
	fieldDataCacheMetric.AddAxi("FieldData Cache","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "fielddata_cache",
		Field: "payload.elasticsearch.node_stats.indices.fielddata.memory_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: fieldDataCacheMetric,
		FormatType: "bytes",
		Units: "",
	})

	// http 活跃连接数
	httpActiveMetric:=newMetricItem("http_connect_num", 12, HttpGroupKey)
	httpActiveMetric.AddAxi("http connect number","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "http_connect_num",
		Field: "payload.elasticsearch.node_stats.http.current_open",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: httpActiveMetric,
		FormatType: "num",
		Units: "conns",
	})
	// http 活跃连接数速率
	httpRateMetric:=newMetricItem("http_rate", 12, HttpGroupKey)
	httpRateMetric.AddAxi("http rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "http_rate",
		Field: "payload.elasticsearch.node_stats.http.total_opened",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: httpRateMetric,
		FormatType: "num",
		Units: "conns/s",
	})

	// segment 数量
	segmentCountMetric:=newMetricItem("segment_count", 15, StorageGroupKey)
	segmentCountMetric.AddAxi("segment count","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_count",
		Field: "payload.elasticsearch.node_stats.indices.segments.count",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentCountMetric,
		FormatType: "num",
		Units: "",
	})

	// segment memory
	segmentMemoryMetric:=newMetricItem("segment_memory", 16, MemoryGroupKey)
	segmentMemoryMetric.AddAxi("segment memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})
	// segment stored fields memory
	segmentStoredFieldsMemoryMetric:=newMetricItem("segment_stored_fields_memory", 16, MemoryGroupKey)
	segmentStoredFieldsMemoryMetric.AddAxi("segment stored fields memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_stored_fields_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.stored_fields_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentStoredFieldsMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})
	// segment terms fields memory
	segmentTermsMemoryMetric:=newMetricItem("segment_terms_memory", 16, MemoryGroupKey)
	segmentTermsMemoryMetric.AddAxi("segment terms memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_terms_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.terms_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentTermsMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})
	// segment doc values memory
	segmentDocValuesMemoryMetric:=newMetricItem("segment_doc_values_memory", 16, MemoryGroupKey)
	segmentDocValuesMemoryMetric.AddAxi("segment doc values memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_doc_values_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.doc_values_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentDocValuesMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})
	// segment index writer memory
	segmentIndexWriterMemoryMetric:=newMetricItem("segment_index_writer_memory", 16, MemoryGroupKey)
	segmentIndexWriterMemoryMetric.AddAxi("segment doc values memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_index_writer_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.index_writer_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentIndexWriterMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})
	// segment term vectors memory
	segmentTermVectorsMemoryMetric:=newMetricItem("segment_term_vectors_memory", 16, MemoryGroupKey)
	segmentTermVectorsMemoryMetric.AddAxi("segment term vectors memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "segment_term_vectors_memory",
		Field: "payload.elasticsearch.node_stats.indices.segments.term_vectors_memory_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: segmentTermVectorsMemoryMetric,
		FormatType: "bytes",
		Units: "",
	})

	// indexing_pressure memory
	indexingPressureMemMetric:=newMetricItem("indexing_pressure_memory", 16, MemoryGroupKey)
	indexingPressureMemMetric.AddAxi("indexing pressure memory","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "indexing_pressure_memory",
		Field: "payload.elasticsearch.node_stats.indexing_pressure.memory.total.all_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: indexingPressureMemMetric,
		FormatType: "bytes",
		Units: "",
	})

	// docs 数量
	docsCountMetric:=newMetricItem("docs_count", 17, DocumentGroupKey)
	docsCountMetric.AddAxi("docs count","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "docs_count",
		Field: "payload.elasticsearch.node_stats.indices.docs.count",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: docsCountMetric,
		FormatType: "num",
		Units: "",
	})
	// docs 删除数量
	docsDeletedMetric:=newMetricItem("docs_deleted", 17, DocumentGroupKey)
	docsDeletedMetric.AddAxi("docs deleted","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "docs_deleted",
		Field: "payload.elasticsearch.node_stats.indices.docs.deleted",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: docsDeletedMetric,
		FormatType: "num",
		Units: "",
	})

	// index store size
	indexStoreMetric:=newMetricItem("index_storage", 18, StorageGroupKey)
	indexStoreMetric.AddAxi("indices storage","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "index_storage",
		Field: "payload.elasticsearch.node_stats.indices.store.size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: indexStoreMetric,
		FormatType: "bytes",
		Units: "",
	})

	// jvm used heap
	jvmUsedPercentMetric:=newMetricItem("jvm_heap_used_percent", 1, JVMGroupKey)
	jvmUsedPercentMetric.AddAxi("JVM heap used percent","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_heap_used_percent",
		Field: "payload.elasticsearch.node_stats.jvm.mem.heap_used_percent",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: jvmUsedPercentMetric,
		FormatType: "num",
		Units: "%",
	})
	//JVM mem Young pools used
	youngPoolsUsedMetric:=newMetricItem("jvm_mem_young_used", 2, JVMGroupKey)
	youngPoolsUsedMetric.AddAxi("Mem Pools Young Used","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_mem_young_used",
		Field: "payload.elasticsearch.node_stats.jvm.mem.pools.young.used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: youngPoolsUsedMetric,
		FormatType: "bytes",
		Units: "",
	})
	//JVM mem Young pools peak used
	youngPoolsUsedPeakMetric:=newMetricItem("jvm_mem_young_peak_used", 2, JVMGroupKey)
	youngPoolsUsedPeakMetric.AddAxi("Mem Pools Young Peak Used","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_mem_young_peak_used",
		Field: "payload.elasticsearch.node_stats.jvm.mem.pools.young.peak_used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: youngPoolsUsedPeakMetric,
		FormatType: "bytes",
		Units: "",
	})

	//JVM mem old pools used
	oldPoolsUsedMetric:=newMetricItem("jvm_mem_old_used", 3, JVMGroupKey)
	oldPoolsUsedMetric.AddAxi("Mem Pools Old Used","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_mem_old_used",
		Field: "payload.elasticsearch.node_stats.jvm.mem.pools.old.used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: oldPoolsUsedMetric,
		FormatType: "bytes",
		Units: "",
	})
	//JVM mem old pools peak used
	oldPoolsUsedPeakMetric:=newMetricItem("jvm_mem_old_peak_used", 3, JVMGroupKey)
	oldPoolsUsedPeakMetric.AddAxi("Mem Pools Old Peak Used","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_mem_old_peak_used",
		Field: "payload.elasticsearch.node_stats.jvm.mem.pools.old.peak_used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: oldPoolsUsedPeakMetric,
		FormatType: "bytes",
		Units: "",
	})

	//JVM used heap
	heapUsedMetric:=newMetricItem("jvm_used_heap", 1, JVMGroupKey)
	heapUsedMetric.AddAxi("JVM Used Heap","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_used_heap",
		Field: "payload.elasticsearch.node_stats.jvm.mem.heap_used_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: false,
		MetricItem: heapUsedMetric,
		FormatType: "bytes",
		Units: "",
	})
	//JVM Young GC Rate
	gcYoungRateMetric:=newMetricItem("jvm_young_gc_rate", 2, JVMGroupKey)
	gcYoungRateMetric.AddAxi("JVM Young GC Rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_young_gc_rate",
		Field: "payload.elasticsearch.node_stats.jvm.gc.collectors.young.collection_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: gcYoungRateMetric,
		FormatType: "num",
		Units: "times/s",
	})
	//JVM Young GC Latency
	gcYoungLatencyMetric:=newMetricItem("jvm_young_gc_latency", 2, JVMGroupKey)
	gcYoungLatencyMetric.AddAxi("JVM Young GC Time","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_young_gc_latency",
		Field: "payload.elasticsearch.node_stats.jvm.gc.collectors.young.collection_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: gcYoungLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})

	//JVM old GC Rate
	gcOldRateMetric:=newMetricItem("jvm_old_gc_rate", 3, JVMGroupKey)
	gcOldRateMetric.AddAxi("JVM Old GC Rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_old_gc_rate",
		Field: "payload.elasticsearch.node_stats.jvm.gc.collectors.old.collection_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: gcOldRateMetric,
		FormatType: "num",
		Units: "times/s",
	})
	//JVM old GC Latency
	gcOldLatencyMetric:=newMetricItem("jvm_old_gc_latency", 3, JVMGroupKey)
	gcOldLatencyMetric.AddAxi("JVM Old GC Time","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "jvm_old_gc_latency",
		Field: "payload.elasticsearch.node_stats.jvm.gc.collectors.old.collection_time_in_millis",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: gcOldLatencyMetric,
		FormatType: "num",
		Units: "ms",
	})
	//Transport 发送速率
	transTxRateMetric:=newMetricItem("transport_tx_rate", 19, TransportGroupKey)
	transTxRateMetric.AddAxi("Transport Send Rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "transport_tx_rate",
		Field: "payload.elasticsearch.node_stats.transport.tx_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: transTxRateMetric,
		FormatType: "num",
		Units: "times/s",
	})
	//Transport 接收速率
	transRxRateMetric:=newMetricItem("transport_rx_rate", 19, TransportGroupKey)
	transRxRateMetric.AddAxi("Transport Receive Rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "transport_rx_rate",
		Field: "payload.elasticsearch.node_stats.transport.rx_count",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: transRxRateMetric,
		FormatType: "num",
		Units: "times/s",
	})

	//Transport 发送流量
	transTxBytesMetric:=newMetricItem("transport_tx_bytes", 19, TransportGroupKey)
	transTxBytesMetric.AddAxi("Transport Send Bytes","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "transport_tx_bytes",
		Field: "payload.elasticsearch.node_stats.transport.tx_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: transTxBytesMetric,
		FormatType: "bytes",
		Units: "s",
	})
	//Transport 接收流量
	transRxBytesMetric:=newMetricItem("transport_rx_bytes", 19, TransportGroupKey)
	transRxBytesMetric.AddAxi("Transport Receive Bytes","group1",common.PositionLeft,"bytes","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "transport_rx_bytes",
		Field: "payload.elasticsearch.node_stats.transport.rx_size_in_bytes",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: transRxBytesMetric,
		FormatType: "bytes",
		Units: "s",
	})


	aggs:=map[string]interface{}{}

	for _,metricItem:=range nodeMetricItems{
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
	return h.getMetrics(query, nodeMetricItems, bucketSize)

}

type MetricData map[string][][]interface{}

func (h *APIHandler) getMetrics( query map[string]interface{}, grpMetricItems []GroupMetricItem, bucketSize int) map[string]*common.MetricItem{
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	response,err:=elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(),util.MustToJSONBytes(query))
	if err!=nil{
		log.Error(err)
	}
	grpMetricItemsIndex := map[string] int{}
	for i, item := range grpMetricItems {
		grpMetricItemsIndex[item.ID] = i
	}
	grpMetricData :=map[string]MetricData{}

	var minDate,maxDate int64
	if response.StatusCode==200{
		if nodeAgg, ok :=  response.Aggregations["group_by_level"]; ok {
			for _,bucket:=range nodeAgg.Buckets {
				grpKey := bucket["key"].(string)
				for _, metricItem := range grpMetricItems {
					metricItem.MetricItem.AddLine(metricItem.Key, grpKey, "", "group1", metricItem.Field, "max",bucketSizeStr,metricItem.Units,metricItem.FormatType,"0.[00]","0.[00]",false,false)
					dataKey := metricItem.ID
					if metricItem.IsDerivative {
						dataKey = dataKey+"_deriv"
					}
					if _, ok := grpMetricData[dataKey]; !ok{
						grpMetricData[dataKey] = map[string][][]interface{}{}
					}
					grpMetricData[dataKey][grpKey]=[][]interface{}{}
				}
				if datesAgg, ok := bucket["dates"].(map[string]interface{}); ok {
					if datesBuckets, ok := datesAgg["buckets"].([]interface{}); ok {
						for _, dateBucket := range datesBuckets {
							if bucketMap, ok := dateBucket.(map[string]interface{}); ok {
								v,ok:=bucketMap["key"].(float64)
								if !ok{
									panic("invalid bucket key")
								}
								dateTime:=(int64(v))
								minDate=util.MinInt64(minDate,dateTime)
								maxDate=util.MaxInt64(maxDate,dateTime)

								for mk1,mv1:=range grpMetricData {
									v1,ok:=bucketMap[mk1]
									if ok{
										v2,ok:=v1.(map[string]interface{})
										if ok{
											v3,ok:=v2["value"].(float64)
											if ok{
												if strings.HasSuffix(mk1, "_deriv"){
													v3 = v3/float64(bucketSize)
												}
												if field2, ok := bucketMap[mk1 + "_field2"]; ok {
													if idx, ok := grpMetricItemsIndex[mk1]; ok {
														if field2Map, ok := field2.(map[string]interface{}); ok {
															v3 = grpMetricItems[idx].Calc(v3, field2Map["value"].(float64))
														}
													}
												}
												if v3<0{
													continue
												}
												points:=[]interface{}{dateTime,v3}
												mv1[grpKey]=append(mv1[grpKey],points)
											}
										}
									}
								}
							}
						}
					}

				}
			}
		}
	}

	result:=map[string]*common.MetricItem{}

	for _,metricItem:=range grpMetricItems {
		for _,line:=range metricItem.MetricItem.Lines{
			line.TimeRange=common.TimeRange{Min: minDate,Max: maxDate}
			dataKey := metricItem.ID
			if metricItem.IsDerivative {
				dataKey = dataKey +"_deriv"
			}
			line.Data= grpMetricData[dataKey][line.Metric.Label]
		}
		result[metricItem.Key]=metricItem.MetricItem
	}
	return result
}

func (h *APIHandler) getTopNodeName(clusterID string, top int, lastMinutes int) ([]string, error){
	var (
		now = time.Now()
		max = now.UnixNano()/1e6
		min = now.Add(-time.Duration(lastMinutes) * time.Minute).UnixNano()/1e6
	)
	query := util.MapStr{
		"size": 0,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
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
					{
						"term": util.MapStr{
							"metadata.labels.cluster_id": util.MapStr{
								"value": clusterID,
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
		},
		"aggs": util.MapStr{
			"group_by_index": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.transport_address",
					"size":  10000,
				},
				"aggs": util.MapStr{
					"max_qps": util.MapStr{
						"max_bucket": util.MapStr{
							"buckets_path": "dates>search_qps",
						},
					},
					"max_qps_bucket_sort": util.MapStr{
						"bucket_sort": util.MapStr{
							"sort": []util.MapStr{
								{"max_qps": util.MapStr{"order": "desc"}}},
							"size": top,
						},
					},
					"dates": util.MapStr{
						"date_histogram": util.MapStr{
							"field":    "timestamp",
							"interval": "60s",
						},
						"aggs": util.MapStr{
							"search_query_total": util.MapStr{
								"max": util.MapStr{
									"field": "payload.elasticsearch.node_stats.indices.search.query_total",
								},
							},
							"search_qps": util.MapStr{
								"derivative": util.MapStr{
									"buckets_path": "search_query_total",
								},
							},
						},
					},
				},
			},
			"group_by_index1": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.index_name",
					"size":  10000,
				},
				"aggs": util.MapStr{
					"max_qps": util.MapStr{
						"max_bucket": util.MapStr{
							"buckets_path": "dates>index_qps",
						},
					},
					"max_qps_bucket_sort": util.MapStr{
						"bucket_sort": util.MapStr{
							"sort": []util.MapStr{
								{"max_qps": util.MapStr{"order": "desc"}},
							},
							"size": top,
						},
					},
					"dates": util.MapStr{
						"date_histogram": util.MapStr{
							"field":    "timestamp",
							"interval": "60s",
						},
						"aggs": util.MapStr{
							"index_total": util.MapStr{
								"max": util.MapStr{
									"field": "payload.elasticsearch.node_stats.indices.indexing.index_total",
								},
							},
							"index_qps": util.MapStr{
								"derivative": util.MapStr{
									"buckets_path": "index_total",
								},
							},
						},
					},
				},
			},
		},
	}
	response,err:=elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(),util.MustToJSONBytes(query))
	if err!=nil{
		log.Error(err)
		return nil, err
	}
	var maxQpsKVS = map[string] float64{}
	for _, agg := range response.Aggregations {
		for _, bk := range agg.Buckets {
			key := bk["key"].(string)
			if maxQps, ok := bk["max_qps"].(map[string]interface{}); ok {
				val := maxQps["value"].(float64)
				if _, ok = maxQpsKVS[key] ; ok {
					maxQpsKVS[key] = maxQpsKVS[key] + val
				}else{
					maxQpsKVS[key] = val
				}
			}
		}
	}
	var (
		qpsValues TopTermOrder
	)
	for k, v := range maxQpsKVS {
		qpsValues = append(qpsValues, TopTerm{
			Key:   k,
			Value: v,
		})
	}
	sort.Sort(qpsValues)
	var length = top
	if top > len(qpsValues) {
		length = len(qpsValues)
	}
	nodeNames := []string{}
	for i := 0; i <length; i++ {
		nodeNames = append(nodeNames, qpsValues[i].Key)
	}
	return nodeNames, nil
}
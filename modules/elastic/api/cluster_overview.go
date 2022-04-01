package api

import (
	"fmt"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/graph"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
)

func (h *APIHandler) ClusterOverTreeMap(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	clusterID := ps.ByName("id")

	queryLatency := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"indices": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.index_name",
					"size":  1000,
				},
				"aggs": util.MapStr{
					"recent_15m": util.MapStr{
						"auto_date_histogram": util.MapStr{
							"field":            "timestamp",
							"minimum_interval": "minute",
							"buckets":          12,
						},
						"aggs": util.MapStr{
							"max_query_count": util.MapStr{
								"max": util.MapStr{
									"field": "payload.elasticsearch.index_stats.primaries.search.query_total",
								},
							},
							"query_count_deriv": util.MapStr{
								"derivative": util.MapStr{
									"buckets_path": "max_query_count",
								},
							},
							"max_query_time": util.MapStr{
								"max": util.MapStr{
									"field": "payload.elasticsearch.index_stats.primaries.search.query_time_in_millis",
								},
							},
							"query_time_deriv": util.MapStr{
								"derivative": util.MapStr{
									"buckets_path": "max_query_time",
								},
							},
							"query_latency": util.MapStr{
								"bucket_script": util.MapStr{
									"buckets_path": util.MapStr{
										"my_var1": "query_time_deriv",
										"my_var2": "query_count_deriv",
									},
									"script": "params.my_var1 / params.my_var2",
								},
							},
						},
					},
					"max_query_latency": util.MapStr{
						"max_bucket": util.MapStr{
							"buckets_path": "recent_15m>query_latency",
						},
					},
					"sort": util.MapStr{
						"bucket_sort": util.MapStr{
							"sort": []util.MapStr{
								{
									"max_query_latency": util.MapStr{
										"order": "desc",
									},
								},
							},
						},
					},
				},
			},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"must_not": []util.MapStr{{
					"term": util.MapStr{
						"metadata.labels.index_name": util.MapStr{
							"value": "_all",
						},
					},
				},
				},
				"must": []util.MapStr{
					{
						"match": util.MapStr{
							"metadata.name": "index_stats",
						}},
					util.MapStr{
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
								"gte": "now-7d",
								"lte": "now",
							},
						},
					},
				},
			}},
	}

	q := orm.Query{WildcardIndex: true}
	q.AddQueryArgs("filter_path", "aggregations.indices.buckets.key,aggregations.indices.buckets.max_query_latency")
	q.RawQuery = util.MustToJSONBytes(queryLatency)
	err, searchR1 := orm.Search(&event.Event{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	searchResponse := SearchResponse{}
	err = util.FromJSONBytes(searchR1.Raw, &searchResponse)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	root := graph.NestedNode{Name: "root"}

	indices, ok := searchResponse.Aggregations["indices"].(map[string]interface{})
	if ok {
		buckets, ok := indices["buckets"]
		if ok {
			items, ok := buckets.([]interface{})
			if ok {
				for _, item := range items {
					itemMap, ok := item.(map[string]interface{})
					if ok {
						indexName := itemMap["key"]
						latencyObj, ok := itemMap["max_query_latency"].(map[string]interface{})
						if ok {
							v := latencyObj["value"]
							date := latencyObj["keys"].([]interface{})
							root.Add(indexName.(string), date[0].(string), v.(float64))
						}
					}
				}
			}
		}
	}

	result := util.MapStr{
		"_index": ".infini-graph",
		"_type":  "_doc",
		"_id":    "graph-1",
		"_source": util.MapStr{
			"name": "Avg search latency by index",
			"unit": "ms",
			"data": root,
		},
	}

	h.Write(w, util.MustToJSONBytes(result))
}

func (h *APIHandler) FetchClusterInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	//var (
	//	name        = h.GetParameterOrDefault(req, "keyword", "")
	//	queryDSL    = `{"query":{"bool":{"should":[%s]}}, "size": %d, "from": %d}`
	//	size        = h.GetIntOrDefault(req, "size", 20)
	//	from        = h.GetIntOrDefault(req, "from", 0)
	//	mustBuilder = &strings.Builder{}
	//)
	//
	//if name != "" {
	//	mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.text": "%s"}}`, name))
	//	mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s"}}`, name))
	//	mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s*"}}`, name))
	//}
	//
	//if size <= 0 {
	//	size = 20
	//}
	//
	//if from < 0 {
	//	from = 0
	//}
	//
	//q := orm.Query{}
	//queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	//q.RawQuery = []byte(queryDSL)
	//
	//err, res := orm.Search(&elastic.ElasticsearchConfig{}, &q)
	//if err != nil {
	//	h.WriteError(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}
	//
	//response := elastic.SearchResponse{}
	//util.FromJSONBytes(res.Raw, &response)

	//var clusterIDs []interface{}
	//
	//for _, hit := range response.Hits.Hits {
	//	clusterIDs = append(clusterIDs, hit.ID)
	//}
	//
	//if len(clusterIDs) == 0 {
	//	h.WriteJSON(w, util.MapStr{
	//		"hits": util.MapStr{
	//			"total": util.MapStr{
	//				"value":    0,
	//				"relation": "eq",
	//			},
	//			"hits": []interface{}{},
	//		},
	//	}, 200)
	//	return
	//}
	var clusterIDs = []string{}
	h.DecodeJSON(req, &clusterIDs)

	if len(clusterIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}

	cids := make([]interface{}, 0, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		cids = append(cids, clusterID)
	}
	healthMap := map[string]interface{}{}


	//fetch extra cluster status
	q1 := orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "cluster_stats"),
		orm.In("metadata.labels.cluster_id", cids),
	)
	q1.Collapse("metadata.labels.cluster_id")
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = len(clusterIDs) + 1

	err, results := orm.Search(&event.Event{}, &q1)
	for _, v := range results.Result {
		result, ok := v.(map[string]interface{})
		if ok {
			clusterID, ok := util.GetMapValueByKeys([]string{"metadata", "labels", "cluster_id"}, result)
			if ok {
				source := map[string]interface{}{}
				indicesCount, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "indices", "count"}, result)
				if ok {
					source["number_of_indices"] = indicesCount
				}

				docsCount, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "indices", "docs", "count"}, result)
				if ok {
					source["number_of_documents"] = docsCount
				}

				docsDeletedCount, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "indices", "docs", "deleted"}, result)
				if ok {
					source["number_of_deleted_documents"] = docsDeletedCount
				}
				fs, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "nodes", "fs"}, result)
				if ok {
					source["fs"] = fs
				}
				jvm, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "nodes", "jvm", "mem"}, result)
				if ok {
					source["jvm"] = jvm
				}
				nodeCount, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "nodes", "count", "total"}, result)
				if ok {
					source["number_of_nodes"] = nodeCount
				}
				shardCount, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_stats", "indices", "shards", "total"}, result)
				if ok {
					source["number_of_shards"] = shardCount
				}

				healthMap[util.ToString(clusterID)] = source
			}
		}
	}

	//fetch cluster metrics
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req, 60, (15))
	if err != nil {
		panic(err)
		return
	}

	query := util.MapStr{}
	query["query"] = util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"terms": util.MapStr{
						"metadata.labels.cluster_id": clusterIDs,
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

	var top = len(clusterIDs) + 1

	metricItems := []*common.MetricItem{}
	var bucketSizeStr = fmt.Sprintf("%vs", bucketSize)
	metricItem := newMetricItem("cluster_summary", 2, "cluster")
	indexLine := metricItem.AddLine("Indexing", "Total Indexing", "Number of documents being indexed for primary and replica shards.", "group1",
		"payload.elasticsearch.index_stats.total.indexing.index_total", "max", bucketSizeStr, "doc/s", "num", "0,0.[00]", "0,0.[00]", false, true)

	searchLine := metricItem.AddLine("Search", "Total Search", "Number of search requests being executed across primary and replica shards. A single search can run against multiple shards!", "group1",
		"payload.elasticsearch.index_stats.total.search.query_total", "max", bucketSizeStr, "query/s", "num", "0,0.[00]", "0,0.[00]", false, true)
	metricItems = append(metricItems, metricItem)

	bucketItem := common.NewBucketItem(
		common.TermsBucket, util.MapStr{
			"field": "metadata.labels.cluster_id",
			"size":  top,
		})

	histgram := common.NewBucketItem(
		common.DateHistogramBucket, util.MapStr{
			"field":          "timestamp",
			"fixed_interval": bucketSizeStr,
		})
	histgram.AddMetricItems(metricItems...)

	bucketItem.AddNestBucket(histgram)

	query["size"] = 0

	aggs := ConvertBucketItemsToAggQuery([]*common.BucketItem{bucketItem}, nil)

	util.MergeFields(query, aggs, true)

	searchR1, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		panic(err)
	}

	searchResponse := SearchResponse{}
	util.FromJSONBytes(searchR1.RawResult.Body, &searchResponse)

	m1 := ParseAggregationResult(bucketSize, searchResponse.Aggregations, bucketItem.Key, histgram.Key, indexLine.Metric.GetDataKey())
	m2 := ParseAggregationResult(bucketSize, searchResponse.Aggregations, bucketItem.Key, histgram.Key, searchLine.Metric.GetDataKey())

	//fetch recent cluster health status
	bucketItem = common.NewBucketItem(
		common.TermsBucket, util.MapStr{
			"field": "metadata.labels.cluster_id",
			"size":  top,
		})

	bucketSizeStr = "1d"
	histgram = common.NewBucketItem(
		common.DateRangeBucket, util.MapStr{
			"field":     "timestamp",
			"format":    "yyyy-MM-dd",
			"time_zone": "+08:00",
			"ranges": []util.MapStr{
				{
					"from": "now-13d/d",
					"to": "now-12d/d",
				}, {
					"from": "now-12d/d",
					"to": "now-11d/d",
				},
				{
					"from": "now-11d/d",
					"to": "now-10d/d",
				},
				{
					"from": "now-10d/d",
					"to": "now-9d/d",
				}, {
					"from": "now-9d/d",
					"to": "now-8d/d",
				},
				{
					"from": "now-8d/d",
					"to": "now-7d/d",
				},
				{
					"from": "now-7d/d",
					"to": "now-6d/d",
				},
				{
					"from": "now-6d/d",
					"to": "now-5d/d",
				}, {
					"from": "now-5d/d",
					"to": "now-4d/d",
				},
				{
					"from": "now-4d/d",
					"to": "now-3d/d",
				},{
					"from": "now-3d/d",
					"to": "now-2d/d",
				}, {
					"from": "now-2d/d",
					"to": "now-1d/d",
				}, {
					"from": "now-1d/d",
					"to": "now/d",
				},
				{
					"from": "now/d",
					"to": "now",
				},
			},
		})

	termBucket := common.NewBucketItem(common.TermsBucket, util.MapStr{
		"field":   "payload.elasticsearch.cluster_health.status",
		"size":    top,
		"missing": "",
	})
	histgram.AddNestBucket(termBucket)

	bucketItem.AddNestBucket(histgram)

	aggs = ConvertBucketItemsToAggQuery([]*common.BucketItem{bucketItem}, nil)
	query = util.MapStr{}
	query["query"] = util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"terms": util.MapStr{
						"metadata.labels.cluster_id": clusterIDs,
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
							"value": "cluster_health",
						},
					},
				},
			},
			"filter": []util.MapStr{
				{
					"range": util.MapStr{
						"timestamp": util.MapStr{
							"gte": "now-15d",
							"lte": "now",
						},
					},
				},
			},
		},
	}
	query["size"] = 0

	util.MergeFields(query, aggs, true)

	searchR1, err = elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		panic(err)
	}
	searchResponse = SearchResponse{}
	util.FromJSONBytes(searchR1.RawResult.Body, &searchResponse)
	m3 := ParseAggregationBucketResult(bucketSize, searchResponse.Aggregations, bucketItem.Key, histgram.Key, termBucket.Key, nil)

	infos := util.MapStr{}
	for _, clusterID := range clusterIDs {
		result := util.MapStr{}

		//TODO update last active timestamp
		//source := hit.Source
		//source["project"]=util.MapStr{
		//	"id":"12312312",
		//	"name":"统一日志云平台v1.0",
		//}
		//source["location"]=util.MapStr{
		//	"provider" : "阿里云",
		//	"region" : "北京",
		//	"dc" : "昌平机房",
		//	"rack" : "rack-1",
		//}
		//source["owner"]=[]util.MapStr{util.MapStr{
		//	"department" : "运维部",
		//	"name":"廖石阳",
		//	"id":"123123123",
		//}}

		//result["metadata"] = source
		result["summary"] = healthMap[clusterID]
		result["metrics"] = util.MapStr{
			"status": util.MapStr{
				"metric": util.MapStr{
					"label": "Recent Cluster Status",
					"units": "day",
				},
				"data": getClusterMetrics(clusterID, m3),
			},
			"indexing": util.MapStr{
				"metric": util.MapStr{
					"label": "Indexing",
					"units": "s",
				},
				"data": getClusterMetrics(clusterID, m1),
			},
			"search": util.MapStr{
				"metric": util.MapStr{
					"label": "Search",
					"units": "s",
				},
				"data": getClusterMetrics(clusterID, m2),
			},
		}
		infos[clusterID]= result
	}

	h.WriteJSON(w, infos , 200)
}

func getClusterMetrics(id string, data MetricData) [][]interface{} {
	return data[id]
}

func (h *APIHandler) GetClusterInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	q := &orm.Query{WildcardIndex: true, Size: 1}
	q.AddSort("timestamp", orm.DESC)
	q.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "cluster_health"),
		orm.Eq("metadata.labels.cluster_id", id),
		)

	err, result := orm.Search(event.Event{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	var healthInfo interface{} = util.MapStr{}
	if len(result.Result) > 0 {
		if rowM, ok := result.Result[0].(map[string]interface{}); ok {
			healthInfo, _ = util.MapStr(rowM).GetValue("payload.elasticsearch.cluster_health")
		}
	}

	h.WriteJSON(w, healthInfo,200)
}

func (h *APIHandler) GetClusterNodes(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		size        = h.GetIntOrDefault(req, "size", 20)
		from        = h.GetIntOrDefault(req, "from", 0)
	)
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	q := &orm.Query{ Size: size, From: from}
	q.AddSort("timestamp", orm.DESC)
	q.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.cluster_id", id),
	)

	err, result := orm.Search(elastic.NodeConfig{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	h.Write(w, result.Raw)
}

func (h *APIHandler) GetRealtimeClusterNodes(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	esClient := elastic.GetClient(id)
	if esClient == nil {
		h.WriteJSON(w, util.MapStr{
			"error": "cluster not found",
		}, http.StatusNotFound)
		return
	}
	catNodesInfo, err := esClient.CatNodes("id,name,ip,port,master,heap.percent,disk.avail,cpu,load_1m,uptime")
	if err != nil {
		h.WriteJSON(w, util.MapStr{
			"error": err.Error(),
		}, http.StatusInternalServerError)
		return
	}
	catShardsInfo, err := esClient.CatShards()
	if err != nil {
		h.WriteJSON(w, util.MapStr{
			"error": err.Error(),
		}, http.StatusInternalServerError)
		return
	}
	shardCounts := map[string]int{}
	for _, shardInfo := range catShardsInfo {
		if c, ok := shardCounts[shardInfo.NodeName]; ok {
			shardCounts[shardInfo.NodeName] = c +1
		}else{
			shardCounts[shardInfo.NodeName] = 1
		}
	}

	for i, nodeInfo := range catNodesInfo {
		if c, ok := shardCounts[nodeInfo.Name]; ok {
			catNodesInfo[i].Shards = c
		}
	}
	h.WriteJSON(w, catNodesInfo, http.StatusOK)
}

func (h *APIHandler) GetClusterIndices(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	//var (
	//	size        = h.GetIntOrDefault(req, "size", 20)
	//	from        = h.GetIntOrDefault(req, "from", 0)
	//)
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	q := &orm.Query{ Size: 2000}
	q.AddSort("timestamp", orm.DESC)
	q.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.cluster_id", id),
		orm.NotEq("metadata.labels.state", "delete"),
	)

	err, result := orm.Search(elastic.IndexConfig{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	query := util.MapStr{
		"size": 2000,
		"_source": []string{"metadata","payload.elasticsearch.index_stats.index_info", "timestamp"},
		"collapse": util.MapStr{
			"field": "metadata.labels.index_name",
		},
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"filter": []util.MapStr{
					{
						"range": util.MapStr{
							"timestamp": util.MapStr{
								"gte": "now-15m",
							},
						},
					},
				},
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
							"metadata.labels.cluster_id": util.MapStr{
								"value": id,
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
				},
			},
		},
	}
	q = &orm.Query{ RawQuery: util.MustToJSONBytes(query),WildcardIndex: true}
	err, searchResult := orm.Search(event.Event{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	indexInfos := map[string]util.MapStr{}
	for _, hit := range searchResult.Result {
		if hitM, ok := hit.(map[string]interface{}); ok {
			indexInfo, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "index_info"}, hitM)
			indexName, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "index_name"}, hitM)
			if v, ok := indexName.(string); ok {
				if infoM, ok := indexInfo.(map[string]interface{}); ok {
					infoM["timestamp"] = hitM["timestamp"]
					indexInfos[v] = infoM
				}
			}
		}
	}
	indices := []interface{}{}
	for _, hit := range result.Result {
		if hitM, ok := hit.(map[string]interface{}); ok {
			indexName, _ := util.GetMapValueByKeys([]string{"metadata", "index_name"}, hitM)
			if v, ok := indexName.(string); ok {
				if indexInfos[v] != nil {
					indices = append(indices, indexInfos[v])
				}else{
					indices = append(indices, util.MapStr{
						"index": v,
					})
				}
			}
		}
	}


	h.WriteJSON(w, indices, http.StatusOK)
}

func (h *APIHandler) GetRealtimeClusterIndices(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	esClient := elastic.GetClient(id)
	indexInfos, err := esClient.GetIndices("")
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}

	var indices []elastic.IndexInfo
	for _, item := range *indexInfos {
		indices = append(indices, item)
	}
	h.WriteJSON(w, indices, http.StatusOK)
}

func (h *APIHandler) SearchClusterMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody:=util.MapStr{}
	reqBody := struct{
		Keyword string `json:"keyword"`
		Size int `json:"size"`
		From int `json:"from"`
		Aggregations []elastic.SearchAggParam `json:"aggs"`
		Highlight elastic.SearchHighlightParam `json:"highlight"`
		Filter elastic.SearchFilterParam `json:"filter"`
		Sort []string `json:"sort"`
		SearchField string `json:"search_field"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	var should []util.MapStr
	if reqBody.SearchField != "" {
		should =  []util.MapStr{
			{
				"prefix": util.MapStr{
					reqBody.SearchField: util.MapStr{
						"value": reqBody.Keyword,
						"boost": 20,
					},
				},
			},
			{
				"match": util.MapStr{
					reqBody.SearchField: util.MapStr{
						"query":                reqBody.Keyword,
						"fuzziness":            "AUTO",
						"max_expansions":       10,
						"prefix_length":        2,
						"fuzzy_transpositions": true,
						"boost":                2,
					},
				},
			},
		}
	}else {
		should = []util.MapStr{
			{
				"prefix": util.MapStr{
					"name": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 20,
					},
				},
			},
			{
				"prefix": util.MapStr{
					"host": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 20,
					},
				},
			},
			{
				"prefix": util.MapStr{
					"version": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 15,
					},
				},
			},
			{
				"match_phrase_prefix": util.MapStr{
					"name.text": util.MapStr{
						"query": reqBody.Keyword,
						"boost": 6,
					},
				},
			},
			{
				"match": util.MapStr{
					"search_text": util.MapStr{
						"query":                reqBody.Keyword,
						"fuzziness":            "AUTO",
						"max_expansions":       10,
						"prefix_length":        2,
						"fuzzy_transpositions": true,
						"boost":                2,
					},
				},
			},
			{
				"query_string": util.MapStr{
					"fields":                 []string{"*"},
					"query":                  reqBody.Keyword,
					"fuzziness":              "AUTO",
					"fuzzy_prefix_length":    2,
					"fuzzy_max_expansions":   10,
					"fuzzy_transpositions":   true,
					"allow_leading_wildcard": false,
				},
			},
		}
	}
	query := util.MapStr{
		"aggs":      elastic.BuildSearchTermAggregations(reqBody.Aggregations),
		"size":      reqBody.Size,
		"from": reqBody.From,
		"highlight": elastic.BuildSearchHighlight(&reqBody.Highlight),
		"query": util.MapStr{
			"bool": util.MapStr{
				"filter": elastic.BuildSearchTermFilter(reqBody.Filter),
				"should": should,
			},
		},
	}
	if len(reqBody.Sort) > 1 {
		query["sort"] =  []util.MapStr{
			{
				reqBody.Sort[0]: util.MapStr{
					"order": reqBody.Sort[1],
				},
			},
		}
	}
	dsl := util.MustToJSONBytes(query)
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(orm.GetIndexName(elastic.ElasticsearchConfig{}), dsl)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	w.Write(util.MustToJSONBytes(response))
}
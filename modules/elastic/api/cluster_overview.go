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
	"strings"
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

func (h *APIHandler) SearchClusterMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		name        = h.GetParameterOrDefault(req, "keyword", "")
		queryDSL    = `{"query":{"bool":{"should":[%s]}}, "size": %d, "from": %d}`
		size        = h.GetIntOrDefault(req, "size", 20)
		from        = h.GetIntOrDefault(req, "from", 0)
		mustBuilder = &strings.Builder{}
	)

	if name != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.text": "%s"}}`, name))
		mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s"}}`, name))
		mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s*"}}`, name))
	}

	if size <= 0 {
		size = 20
	}

	if from < 0 {
		from = 0
	}

	q := orm.Query{}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	q.RawQuery = []byte(queryDSL)

	err, res := orm.Search(&elastic.ElasticsearchConfig{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)

	var clusterIDs []interface{}

	for _, hit := range response.Hits.Hits {
		clusterIDs = append(clusterIDs, hit.ID)
	}

	if len(clusterIDs) == 0 {
		h.WriteJSON(w, util.MapStr{
			"hits": util.MapStr{
				"total": util.MapStr{
					"value":    0,
					"relation": "eq",
				},
				"hits": []interface{}{},
			},
		}, 200)
		return
	}

	//fetch cluster status
	q1 := orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "cluster_health"),
		orm.In("metadata.labels.cluster_id", clusterIDs),
	)
	q1.Collapse("metadata.labels.cluster_id")
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = len(clusterIDs) + 1

	err, results := orm.Search(&event.Event{}, &q1)

	healthMap := map[string]interface{}{}
	for _, v := range results.Result {
		result, ok := v.(map[string]interface{})
		clusterID, ok := util.GetMapValueByKeys([]string{"metadata", "labels", "cluster_id"}, result)
		if ok {
			health, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_health"}, result)
			if ok {
				cid := util.ToString(clusterID)
				source := health.(map[string]interface{})
				meta := elastic.GetMetadata(cid)
				if meta != nil && !meta.IsAvailable() {
					source["status"] = "unavailable"
				}
				healthMap[cid] = source
			}
		}
	}

	//fetch extra cluster status
	q1 = orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "cluster_stats"),
		orm.In("metadata.labels.cluster_id", clusterIDs),
	)
	q1.Collapse("metadata.labels.cluster_id")
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = len(clusterIDs) + 1

	err, results = orm.Search(&event.Event{}, &q1)
	for _, v := range results.Result {
		result, ok := v.(map[string]interface{})
		if ok {
			clusterID, ok := util.GetMapValueByKeys([]string{"metadata", "labels", "cluster_id"}, result)
			if ok {
				source := healthMap[util.ToString(clusterID)].(map[string]interface{})
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
				util.MapStr{
					"to": "now-13d/d",
				}, util.MapStr{
					"to": "now-12d/d",
				},
				util.MapStr{
					"to": "now-11d/d",
				},
				util.MapStr{
					"to": "now-10d/d",
				}, util.MapStr{
					"to": "now-9d/d",
				},
				util.MapStr{
					"to": "now-8d/d",
				},
				util.MapStr{
					"to": "now-7d/d",
				},
				util.MapStr{
					"to": "now-6d/d",
				}, util.MapStr{
					"to": "now-5d/d",
				},
				util.MapStr{
					"to": "now-4d/d",
				}, util.MapStr{
					"to": "now-3d/d",
				}, util.MapStr{
					"to": "now-2d/d",
				}, util.MapStr{
					"to": "now-1d/d",
				},
				util.MapStr{
					"from": "now/d",
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

	for i, hit := range response.Hits.Hits {
		result := util.MapStr{}

		//TODO update last active timestamp
		source := hit.Source
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

		result["metadata"] = source
		result["summary"] = healthMap[hit.ID]
		result["metrics"] = util.MapStr{
			"status": util.MapStr{
				"metric": util.MapStr{
					"label": "Recent Cluster Status",
					"units": "day",
				},
				"data": getClusterMetrics(hit.ID, m3),
			},
			"indexing": util.MapStr{
				"metric": util.MapStr{
					"label": "Indexing",
					"units": "s",
				},
				"data": getClusterMetrics(hit.ID, m1),
			},
			"search": util.MapStr{
				"metric": util.MapStr{
					"label": "Search",
					"units": "s",
				},
				"data": getClusterMetrics(hit.ID, m2),
			},
		}
		response.Hits.Hits[i].Source = result
		clusterIDs = append(clusterIDs, hit.ID)
	}

	h.WriteJSON(w, response, 200)
}

func getClusterMetrics(id string, data MetricData) [][]interface{} {
	return data[id]
}

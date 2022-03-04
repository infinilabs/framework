/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"fmt"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	log "src/github.com/cihub/seelog"
)

func (h *APIHandler) SearchIndexMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		keyword        = h.GetParameterOrDefault(req, "keyword", "")
		clusterID = h.GetParameterOrDefault(req, "cluster_id", "")
		size        = h.GetIntOrDefault(req, "size", 20)
		from        = h.GetIntOrDefault(req, "from", 0)
		indexID = h.GetParameterOrDefault(req, "index_id", "")
	)

	var should []interface{}
	if keyword != "" {
		should = append(should, util.MapStr{
			"prefix": util.MapStr{
				"metadata.index_name": keyword,
			},
		}, util.MapStr{
			"query_string": util.MapStr{"query": keyword},
		}, util.MapStr{
			"query_string": util.MapStr{"query": keyword+"*"},
		})
	}
	var must  []interface{}
	if clusterID != "" {
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"metadata.cluster_id": util.MapStr{
					"value": clusterID,
				} ,
			},
		})
	}
	if indexID != "" {
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"_id": util.MapStr{
					"value": indexID,
				} ,
			},
		})
	}

	if size <= 0 {
		size = 20
	}

	if from < 0 {
		from = 0
	}

	query := util.MapStr{
		"size": size,
		"from": from,
		"sort": []util.MapStr{
			{"timestamp":util.MapStr{
				"order": "desc",
			}},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"should": should,
				"must": must,
				"must_not": []util.MapStr{
					{"term":util.MapStr{
						"metadata.labels.index_status": util.MapStr{
							"value": "deleted",
						},
					}},
				},
			},
		},
	}

	q := orm.Query{}
	q.RawQuery = util.MustToJSONBytes(query)

	err, res := orm.Search(&elastic.IndexConfig{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)

	var indexIDs []interface{}

	for _, hit := range response.Hits.Hits {
		if indexID, ok := util.GetMapValueByKeys([]string{"metadata", "index_id"}, hit.Source); ok {
			indexIDs = append(indexIDs, indexID)
		}
	}

	if len(indexIDs) == 0 {
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

	q1 := orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "index_stats"),
		orm.In("metadata.labels.index_id", indexIDs),
	)
	q1.Collapse("metadata.labels.index_id")
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = len(indexIDs) + 1

	err, results := orm.Search(&event.Event{}, &q1)

	summaryMap := util.MapStr{}
	for _, v := range results.Result {
		result, ok := v.(map[string]interface{})
		if ok {
			if indexID, ok :=  util.GetMapValueByKeys([]string{"metadata", "labels", "index_id"}, result); ok {
				summary := map[string]interface{}{}
				if docs, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "total", "docs"}, result); ok {
					summary["docs"] = docs
				}
				if indexInfo, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "index_info"}, result); ok {
					summary["index_info"] = indexInfo
				}
				if shardInfo, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "shard_info"}, result); ok {
					if sinfo, ok := shardInfo.([]interface{}); ok {
						unassignedCount := 0
						for _, item := range sinfo {
							if itemMap, ok := item.(map[string]interface{}); ok{
								if itemMap["state"] == "UNASSIGNED" {
									unassignedCount++
								}
							}

						}
						summary["unassigned_shards"] = unassignedCount
					}
				}
				summaryMap[indexID.(string)] = summary
			}
		}
	}

	statusMetric, err := getIndexStatusOfRecentDay(indexIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req, 60, (15))
	if err != nil {
		panic(err)
		return
	}
	// 索引速率
	indexMetric:=newMetricItem("indexing", 1, OperationGroupKey)
	indexMetric.AddAxi("indexing rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems := []GroupMetricItem{}
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "indexing",
		Field: "payload.elasticsearch.index_stats.primaries.indexing.index_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexMetric,
		FormatType: "num",
		Units: "Indexing/s",
	})
	queryMetric:=newMetricItem("search", 2, OperationGroupKey)
	queryMetric.AddAxi("query rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "search",
		Field: "payload.elasticsearch.index_stats.total.search.query_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryMetric,
		FormatType: "num",
		Units: "Search/s",
	})

	aggs:=map[string]interface{}{}
	query =map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must":  []util.MapStr{
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
				{
					"terms": util.MapStr{
						"metadata.labels.index_id": indexIDs,
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

	for _,metricItem:=range nodeMetricItems{
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
				"field": "metadata.labels.index_id",
				"size":  100,
			},
			"aggs": util.MapStr{
				"dates": util.MapStr{
					"date_histogram":util.MapStr{
						"field": "timestamp",
						"fixed_interval": fmt.Sprintf("%ds", bucketSize),
					},
					"aggs":aggs,
				},
			},
		},
	}
	metrics := h.getMetrics(query, nodeMetricItems, bucketSize)
	indexMetrics := map[string]util.MapStr{}
	for key, item := range metrics {
		for _, line := range item.Lines {
			if _, ok := indexMetrics[line.Metric.Label]; !ok{
				indexMetrics[line.Metric.Label] = util.MapStr{
				}
			}
			indexMetrics[line.Metric.Label][key] = line.Data
		}
	}

	for i, hit := range response.Hits.Hits {
		result := util.MapStr{}

		source := hit.Source
		tempIndexID, _ := util.GetMapValueByKeys([]string{"metadata", "index_id"}, source)
		indexID := tempIndexID.(string)
		source["index_id"] =  indexID
		tempIndexName, _ := util.GetMapValueByKeys([]string{"metadata", "index_name"}, source)
		if tempClusterID, ok := util.GetMapValueByKeys([]string{"metadata", "cluster_id"}, source); ok {
			if clusterID, ok :=  tempClusterID.(string); ok {
				if data :=  elastic.GetMetadata(clusterID); data != nil {
					source["cluster_name"] = data.Config.Name
					source["cluster_id"] = clusterID
				}
			}
		}
		indexName := tempIndexName.(string)
		delete(source, "metadata")
		delete(source, "payload")
		source["index_name"] = indexName
		labels, _ := util.GetMapValueByKeys([]string{"metadata", "labels"}, source)
		if mp, ok := labels.(map[string]interface{}); ok {
			source["aliases"] = mp["aliases"]
		}

		result["metadata"] = source
		result["summary"] = summaryMap[indexID]
		result["metrics"] = util.MapStr{
			"status": util.MapStr{
				"metric": util.MapStr{
					"label": "Recent Index Status",
					"units": "day",
				},
				"data": statusMetric[indexID],
			},
			"indexing": util.MapStr{
				"metric": util.MapStr{
					"label": "Indexing",
					"units": "s",
				},
				"data": indexMetrics[indexID]["indexing"],
			},
			"search": util.MapStr{
				"metric": util.MapStr{
					"label": "Search",
					"units": "s",
				},
				"data": indexMetrics[indexID]["search"],
			},
		}
		response.Hits.Hits[i].Source = result
	}
	h.WriteJSON(w, response, http.StatusOK)

}

func (h *APIHandler) GetIndexInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.MustGetParameter("id")
	indexID := ps.MustGetParameter("index")

	q := orm.Query{
		Size: 1,
	}
	q.Conds = orm.And(orm.Eq("metadata.index_id", indexID))
	q.AddSort("timestamp", orm.DESC)

	err, res := orm.Search(&elastic.IndexConfig{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)
	if len(response.Hits.Hits) == 0 {
		h.WriteError(w, "", http.StatusNotFound)
		return
	}
	q1 := orm.Query{
		Size: 1,
		WildcardIndex: true,
	}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "index_stats"),
		orm.Eq("metadata.labels.index_id", indexID),
		orm.Eq("metadata.labels.cluster_id", clusterID),
	)
	q1.Collapse("metadata.labels.node_id")
	q1.AddSort("timestamp", orm.DESC)
	err, result := orm.Search(&event.Event{}, &q1)
	summary := util.MapStr{}
	hit := response.Hits.Hits[0].Source
	if aliases, ok := util.GetMapValueByKeys([]string{"metadata","labels", "aliases"}, hit); ok {
		summary["aliases"] = aliases
	}
	//if mappings, ok := util.GetMapValueByKeys([]string{"metadata", "mappings"}, hit); ok {
	//	summary["mappings"] = mappings
	//}
	//if settings, ok := util.GetMapValueByKeys([]string{"metadata", "settings"}, hit); ok {
	//	summary["settings"] = settings
	//}
	if len(result.Result) > 0 {
		result, ok := result.Result[0].(map[string]interface{})
		if ok {
			if docs, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "total", "docs"}, result); ok {
				summary["docs"] = docs
			}
			if indexInfo, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "index_info"}, result); ok {
				summary["index_info"] = indexInfo
			}
			if shardInfo, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_stats", "shard_info"}, result); ok {
				if sinfo, ok := shardInfo.([]interface{}); ok {
					unassignedCount := 0
					for _, item := range sinfo {
						if itemMap, ok := item.(map[string]interface{}); ok{
							if itemMap["state"] == "UNASSIGNED" {
								unassignedCount++
							}
						}

					}
					summary["unassigned_shards"] = unassignedCount
				}
			}
		}
	}

	h.WriteJSON(w, summary, http.StatusOK)
}

func (h *APIHandler) GetSingleIndexMetrics(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.MustGetParameter("id")
	indexID := ps.MustGetParameter("index")
	indexConfig := &elastic.IndexConfig{
		ID: indexID,
	}
	exists, err := orm.Get(indexConfig)
	if !exists {
		h.WriteError(w, fmt.Sprintf("index id %s not found", indexID), http.StatusNotFound)
		return
	}
	var must = []util.MapStr{
		{
			"term": util.MapStr{
				"metadata.labels.cluster_id": util.MapStr{
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
		{
			"term": util.MapStr{
				"metadata.labels.index_name": util.MapStr{
					"value": indexConfig.Metadata.IndexName,
				},
			},
		},
	}
	resBody := map[string]interface{}{}
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req, 10, 60)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	query := map[string]interface{}{}
	query["query"] = util.MapStr{
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

	bucketSizeStr := fmt.Sprintf("%vs", bucketSize)
	metricItems := []*common.MetricItem{}
	metricItem:=newMetricItem("index_throughput", 1, OperationGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	metricItem.AddLine("Indexing Rate","Indexing Rate","Number of documents being indexed for node.","group1","payload.elasticsearch.index_stats.primaries.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Deleting Rate","Deleting Rate","Number of documents being deleted for node.","group1","payload.elasticsearch.index_stats.primaries.indexing.delete_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)
	metricItem=newMetricItem("search_throughput", 2, OperationGroupKey)
	metricItem.AddAxi("searching","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Search Rate","Search Rate",
		"Number of search requests being executed.",
		"group1","payload.elasticsearch.index_stats.total.search.query_total","max",bucketSizeStr,"query/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)

	metricItem=newMetricItem("index_latency", 3, LatencyGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)

	metricItem.AddLine("Indexing","Indexing Latency","Average latency for indexing documents.","group1","payload.elasticsearch.index_stats.primaries.indexing.index_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[0].Metric.Field2 = "payload.elasticsearch.index_stats.primaries.indexing.index_total"
	metricItem.Lines[0].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Indexing","Delete Latency","Average latency for delete documents.","group1","payload.elasticsearch.index_stats.primaries.indexing.delete_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[1].Metric.Field2 = "payload.elasticsearch.index_stats.primaries.indexing.delete_total"
	metricItem.Lines[1].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItems=append(metricItems,metricItem)

	metricItem=newMetricItem("search_latency", 4, LatencyGroupKey)
	metricItem.AddAxi("searching","group2",common.PositionLeft,"num","0,0","0,0.[00]",5,false)

	metricItem.AddLine("Searching","Query Latency","Average latency for searching query.","group2","payload.elasticsearch.index_stats.total.search.query_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[0].Metric.Field2 = "payload.elasticsearch.index_stats.total.search.query_total"
	metricItem.Lines[0].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Searching","Fetch Latency","Average latency for searching fetch.","group2","payload.elasticsearch.index_stats.total.search.fetch_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[1].Metric.Field2 = "payload.elasticsearch.index_stats.total.search.fetch_total"
	metricItem.Lines[1].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Searching","Scroll Latency","Average latency for searching fetch.","group2","payload.elasticsearch.index_stats.total.search.scroll_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[2].Metric.Field2 = "payload.elasticsearch.index_stats.total.search.scroll_total"
	metricItem.Lines[2].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItems=append(metricItems,metricItem)
	resBody["metrics"] = h.getSingleMetrics(metricItems,query, bucketSize)
	h.WriteJSON(w, resBody, http.StatusOK)
}


func getIndexStatusOfRecentDay(indexIDs []interface{})(map[string][]interface{}, error){
	q := orm.Query{
		WildcardIndex: true,
	}
	query := util.MapStr{
		"aggs": util.MapStr{
			"group_by_index_id": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.index_id",
					"size": 100,
				},
				"aggs": util.MapStr{
					"time_histogram": util.MapStr{
						"date_range": util.MapStr{
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
						},
						"aggs": util.MapStr{
							"term_health": util.MapStr{
								"terms": util.MapStr{
									"field": "payload.elasticsearch.index_stats.index_info.health",
								},
							},
						},
					},
				},
			},
		},
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
		"size": 0,
		"query": util.MapStr{
			"bool": util.MapStr{
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
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.name": util.MapStr{
								"value": "index_stats",
							},
						},
					},
					{
						"terms": util.MapStr{
							"metadata.labels.index_id": indexIDs,
						},
					},
				},
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(query)

	err, res := orm.Search(&event.Event{}, &q)
	if err != nil {
		return nil, err
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)
	recentStatus := map[string][]interface{}{}
	for _, bk := range response.Aggregations["group_by_index_id"].Buckets {
		indexKey := bk["key"].(string)
		recentStatus[indexKey] = []interface{}{}
		if histogramAgg, ok := bk["time_histogram"].(map[string]interface{}); ok {
			if bks, ok := histogramAgg["buckets"].([]interface{}); ok {
				for _, bkItem := range  bks {
					if bkVal, ok := bkItem.(map[string]interface{}); ok {
						if termHealth, ok := bkVal["term_health"].(map[string]interface{}); ok {
							if healthBks, ok := termHealth["buckets"].([]interface{}); ok {
								if len(healthBks) == 0 {
									continue
								}
								healthMap := map[string]int{}
								status := "unknown"
								for _, hbkItem := range  healthBks {
									if hitem, ok := hbkItem.(map[string]interface{}); ok {
										healthMap[hitem["key"].(string)] = 1
									}
								}
								if _, ok = healthMap["red"]; ok {
									status = "red"
								}else if _, ok = healthMap["yellow"]; ok {
									status = "yellow"
								}else if _, ok = healthMap["green"]; ok {
									status = "green"
								}
								recentStatus[indexKey] = append(recentStatus[indexKey], []interface{}{bkVal["key"], status})
							}
						}
					}
				}
			}
		}
	}
	return recentStatus, nil
}

func (h *APIHandler) getIndexNodes(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	indexName := ps.ByName("index")
	q := &orm.Query{ Size: 1}
	q.AddSort("timestamp", orm.DESC)
	q.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.labels.cluster_id", id),
		orm.Eq("metadata.labels.index_name", indexName),
		orm.Eq("metadata.name", "index_routing_table"),
	)

	err, result := orm.Search(event.Event{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	namesM := util.MapStr{}
	if len(result.Result) > 0 {
		if data, ok := result.Result[0].(map[string]interface{}); ok {
			if routingTable, exists := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_routing_table"}, data); exists {
				if table, ok := routingTable.(map[string]interface{}); ok{
					if shardsM, ok := table["shards"].(map[string]interface{}); ok {
						for _, rows := range shardsM {
							if rowsArr, ok := rows.([]interface{}); ok {
								for _, rowsInner := range rowsArr {
									if rowsInnerM, ok := rowsInner.(map[string]interface{}); ok {
										if v, ok := rowsInnerM["node"].(string); ok {
											namesM[v] = true
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

	//node uuid
	nodeIds := make([]interface{}, 0, len(namesM) )
	for name, _ := range namesM {
		nodeIds = append(nodeIds, name)
	}

	q1 := &orm.Query{ Size: 100}
	q1.AddSort("timestamp", orm.DESC)
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.cluster_id", id),
		orm.In("metadata.node_id", nodeIds),
	)
	err, result = orm.Search(elastic.NodeConfig{}, q1)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}

	h.Write(w, result.Raw)
}
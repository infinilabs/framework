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
	"strings"
)

func (h *APIHandler) SearchIndexMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		keyword        = h.GetParameterOrDefault(req, "keyword", "")
		queryDSL    = `{"query":{"bool":{"should":[%s]}}, "size": %d, "from": %d, "sort": [
    {
      "timestamp": {
        "order": "desc"
      }
    }
  ], "collapse": {"field": "index_id"}}`
		size        = h.GetIntOrDefault(req, "size", 20)
		from        = h.GetIntOrDefault(req, "from", 0)
		mustBuilder = &strings.Builder{}
	)

	if keyword != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"index_name": "%s"}}`, keyword))
		mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s"}}`, keyword))
		mustBuilder.WriteString(fmt.Sprintf(`,{"query_string":{"query": "%s*"}}`, keyword))
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

	err, res := orm.Search(&elastic.IndexMetadata{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)

	var indexIDs []interface{}

	for _, hit := range response.Hits.Hits {
		indexIDs = append(indexIDs, hit.Source["index_id"])
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
					if sinfo, ok := shardInfo.(map[string]interface{}); ok {
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

	statusMetric, err := getIndexStatusOfRecentDay(indexIDs, 14, "1d")
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
	query:=map[string]interface{}{}
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
		indexID := source["index_id"].(string)
		innerMetaData := source["metadata"]
		delete(source, "metadata")
		if mp, ok := innerMetaData.(map[string]interface{}); ok {
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


func getIndexStatusOfRecentDay(indexIDs []interface{}, days int, interval string)(map[string][]interface{}, error){
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
						"date_histogram": util.MapStr{
							"field": "timestamp",
							"interval": interval,
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
								"gte": fmt.Sprintf("now-%dd", days),
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
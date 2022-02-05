package api

import (
	"fmt"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	"strings"
)

func (h *APIHandler) ClusterOverTreeMap(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	str := "{ \"_index\": \".infini-search-center_logs\", \"_type\": \"_doc\", \"_id\": \"c0oc4kkgq9s8qss2uk50\", \"_source\": { \"name\": \"root\", \"children\": [ { \"brand\": \"172.19.3.151\", \"name\": \"172.19.3.151\", \"value\": 50449102, \"children\": [ { \"name\": \"sales_29_v5\", \"value\": 87891, \"children\": [ { \"name\": \"0\", \"value\": 87891 } ] }, { \"name\": \"orders_219_v4\", \"value\": 782368, \"children\": [ { \"name\": \"1\", \"value\": 782368 } ] }, { \"name\": \"orders_58_v4\", \"value\": 994462, \"children\": [ { \"name\": \"0\", \"value\": 994462 } ] } ] }, { \"brand\": \"172.19.3.95\", \"name\": \"172.19.3.95\", \"value\": 18800601, \"children\": [ { \"name\": \"sales_74_v5\", \"value\": 13585101, \"children\": [ { \"name\": \"1\", \"value\": 13585101 } ] }, { \"name\": \"sales_108_v5\", \"value\": 3859386, \"children\": [ { \"name\": \"1\", \"value\": 3859386 } ] } ] } ] } }"
	h.Write(w, []byte(str))
}

func (h *APIHandler) SearchClusterMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		name        = h.GetParameterOrDefault(req, "name", "")
		queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		size        = h.GetIntOrDefault(req, "size", 20)
		from        = h.GetIntOrDefault(req, "from", 0)
		mustBuilder = &strings.Builder{}
	)

	if name != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.text": "%s"}}`, name))
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
		h.WriteError(w, "no cluster found", 404)
		return
	}

	//fetch cluster status
	q1 := orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "cluster_health"),
		orm.In("metadata.labels.cluster_id", clusterIDs),
	)
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = len(clusterIDs)+1

	err, results := orm.Search(&event.Event{}, &q1)

	healthMap := map[string]interface{}{}
	for _, v := range results.Result {
		result, ok := v.(map[string]interface{})
		clusterID, ok := util.GetMapValueByKeys([]string{"metadata", "labels", "cluster_id"}, result)
		if ok {
			health, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "cluster_health"}, result)
			if ok {
				source := health.(map[string]interface{})
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

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

func (h *APIHandler) SearchClusterMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		name        = h.GetParameterOrDefault(req, "name", "")
		queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		size     = h.GetIntOrDefault(req, "size", 20)
		from     = h.GetIntOrDefault(req, "from", 0)
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


	response:=elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw,&response)

	var clusterIDs []interface{}

	for _,hit:=range  response.Hits.Hits{
		clusterIDs=append(clusterIDs,hit.ID)
	}

	//fetch cluster status
	q1:=orm.Query{}
	q1.Conds=orm.And(
		orm.Eq("metadata.category","elasticsearch"),
		orm.Eq("metadata.name","cluster_health"),
		orm.In("metadata.labels.cluster_id",clusterIDs))
	q1.AddSort("timestamp",orm.DESC)
	q1.Size=1

	err,results:=orm.Search(&event.Event{},&q1)

	healthMap:=map[string]interface{}{}
	for _,v:=range results.Result{
		result,ok:=v.(map[string]interface{})
		clusterID,ok:=util.GetMapValueByKeys([]string{"metadata","labels","cluster_id"},result)
		if ok{
			health,ok:=util.GetMapValueByKeys([]string{"payload","elasticsearch","cluster_health"},result)
			if ok{
				healthMap[util.ToString(clusterID)]=health
			}
		}
	}

	//fetch cluster metrics

	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req,60,(15))
	if err != nil {
		panic(err)
		return
	}

	fmt.Println(bucketSize)

	metricItems:=[]*common.MetricItem{}
	var bucketSizeStr ="10s"
	metricItem:=newMetricItem("cluster_summary", 2, "cluster")
	//metricItem.AddLine("Status","Cluster Status","Cluster health change","group1",
	//	"payload.elasticsearch.index_stats.primaries.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Indexing","Total Indexing","Number of documents being indexed for primary and replica shards.","group1",
		"payload.elasticsearch.index_stats.total.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Search","Total Search","Number of search requests being executed across primary and replica shards. A single search can run against multiple shards!","group1",
		"payload.elasticsearch.index_stats.total.search.query_total","max",bucketSizeStr,"query/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)

	query:=map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"terms":util.MapStr{
						"metadata.labels.cluster_id":clusterIDs,
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


	var top=len(clusterIDs)+1
	aggs := h.convertMetricItemsToAgg(metricItems)

	query["size"]=0
	query["aggs"]= util.MapStr{
		"group_by_level": util.MapStr{
			"terms": util.MapStr{
				"field": "metadata.labels.cluster_id",
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

	fmt.Println(string(util.MustToJSONBytes(query)))

	 h.getGroupMetrics(query)


	//combine
	for i,hit:=range  response.Hits.Hits{
		result:=util.MapStr{}
		result["metadata"]=hit.Source
		result["summary"]=healthMap[hit.ID]
		//result["metrics"]=getClusterMetrics(hit.ID)
		response.Hits.Hits[i].Source=result
		clusterIDs=append(clusterIDs,hit.ID)
	}

	h.WriteJSON(w, response,200)
}


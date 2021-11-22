package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type APIHandler struct {
	api.Handler
	Config common.ModuleConfig
}

func (h *APIHandler) Client() elastic.API {
	return elastic.GetClient(h.Config.Elasticsearch)
}

func (h *APIHandler) HandleCreateClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	var conf = &elastic.ElasticsearchConfig{}
	resBody := map[string] interface{}{
	}
	err := h.DecodeJSON(req, conf)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	// TODO validate data format
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	id := util.GetUUID()
	conf.Created = time.Now()
	conf.Enabled=true
	conf.Updated = conf.Created
	conf.Endpoint = fmt.Sprintf("%s://%s", conf.Schema, conf.Host)
	index:=orm.GetIndexName(elastic.ElasticsearchConfig{})
	_, err = esClient.Index(index, "", id, conf)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	resBody["_source"] = conf
	resBody["_id"] = id
	resBody["result"] = "created"
	conf.ID = id
	conf.Discovery.Enabled = true
	_, err = common.InitElasticInstance(*conf)
	if err != nil {
		log.Warn("error on init elasticsearch:", err)
	}

	h.WriteJSON(w, resBody,http.StatusOK)

}

func (h *APIHandler) HandleGetClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{}

	id := ps.ByName("id")
	indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
	getResponse, err := h.Client().Get(indexName, "", id)
	if err != nil {
		resBody["error"] = err.Error()
		if getResponse!=nil{
			h.WriteJSON(w, resBody, getResponse.StatusCode)
		}else{
			h.WriteJSON(w, resBody, http.StatusInternalServerError)
		}
		return
	}
	if basicAuth, ok := getResponse.Source["basic_auth"]; ok {
		if authMap, ok := basicAuth.(map[string]interface{}); ok {
			delete(authMap, "password")
		}
	}
	h.WriteJSON(w,getResponse,200)
}

func (h *APIHandler) HandleUpdateClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	var conf = map[string]interface{}{}
	resBody := map[string] interface{}{
	}
	err := h.DecodeJSON(req, &conf)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	id := ps.ByName("id")
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
	originConf, err := esClient.Get(indexName, "", id)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	source := originConf.Source
	for k, v := range conf {
		if k == "id" {
			continue
		}
		source[k] = v
	}

	if host, ok := conf["host"].(string); ok {
		if schema, ok := conf["schema"].(string); ok {
			source["endpoint"] =  fmt.Sprintf("%s://%s", schema, host)
		}
	}

	conf["updated"] = time.Now()
	_, err = esClient.Index(indexName, "", id, source)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	resBody["_source"] = conf
	resBody["_id"] = id
	resBody["result"] = "updated"

	//update config in heap
	confBytes, _ := json.Marshal(source)
	newConf := &elastic.ElasticsearchConfig{}
	json.Unmarshal(confBytes, newConf)
	newConf.ID = id
	newConf.Discovery.Enabled = true
	newConf.Enabled = true
	_, err = common.InitElasticInstance(*newConf)
	if err != nil {
		log.Warn("error on init elasticsearch:", err)
	}

	h.WriteJSON(w, resBody,http.StatusOK)}

func (h *APIHandler) HandleDeleteClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	id := ps.ByName("id")
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	response, err := esClient.Delete(orm.GetIndexName(elastic.ElasticsearchConfig{}), "", id, "wait_for")

	if err != nil {
		resBody["error"] = err.Error()
		if response!=nil{
			h.WriteJSON(w, resBody, response.StatusCode)
		}else{
			h.WriteJSON(w, resBody, http.StatusInternalServerError)
		}
		return
	}

	elastic.RemoveInstance(id)
	resBody["_id"] = id
	resBody["result"] = response.Result
	h.WriteJSON(w, resBody, response.StatusCode)
}

func (h *APIHandler) HandleSearchClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	var (
		name          = h.GetParameterOrDefault(req, "name", "")
		queryDSL      = `{"query":{"bool":{"filter":[%s]}}, "size": %d, "from": %d}`
		strSize       = h.GetParameterOrDefault(req, "size", "20")
		strFrom       = h.GetParameterOrDefault(req, "from", "0")
		filterBuilder = &strings.Builder{}
	)
	if name != ""{
		filterBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.keyword": "%s"}}`, name))
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	queryDSL = fmt.Sprintf(queryDSL, filterBuilder.String(), size, from)
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	res, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.ElasticsearchConfig{}), []byte(queryDSL))

	if len(res.Hits.Hits) > 0 {
		for _, hit := range res.Hits.Hits {
			if basicAuth, ok := hit.Source["basic_auth"]; ok {
				if authMap, ok := basicAuth.(map[string]interface{}); ok {
					delete(authMap, "password")
				}
			}
		}
	}

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, res, http.StatusOK)
}

//new
func (h *APIHandler) HandleClusterMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{}
	id := ps.ByName("id")

	summary:=map[string]interface{}{}
	client := elastic.GetClient(h.Config.Elasticsearch)
	var query = util.MapStr{
		"sort": util.MapStr{
			"timestamp": util.MapStr{
				"order": "desc",
			},
		},
		"size": 1,
	}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"term":util.MapStr{
						"metadata.labels.cluster_id":util.MapStr{
							"value": id,
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
							"value": "cluster_stats",
						},
					},
				},
			},
		},
	}
	searchRes, err := client.SearchWithRawQueryDSL(orm.GetIndexName(event.Event{}), util.MustToJSONBytes(query))
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if len(searchRes.Hits.Hits) > 0 {
		status, _ := util.MapStr(searchRes.Hits.Hits[0].Source).GetValue("payload.elasticsearch.cluster_stats")
		statusMap := util.MapStr(status.(map[string]interface{}))
		summary["cluster_name"], _ = statusMap.GetValue("cluster_name")
		summary["status"],_ = statusMap.GetValue("status")
		summary["indices_count"],_ =statusMap.GetValue("indices.count")
		summary["total_shards"],_ = statusMap.GetValue("indices.shards.total")
		summary["primary_shards"],_ = statusMap.GetValue("indices.shards.primaries")
		summary["replication_shards"],_ = statusMap.GetValue("indices.shards.replication")
		//summary["unassigned_shards"]=status.Indices["shards"].(map[string]interface{})["primaries"]

		summary["document_count"], _ = statusMap.GetValue("indices.docs.count")
		summary["deleted_document_count"],_ = statusMap.GetValue("indices.docs.deleted")

		summary["used_store_bytes"],_ = statusMap.GetValue("indices.store.size_in_bytes")

		summary["max_store_bytes"],_ = statusMap.GetValue("nodes.fs.total_in_bytes")
		summary["available_store_bytes"],_ =  statusMap.GetValue("nodes.fs.available_in_bytes")

		summary["fielddata_bytes"],_ = statusMap.GetValue("indices.fielddata.memory_size_in_bytes")
		summary["fielddata_evictions"],_ = statusMap.GetValue("indices.fielddata.evictions")

		summary["query_cache_bytes"],_ = statusMap.GetValue("indices.query_cache.memory_size_in_bytes")
		summary["query_cache_total_count"],_ = statusMap.GetValue("indices.query_cache.total_count")
		summary["query_cache_hit_count"],_ = statusMap.GetValue("indices.query_cache.hit_count")
		summary["query_cache_miss_count"],_ = statusMap.GetValue("indices.query_cache.miss_count")
		summary["query_cache_evictions"],_ = statusMap.GetValue("indices.query_cache.evictions")

		summary["segments_count"],_ = statusMap.GetValue("indices.segments.count")
		summary["segments_memory_in_bytes"],_ =statusMap.GetValue("indices.segments.memory_in_bytes")

		summary["nodes_count"],_ = statusMap.GetValue("nodes.count.total")
		summary["version"],_ = statusMap.GetValue("nodes.versions")

		summary["mem_total_in_bytes"],_ =  statusMap.GetValue("nodes.os.mem.total_in_bytes")
		summary["mem_used_in_bytes"],_ =  statusMap.GetValue("nodes.os.mem.used_in_bytes")
		summary["mem_used_percent"],_ = statusMap.GetValue("nodes.os.mem.used_percent")

		summary["uptime"],_ = statusMap.GetValue("nodes.jvm.max_uptime_in_millis")
		summary["used_jvm_bytes"],_ =  statusMap.GetValue("nodes.jvm.mem.heap_used_in_bytes")
		summary["max_jvm_bytes"],_ = statusMap.GetValue("nodes.jvm.mem.heap_max_in_bytes")
	}

	resBody["summary"] = summary

	bucketSize, min, max, err := h.getMetricParams(req)

	//fmt.Println(min," vs ",max,",",rangeFrom,rangeTo,"range hours:",hours)

	//metrics:=h.GetClusterMetrics(id,bucketSize,min,max)
	metrics := h.GetClusterMetrics(id, bucketSize, min, max)
	resBody["metrics"] = metrics

	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}

}

func (h *APIHandler) getMetricParams(req *http.Request) (int, int, int, error) {
	bucketSize := h.GetIntOrDefault(req, "bucket_size", 10)      //默认 10，每个 bucket 的时间范围，单位秒
	metricCount := h.GetIntOrDefault(req, "metric_count", 15*60) //默认 15分钟的区间，每分钟15个指标，也就是 15*60 个 bucket

	now := time.Now()
	//min,max are unix nanoseconds

	minStr := h.Get(req, "min", "")
	maxStr := h.Get(req, "max", "")

	var min, max int
	var rangeFrom, rangeTo time.Time
	var err error
	if minStr == "" {
		rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
	} else {
		//try 2021-08-21T14:06:04.818Z
		rangeFrom, err = util.ParseStandardTime(minStr)
		if err != nil {
			//try 1629637500000
			v, err := util.ToInt(minStr)
			if err != nil {
				log.Error("invalid timestamp:", minStr, err)
				rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
			} else {
				rangeFrom = util.FromUnixTimestamp(int64(v) / 1000)
			}
		}
	}

	if maxStr == "" {
		rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
	} else {
		rangeTo, err = util.ParseStandardTime(maxStr)
		if err != nil {
			v, err := util.ToInt(maxStr)
			if err != nil {
				log.Error("invalid timestamp:", maxStr, err)
				rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
			} else {
				rangeTo = util.FromUnixTimestamp(int64(v) / 1000)
			}
		}
	}

	min = int(rangeFrom.UnixNano() / 1e6)
	max = int(rangeTo.UnixNano() / 1e6)
	hours := rangeTo.Sub(rangeFrom).Hours()
	if hours <= 1 {
		bucketSize = 60
	}else if hours < 3 {
		bucketSize = 90
	} else if hours < 6 {
		bucketSize = 120
	} else if hours < 12 {
		bucketSize = 60 * 3
	} else if hours < 25 { //1day
		bucketSize = 60 * 5 * 2
	} else if hours <= 7*24+1 { //7days
		bucketSize = 60 * 15 * 2
	} else if hours <= 15*24+1 { //15days
		bucketSize = 60 * 30 * 2
	} else if hours < 30*24+1 { //<30 days
		bucketSize = 60 * 60 //hourly
	} else if hours <= 30*24+1 { //<30days
		bucketSize = 12 * 60 * 60 //half daily
	} else if hours >= 30*24+1 { //>30days
		bucketSize = 60 * 60 * 24 //daily bucket
	}
	return bucketSize, min, max, err
}
func (h *APIHandler) HandleNodeMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{}
	id := ps.ByName("id")
	bucketSize, min, max, err := h.getMetricParams(req)
	nodeName := h.Get(req, "node_name", "")
	top := h.GetIntOrDefault(req, "top", 5)
	resBody["metrics"] = h.getNodeMetrics(id, bucketSize, min, max, nodeName, top)

	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}
}

func (h *APIHandler) HandleIndexMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{}
	id := ps.ByName("id")
	bucketSize, min, max, err := h.getMetricParams(req)
	indexName := h.Get(req, "index_name", "")
	top := h.GetIntOrDefault(req, "top", 5)
	resBody["metrics"] = h.getIndexMetrics(id, bucketSize, min, max, indexName, top)

	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}
}


//TODO, use expired hash
var clusters = map[string]elastic.ElasticsearchConfig{}
var clustersMutex = &sync.RWMutex{}

func (h *APIHandler) GetClusterClient(id string) (bool,elastic.API,error) {
	clustersMutex.RLock()
	config,ok:=clusters[id]
	clustersMutex.RUnlock()
	if !ok{
		indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
		getResponse, err := h.Client().Get(indexName, "", id)
		if err != nil {
			return false, nil, err
		}

		bytes:=util.MustToJSONBytes(getResponse.Source)
		cfg:= elastic.ElasticsearchConfig{}
		err=util.FromJSONBytes(bytes,&cfg)
		if err != nil {
			return false, nil, err
		}

		if getResponse.StatusCode==http.StatusNotFound{
			return false, nil, err
		}

		cfg.ID=id
		clustersMutex.Lock()
		clusters[id]=cfg
		clustersMutex.Unlock()
		config = cfg
	}

	client, _:=common.InitClientWithConfig(config)
	elastic.RegisterInstance(id, config, client)

	return true,client,nil
}


func (h *APIHandler) GetClusterHealth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(id)

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",id)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	health:=client.ClusterHealth()

	h.WriteJSON(w,health,200)
}

func (h *APIHandler) HandleGetNodesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	metaData := elastic.GetMetadata(id)
	result := util.MapStr{}
	for k, nodeInfo := range *metaData.Nodes {
		result[k] = util.MapStr{
			"name": nodeInfo.Name,
			"transport_address": nodeInfo.TransportAddress,
		}
	}
	h.WriteJSON(w, result,200)
}

func newMetricItem(metricKey string, order int, group string) *common.MetricItem  {
	metricItem:=common.MetricItem{
		Order: order,
		Key: metricKey,
		Group: group,
	}

	//axis
	metricItem.Axis=[]*common.MetricAxis{}

	//lines
	metricItem.Lines=[]*common.MetricLine{}

	return &metricItem
}

type GroupMetricItem struct {
	Key string
	Field string
	ID string
	IsDerivative bool
	Units string
	FormatType string
	MetricItem *common.MetricItem
	Field2 string
	Calc func(value, value2 float64) float64
}
const (
	SystemGroupKey = "system"
	OperationGroupKey = "operations"
	LatencyGroupKey = "latency"
	CacheGroupKey = "cache"
	HttpGroupKey = "http"
	MemoryGroupKey = "memory"
	StorageGroupKey = "storage"
)

func (h *APIHandler) getNodeMetrics(clusterID string, bucketSize int, min, max int, nodeName string, top int) map[string]*common.MetricItem{
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
				return value2/value * 100
			},
	})
	// 索引速率
	indexMetric:=newMetricItem("indexing_rate", 3, OperationGroupKey)
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
	indexLatencyMetric:=newMetricItem("indexing_latency", 4, LatencyGroupKey)
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

	queryMetric:=newMetricItem("query_rate", 5, OperationGroupKey)
	queryMetric.AddAxi("query rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "query_rate",
		Field: "payload.elasticsearch.node_stats.indices.search.query_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryMetric,
		FormatType: "num",
		Units: "doc/s",
	})

	// 查询延时
	queryLatencyMetric:=newMetricItem("query_latency", 6, LatencyGroupKey)
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

	fetchMetric:=newMetricItem("fetch_rate", 5, OperationGroupKey)
	fetchMetric.AddAxi("fetch rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "fetch_rate",
		Field: "payload.elasticsearch.node_stats.indices.search.fetch_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: fetchMetric,
		FormatType: "num",
		Units: "doc/s",
	})

	// fetch延时
	fetchLatencyMetric:=newMetricItem("fetch_latency", 6, LatencyGroupKey)
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
	refreshLatencyMetric:=newMetricItem("refresh_latency", 8, LatencyGroupKey)
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
	flushLatencyMetric:=newMetricItem("flush_latency", 9, LatencyGroupKey)
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
	// QueryCache 内存占用大小
	queryCacheMetric:=newMetricItem("query_cache", 10, CacheGroupKey)
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
	// QueryCache 内存占用大小
	requestCacheMetric:=newMetricItem("request_cache", 11, CacheGroupKey)
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

	// docs 数量
	docsCountMetric:=newMetricItem("docs_count", 17, StorageGroupKey)
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
	jvmUsedPercentMetric:=newMetricItem("jvm_heap_used_percent", 19, MemoryGroupKey)
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

func (h *APIHandler) getIndexMetrics(clusterID string, bucketSize int, min, max int, indexName string, top int) map[string]*common.MetricItem{
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

type MetricData map[string][][]interface{}

func (h *APIHandler) getMetrics( query map[string]interface{}, grpMetricItems []GroupMetricItem, bucketSize int) map[string]*common.MetricItem{
	defer func() {
		if err:= recover(); err != nil {
			fmt.Println(err)
			debug.PrintStack()
		}
	}()
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	response,err:=elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(orm.GetIndexName(event.Event{}),util.MustToJSONBytes(query))
	if err!=nil{
		log.Error(err)
		panic(err)
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

func (h *APIHandler) GetClusterMetrics(id string,bucketSize int, min, max int) map[string]*common.MetricItem {

	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)

	metricItems:=[]*common.MetricItem{}

	metricItem:=newMetricItem("cluster_throughput", 1, OperationGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	metricItem.AddAxi("searching","group2",common.PositionRight,"num","0,0","0,0.[00]",5,false)

	metricItem.AddLine("Indexing Rate","Total Indexing","Number of documents being indexed for primary and replica shards.","group1","payload.elasticsearch.index_stats.total.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Indexing Rate","Primary Indexing","Number of documents being indexed for primary shards.","group1","payload.elasticsearch.index_stats.primaries.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Search Rate","Total Searching","Number of search requests being executed across primary and replica shards. A single search can run against multiple shards!","group2","payload.elasticsearch.index_stats.total.search.query_total","max",bucketSizeStr,"query/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)



	metricItem=newMetricItem("cluster_latency", 2, LatencyGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	metricItem.AddAxi("searching","group2",common.PositionRight,"num","0,0","0,0.[00]",5,false)

	metricItem.AddLine("Indexing","Indexing Latency","Average latency for indexing documents.","group1","payload.elasticsearch.index_stats.total.indexing.index_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Indexing","Delete Latency","Average latency for delete documents.","group1","payload.elasticsearch.index_stats.total.indexing.delete_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Searching","Query Latency","Average latency for searching query.","group2","payload.elasticsearch.index_stats.total.search.query_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Searching","Fetch Latency","Average latency for searching fetch.","group2","payload.elasticsearch.index_stats.total.search.fetch_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Searching","Scroll Latency","Average latency for searching fetch.","group2","payload.elasticsearch.index_stats.total.search.scroll_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)

	clusterMetricItems:=[]*common.MetricItem{}
	metricItem=newMetricItem("cluster_storage", 3, StorageGroupKey)
	metricItem.AddAxi("indices_storage","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	metricItem.AddAxi("available_storage","group2",common.PositionRight,"bytes","0.[0]","0.[0]",5,true)

	metricItem.AddLine("Disk","Indices Storage","","group1","payload.elasticsearch.cluster_stats.indices.store.size_in_bytes","max",bucketSizeStr,"","bytes","0,0.[00]","0,0.[00]",false,false)
	metricItem.AddLine("Disk","Available Disk","","group2","payload.elasticsearch.cluster_stats.nodes.fs.available_in_bytes","max",bucketSizeStr,"","bytes","0,0.[00]","0,0.[00]",false,false)

	clusterMetricItems=append(clusterMetricItems,metricItem)

	metricItem=newMetricItem("cluster_documents", 4, StorageGroupKey)
	metricItem.AddAxi("documents","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddAxi("counts","group2",common.PositionRight,"num","0,0","0,0.[00]",5,false)

	metricItem.AddLine("Documents Count","Documents Count","","group1","payload.elasticsearch.cluster_stats.indices.docs.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	metricItem.AddLine("Indices Count","Indices Count","","group3","payload.elasticsearch.cluster_stats.indices.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	metricItem.AddLine("Shards Count","Shards Count","","group2","payload.elasticsearch.cluster_stats.indices.shards.total","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)

	clusterMetricItems=append(clusterMetricItems,metricItem)


	metricItem=newMetricItem("system_load", 5, SystemGroupKey)
	metricItem.AddAxi("load","group6",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)

	metricItem.AddLine("Load","System Load","","group6","payload.elasticsearch.cluster_stats.nodes.process.cpu.percent","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	clusterMetricItems=append(clusterMetricItems,metricItem)

	metricItem=newMetricItem("system_memory", 6, MemoryGroupKey)
	metricItem.AddAxi("memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	metricItem.AddAxi("memory","group2",common.PositionRight,"ratio","0.[0]","0.[0]",5,false)

	metricItem.AddLine("JVM Max Heap","Max Heap","","group1","payload.elasticsearch.cluster_stats.nodes.jvm.mem.heap_max_in_bytes","max",bucketSizeStr,"","bytes","0.[00]","0.[00]",false,false)
	metricItem.AddLine("JVM Used Heap","Used Heap","","group1","payload.elasticsearch.cluster_stats.nodes.jvm.mem.heap_used_in_bytes","max",bucketSizeStr,"","bytes","0.[00]","0.[00]",false,false)
	metricItem.AddLine("OS Used Percent","OS Used Percent","","group2","payload.elasticsearch.cluster_stats.nodes.os.mem.used_percent","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	clusterMetricItems=append(clusterMetricItems,metricItem)


	query:=map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"term":util.MapStr{
						"metadata.labels.cluster_id":util.MapStr{
							"value": id,
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
						"metadata.labels.index_id": util.MapStr{
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

	indexMetricsResult := h.getSingleMetrics(metricItems, query, bucketSize)

	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must": []util.MapStr{
				{
					"term":util.MapStr{
						"metadata.labels.cluster_id":util.MapStr{
							"value": id,
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
							"value": "cluster_stats",
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
	clusterMetricsResult := h.getSingleMetrics(clusterMetricItems, query, bucketSize)
	for k, v := range clusterMetricsResult {
		indexMetricsResult[k] = v
	}
	return indexMetricsResult
}

func (h *APIHandler) getSingleMetrics(metricItems []*common.MetricItem, query map[string]interface{}, bucketSize int) map[string]*common.MetricItem {
	metricData := map[string][][]interface{}{}

	aggs := map[string]interface{}{}

	for _, metricItem := range metricItems {
		for _, line := range metricItem.Lines {

			metricData[line.Metric.DataKey] = [][]interface{}{}

			aggs[line.Metric.ID] = util.MapStr{
				"max": util.MapStr{
					"field": line.Metric.Field,
				},
			}

			if line.Metric.IsDerivative {
				//add which metric keys to extract
				aggs[line.Metric.ID+"_deriv"] = util.MapStr{
					"derivative": util.MapStr{
						"buckets_path": line.Metric.ID,
					},
				}
			}
		}
	}
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)

	query["size"] = 0
	query["aggs"] = util.MapStr{
		"dates": util.MapStr{
			"date_histogram": util.MapStr{
				"field":          "timestamp",
				"fixed_interval": bucketSizeStr,
			},
			"aggs": aggs,
		},
	}

	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(orm.GetIndexName(event.Event{}), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
		panic(err)
	}

	var minDate, maxDate int64
	if response.StatusCode == 200 {
		for _, v := range response.Aggregations {
			for _, bucket := range v.Buckets {
				v, ok := bucket["key"].(float64)
				if !ok {
					panic("invalid bucket key")
				}
				dateTime := (int64(v))
				minDate = util.MinInt64(minDate, dateTime)
				maxDate = util.MaxInt64(maxDate, dateTime)
				for mk1, mv1 := range metricData {
					v1, ok := bucket[mk1]
					if ok {
						v2, ok := v1.(map[string]interface{})
						if ok {
							v3, ok := v2["value"].(float64)
							if ok {
								if strings.HasSuffix(mk1, "_deriv") {
									v3 = v3 / float64(bucketSize)
								}
								//only keep positive value
								if v3 < 0 {
									continue
								}
								//v4:=int64(v3)/int64(bucketSize)
								points := []interface{}{dateTime, v3}
								metricData[mk1] = append(mv1, points)
							}
						}
					}
				}
			}
		}
	}

	result := map[string]*common.MetricItem{}

	for _, metricItem := range metricItems {
		for _, line := range metricItem.Lines {
			line.TimeRange = common.TimeRange{Min: minDate, Max: maxDate}
			line.Data = metricData[line.Metric.DataKey]
		}
		result[metricItem.Key] = metricItem
	}

	return result
}

func (h *APIHandler) GetClusterStatusAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var status = map[string]interface{}{}
	elastic.WalkConfigs(func(k, value interface{}) bool {
		key:=k.(string)
		cfg,ok:=value.(*elastic.ElasticsearchConfig)
		if ok&&cfg!=nil{
			meta := elastic.GetOrInitMetadata(cfg)
			status[key] = map[string]interface{}{
				"health": meta.Health,
				"available": meta.IsAvailable(),
			}
		}
		return true
	})
	h.WriteJSON(w, status, http.StatusOK)
}

func (h *APIHandler) HandleTestConnectionAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		freq = fasthttp.AcquireRequest()
		fres = fasthttp.AcquireResponse()
		resBody = map[string] interface{}{}
	)
	defer func() {
		fasthttp.ReleaseRequest(freq)
		fasthttp.ReleaseResponse(fres)
	}()
	var config = &elastic.ElasticsearchConfig{}
	err := h.DecodeJSON(req, &config)
	if err != nil {
		resBody["error"] = fmt.Sprintf("json decode error: %v", err)
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}
	defer req.Body.Close()
	config.Endpoint = fmt.Sprintf("%s://%s", config.Schema, config.Host)
	freq.SetRequestURI(fmt.Sprintf("%s/", config.Endpoint))
	freq.Header.SetMethod("GET")
	if config.BasicAuth != nil && strings.TrimSpace(config.BasicAuth.Username) != ""{
		freq.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}

	client := &fasthttp.Client{
		MaxConnsPerHost: 1000,
		TLSConfig:       &tls.Config{InsecureSkipVerify: true},
		ReadTimeout: time.Second * 5,
	}
	err = client.Do(freq, fres)
	if err != nil {
		resBody["error"] = fmt.Sprintf("request error: %v", err)
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}
	b := fres.Body()
	clusterInfo := &elastic.ClusterInformation{}
	err = json.Unmarshal(b, clusterInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster info decode error: %v", err)
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}
	resBody["version"] = clusterInfo.Version.Number
	resBody["cluster_name"] = clusterInfo.ClusterName

	//fetch cluster health info
	freq.SetRequestURI(fmt.Sprintf("%s/_cluster/health", config.Endpoint))
	fres.Reset()
	err = client.Do(freq, fres)
	if err != nil {
		resBody["error"] = fmt.Sprintf("request cluster health info error: %v", err)
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}
	var statusCode = fres.StatusCode()
	if statusCode == http.StatusUnauthorized {
		resBody["error"] = fmt.Sprintf("required authentication credentials")
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}

	healthInfo := &elastic.ClusterHealth{}
	err = json.Unmarshal(fres.Body(), &healthInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster health info decode error: %v", err)
		h.WriteJSON(w, resBody, http.StatusOK)
		return
	}
	resBody["status"] = healthInfo.Status
	resBody["number_of_nodes"] = healthInfo.NumberOfNodes
	resBody["number_of_data_nodes"] = healthInfo.NumberOf_data_nodes
	resBody["active_shards"] = healthInfo.ActiveShards

	h.WriteJSON(w, resBody, http.StatusOK)

}

func (h *APIHandler) GetMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	result:=util.MapStr{}
	elastic.WalkMetadata(func(key, value interface{}) bool {
		m:=util.MapStr{}
		k:=key.(string)
		if value==nil{
			return true
		}

		v,ok:=value.(*elastic.ElasticsearchMetadata)
		if ok{
			m["major_version"]=v.GetMajorVersion()
			m["seed_hosts"]=v.GetSeedHosts()
			m["state"]=v.ClusterState
			m["topology_version"]=v.NodesTopologyVersion
			m["nodes"]=v.Nodes
			m["indices"]=v.Indices
			m["health"]=v.Health
			m["aliases"]=v.Aliases
			m["primary_shards"]=v.PrimaryShards
			m["available"]=v.IsAvailable()
			m["schema"]=v.GetSchema()
			m["config"]=v.Config
			m["last_success"]=v.LastSuccess()
			result[k]=m
		}
		return true
	})

	h.WriteJSON(w, result, http.StatusOK)

}

func (h *APIHandler) GetHosts(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	result:=util.MapStr{}

	elastic.WalkHosts(func(key, value interface{}) bool {
		k := key.(string)
		if value == nil {
			return true
		}

		v, ok := value.(*elastic.NodeAvailable)
		if ok {
			result[k]=util.MapStr{
				"host":v.Host,
				"available":v.IsAvailable(),
				"last_success":v.LastSuccess(),
				"failure_tickets":v.FailureTickets(),
			}
		}
		return true
	})

	h.WriteJSON(w, result, http.StatusOK)

}
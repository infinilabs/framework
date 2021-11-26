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
		queryDSL      = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		strSize       = h.GetParameterOrDefault(req, "size", "20")
		strFrom     = h.GetParameterOrDefault(req, "from", "0")
		mustBuilder = &strings.Builder{}
	)
	if name != ""{
		mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.text": "%s"}}`, name))
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	res, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.ElasticsearchConfig{}), []byte(queryDSL))

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if len(res.Hits.Hits) > 0 {
		for _, hit := range res.Hits.Hits {
			if basicAuth, ok := hit.Source["basic_auth"]; ok {
				if authMap, ok := basicAuth.(map[string]interface{}); ok {
					delete(authMap, "password")
				}
			}
		}
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

func (h *APIHandler) getMetricParams(req *http.Request) (int, int64, int64, error) {
	bucketSize := h.GetIntOrDefault(req, "bucket_size", 10)      //默认 10，每个 bucket 的时间范围，单位秒
	metricCount := h.GetIntOrDefault(req, "metric_count", 15*60) //默认 15分钟的区间，每分钟15个指标，也就是 15*60 个 bucket

	now := time.Now()
	//min,max are unix nanoseconds

	minStr := h.Get(req, "min", "")
	maxStr := h.Get(req, "max", "")

	var min, max int64
	var rangeFrom, rangeTo time.Time
	var err error
	if minStr == "" {
		rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
	} else {
		//try 2021-08-21T14:06:04.818Z
		rangeFrom, err = util.ParseStandardTime(minStr)
		if err != nil {
			//try 1629637500000
			v, err := util.ToInt64(minStr)
			if err != nil {
				log.Error("invalid timestamp:", minStr, err)
				rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
			} else {
				rangeFrom = util.FromUnixTimestamp(v / 1000)
			}
		}
	}

	if maxStr == "" {
		rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
	} else {
		rangeTo, err = util.ParseStandardTime(maxStr)
		if err != nil {
			v, err := util.ToInt64(maxStr)
			if err != nil {
				log.Error("invalid timestamp:", maxStr, err)
				rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
			} else {
				rangeTo = util.FromUnixTimestamp(int64(v) / 1000)
			}
		}
	}

	min = rangeFrom.UnixNano() / 1e6
	max = rangeTo.UnixNano() / 1e6
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

	health,_:=client.ClusterHealth()

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
	JVMGroupKey = "JVM"
	ThreadPoolGroupKey = "thread_pool"
)

func (h *APIHandler) GetClusterMetrics(id string,bucketSize int, min, max int64) map[string]*common.MetricItem {

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
	metricItem.AddAxi("count","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddAxi("deleted","group2",common.PositionRight,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Documents Count","Documents Count","","group1","payload.elasticsearch.cluster_stats.indices.docs.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	metricItem.AddLine("Documents Deleted","Documents Deleted","","group2","payload.elasticsearch.cluster_stats.indices.docs.deleted","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	clusterMetricItems=append(clusterMetricItems,metricItem)

	metricItem=newMetricItem("cluster_indices", 4, StorageGroupKey)
	metricItem.AddAxi("count","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Indices Count","Indices Count","","group1","payload.elasticsearch.cluster_stats.indices.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	clusterMetricItems=append(clusterMetricItems,metricItem)

	//Todo shard count
	//metricItem=newMetricItem("shard_count", 4, StorageGroupKey)
	//metricItem.AddAxi("counts","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	//metricItem.AddLine("primary","Indices Count","","group1","payload.elasticsearch.cluster_stats.indices.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	//metricItem.AddLine("Shards Count","Shards Count","","group1","payload.elasticsearch.cluster_stats.indices.shards.total","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	//
	//clusterMetricItems=append(clusterMetricItems,metricItem)


	//metricItem=newMetricItem("system_load", 5, SystemGroupKey)
	//metricItem.AddAxi("load","group6",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)
	//
	//metricItem.AddLine("Load","System Load","","group6","payload.elasticsearch.cluster_stats.nodes.process.cpu.percent","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	//clusterMetricItems=append(clusterMetricItems,metricItem)
	//
	//metricItem=newMetricItem("system_memory", 6, MemoryGroupKey)
	//metricItem.AddAxi("memory","group1",common.PositionLeft,"bytes","0.[0]","0.[0]",5,true)
	//metricItem.AddAxi("memory","group2",common.PositionRight,"ratio","0.[0]","0.[0]",5,false)
	//
	//metricItem.AddLine("JVM Max Heap","Max Heap","","group1","payload.elasticsearch.cluster_stats.nodes.jvm.mem.heap_max_in_bytes","max",bucketSizeStr,"","bytes","0.[00]","0.[00]",false,false)
	//metricItem.AddLine("JVM Used Heap","Used Heap","","group1","payload.elasticsearch.cluster_stats.nodes.jvm.mem.heap_used_in_bytes","max",bucketSizeStr,"","bytes","0.[00]","0.[00]",false,false)
	//metricItem.AddLine("OS Used Percent","OS Used Percent","","group2","payload.elasticsearch.cluster_stats.nodes.os.mem.used_percent","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	//clusterMetricItems=append(clusterMetricItems,metricItem)

	metricItem=newMetricItem("node_count", 7, MemoryGroupKey)
	metricItem.AddAxi("count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)

	metricItem.AddLine("Master Node","Master Node","","group1","payload.elasticsearch.cluster_stats.nodes.count.master","max",bucketSizeStr,"","num","0.[00]","0.[00]",false,false)
	metricItem.AddLine("Data Node","Data Node","","group1","payload.elasticsearch.cluster_stats.nodes.count.data","max",bucketSizeStr,"","num","0.[00]","0.[00]",false,false)
	metricItem.AddLine("Coordinating Node","Coordinating Node","","group1","payload.elasticsearch.cluster_stats.nodes.count.coordinating_only","max",bucketSizeStr,"","num","0.[00]","0.[00]",false,false)
	metricItem.AddLine("Ingest Node","Ingest Node","","group1","payload.elasticsearch.cluster_stats.nodes.count.ingest","max",bucketSizeStr,"","num","0.[00]","0.[00]",false,false)

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
	statusMetric, err := h.getClusterStatusMetric(id,min,max,bucketSize)
	if err == nil {
		indexMetricsResult["cluster_health"] = statusMetric
	}
	return indexMetricsResult
}

func (h *APIHandler) getClusterStatusMetric(id string, min, max int64, bucketSize int)(*common.MetricItem, error){
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	query := util.MapStr{
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.labels.cluster_id": util.MapStr{
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
		},
		"aggs": util.MapStr{
			"dates": util.MapStr{
				"date_histogram": util.MapStr{
					"field": "timestamp",
					"interval": bucketSizeStr,
				},
				"aggs": util.MapStr{
					"group_status": util.MapStr{
						"terms": util.MapStr{
							"field": "payload.elasticsearch.cluster_stats.status",
							"size": 5,
						},
					},
				},
			},
		},
	}
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(orm.GetIndexName(event.Event{}), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	var minDate, maxDate int64
	metricData := map[string][][]interface{}{
		"red": [][]interface{}{},
		"yellow": [][]interface{}{},
		"green": [][]interface{}{},
	}
	metricItem:=newMetricItem("cluster_health", 7, MemoryGroupKey)
	metricItem.AddAxi("percent","group1",common.PositionLeft,"ratio","0.[0]","0.[0]",5,true)

	metricItem.AddLine("Red","Red","","group1","payload.elasticsearch.cluster_stats.status","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	metricItem.AddLine("Yellow","Yellow","","group1","payload.elasticsearch.cluster_stats.status","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)
	metricItem.AddLine("Green","Green","","group1","payload.elasticsearch.cluster_stats.status","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)

	if response.StatusCode == 200 {
		for _, bucket := range response.Aggregations["dates"].Buckets {
			v, ok := bucket["key"].(float64)
			if !ok {
				log.Error("invalid bucket key")
				return nil, fmt.Errorf("invalid bucket key")
			}
			dateTime := (int64(v))
			minDate = util.MinInt64(minDate, dateTime)
			maxDate = util.MaxInt64(maxDate, dateTime)
			totalCount := bucket["doc_count"].(float64)
			if grpStatus, ok := bucket["group_status"].(map[string]interface{}); ok {
				if statusBks, ok := grpStatus["buckets"].([]interface{}); ok {
					for _, statusBk := range statusBks {
						if bkMap, ok := statusBk.(map[string]interface{}); ok {
							statusKey := bkMap["key"].(string)
							count := bkMap["doc_count"].(float64)
							metricData[statusKey] = append(metricData[statusKey], []interface{}{
								dateTime, count/totalCount * 100,
							})
						}
					}
				}
			}
		}
	}
	for _, line := range metricItem.Lines {
		key := strings.ToLower(line.Metric.Title)
		line.Color = key
		line.Data = metricData[key]
	}
	return metricItem, nil
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
		log.Errorf("json decode error: %v", err)
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
		log.Error( "test_connection", "request error: ", err)
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
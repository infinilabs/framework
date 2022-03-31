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
	"math"
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
		log.Error(err)
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
	_, err = esClient.Index(index, "", id, conf, "wait_for")
	if err != nil {
		log.Error(err)
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
		log.Error(err)
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
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	id := ps.ByName("id")
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
	originConf, err := esClient.Get(indexName, "", id)
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	source := originConf.Source
	for k, v := range conf {
		if k == "id" {
			continue
		}
		if k == "basic_auth" {
			if authMap, ok := v.(map[string]interface{}); ok {
				if pwd, ok := authMap["password"]; !ok || (ok && pwd =="") {
					authMap["password"] = source[k].(map[string]interface{})["password"]
				}
			}
		}
		source[k] = v
	}

	if host, ok := conf["host"].(string); ok {
		if schema, ok := conf["schema"].(string); ok {
			source["endpoint"] =  fmt.Sprintf("%s://%s", schema, host)
		}
	}

	conf["updated"] = time.Now()
	_, err = esClient.Index(indexName, "", id, source, "wait_for")
	if err != nil {
		log.Error(err)
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
		log.Error(err)
		resBody["error"] = err.Error()
		if response!=nil{
			h.WriteJSON(w, resBody, response.StatusCode)
		}else{
			h.WriteJSON(w, resBody, http.StatusInternalServerError)
		}
		return
	}
	delDsl := util.MapStr{
		"query": util.MapStr{
			"match": util.MapStr{
				"metadata.cluster_id": id,
			},
		},
	}
	_, err = esClient.DeleteByQuery(orm.GetIndexName(elastic.NodeConfig{}), util.MustToJSONBytes(delDsl))
	if err != nil {
		log.Error(err)
	}
	_, err = esClient.DeleteByQuery(orm.GetIndexName(elastic.IndexConfig{}), util.MustToJSONBytes(delDsl))
	if err != nil {
		log.Error(err)
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
		log.Error(err)
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

func (h *APIHandler) HandleMetricsSummaryAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
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
	searchRes, err := client.SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		resBody["error"] = err.Error()
		log.Error("MetricsSummary search error: ", err)
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if len(searchRes.Hits.Hits) > 0 {
		sourceMap := util.MapStr(searchRes.Hits.Hits[0].Source)
		summary["timestamp"],_= sourceMap.GetValue("timestamp")
		status, _ := sourceMap.GetValue("payload.elasticsearch.cluster_stats")
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
							"value": "cluster_health",
						},
					},
				},
			},
		},
	}
	searchRes, err = client.SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		log.Error("MetricsSummary search error: ", err)
	}else{
		if len(searchRes.Hits.Hits) > 0 {
			health, _ := util.MapStr(searchRes.Hits.Hits[0].Source).GetValue("payload.elasticsearch.cluster_health")
			healthMap := util.MapStr(health.(map[string]interface{}))
			summary["unassigned_shards"], _ = healthMap.GetValue("unassigned_shards")
		}
	}

	resBody["summary"] = summary
	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}
}

//new
func (h *APIHandler) HandleClusterMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{}
	id := ps.ByName("id")

	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req,10,90)
	if err != nil {
		panic(err)
		return
	}

	//fmt.Println(min," vs ",max,",",rangeFrom,rangeTo,"range hours:",hours)

	//metrics:=h.GetClusterMetrics(id,bucketSize,min,max)
	isOverview := h.GetIntOrDefault(req, "overview", 0)
	var metrics interface{}
	if isOverview == 1 {
		metrics = h.GetClusterIndexMetrics(id, bucketSize, min, max)
	}else{
		metrics = h.GetClusterMetrics(id, bucketSize, min, max)
	}

	resBody["metrics"] = metrics

	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}

}

func (h *APIHandler) HandleNodeMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{}
	id := ps.ByName("id")
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req,10,90)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
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
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req,10,90)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	indexName := h.Get(req, "index_name", "")
	top := h.GetIntOrDefault(req, "top", 5)
	resBody["metrics"] = h.getIndexMetrics(id, bucketSize, min, max, indexName, top)

	err = h.WriteJSON(w, resBody, http.StatusOK)
	if err != nil {
		log.Error(err)
	}
}
func (h *APIHandler) HandleQueueMetricsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{}
	id := ps.ByName("id")
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req,10,90)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	nodeName := h.Get(req, "node_name", "")
	top := h.GetIntOrDefault(req, "top", 5)
	resBody["metrics"] = h.getThradPoolMetrics(id, bucketSize, min, max, nodeName, top)

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
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",id)
		log.Warn(resBody["error"])
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
	if metaData==nil||metaData.Nodes==nil{
		h.WriteError(w,"nodes metadata not found",500)
		return
	}
	for k, nodeInfo := range *metaData.Nodes {
		result[k] = util.MapStr{
			"name": nodeInfo.Name,
			"transport_address": nodeInfo.TransportAddress,
		}
	}
	h.WriteJSON(w, result,200)
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
	TransportGroupKey = "transport"
	DocumentGroupKey = "document"
)

func (h *APIHandler) GetClusterMetrics(id string,bucketSize int, min, max int64) map[string]*common.MetricItem {

	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)

	clusterMetricItems:=[]*common.MetricItem{}
	metricItem:=newMetricItem("cluster_storage", 8, StorageGroupKey)
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

	metricItem=newMetricItem("cluster_indices", 6, StorageGroupKey)
	metricItem.AddAxi("count","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Indices Count","Indices Count","","group1","payload.elasticsearch.cluster_stats.indices.count","max",bucketSizeStr,"","num","0,0.[00]","0,0.[00]",false,false)
	clusterMetricItems=append(clusterMetricItems,metricItem)

	metricItem=newMetricItem("node_count", 5, MemoryGroupKey)
	metricItem.AddAxi("count","group1",common.PositionLeft,"num","0.[0]","0.[0]",5,true)
	meta := elastic.GetMetadata(id)
	if meta!=nil&& !meta.IsAvailable(){

	}
	majorVersion:=meta.GetMajorVersion()
	metricItem.AddLine("Total", "Total Nodes", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.total", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)

	//TODO check version difference
	if majorVersion < 5 {
		metricItem.AddLine("Master Only", "Master Only", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.master_only", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
		metricItem.AddLine("Data Node", "Data Only", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.data_only", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
		metricItem.AddLine("Master Data", "Master Data", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.master_data", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
	}else {
		metricItem.AddLine("Master Node", "Master Node", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.master", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
		metricItem.AddLine("Data Node", "Data Node", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.data", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
		metricItem.AddLine("Coordinating Node Only", "Coordinating Node Only", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.coordinating_only", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
		metricItem.AddLine("Ingest Node", "Ingest Node", "", "group1", "payload.elasticsearch.cluster_stats.nodes.count.ingest", "max", bucketSizeStr, "", "num", "0.[00]", "0.[00]", false, false)
	}

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
	//todo: since there is four queries, we can change these query to async
	indexMetricsResult := h.GetClusterIndexMetrics(id, bucketSize, min, max)
	clusterMetricsResult := h.getSingleMetrics(clusterMetricItems, query, bucketSize)
	for k, v := range clusterMetricsResult {
		indexMetricsResult[k] = v
	}
	statusMetric, err := h.getClusterStatusMetric(id,min,max,bucketSize)
	if err == nil {
		indexMetricsResult["cluster_health"] = statusMetric
	}else{
		log.Error("get cluster status metric error: ", err)
	}
	clusterHealthMetricsResult := h.getShardsMetric(id, min, max, bucketSize)
	for k, v := range clusterHealthMetricsResult {
		indexMetricsResult[k] = v
	}
	return indexMetricsResult
}
func (h *APIHandler) GetClusterIndexMetrics(id string,bucketSize int, min, max int64) map[string]*common.MetricItem {
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	metricItems:=[]*common.MetricItem{}
	metricItem:=newMetricItem("index_throughput", 2, OperationGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	metricItem.AddLine("Indexing Rate","Total Indexing","Number of documents being indexed for primary and replica shards.","group1","payload.elasticsearch.index_stats.total.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Indexing Rate","Primary Indexing","Number of documents being indexed for primary shards.","group1","payload.elasticsearch.index_stats.primaries.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)

	metricItem=newMetricItem("search_throughput", 2, OperationGroupKey)
	metricItem.AddAxi("searching","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Search Rate","Total Query",
		"Number of search requests being executed across primary and replica shards. A single search can run against multiple shards!",
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

	metricItem=newMetricItem("search_latency", 3, LatencyGroupKey)
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
	return h.getSingleMetrics(metricItems, query, bucketSize)
}

func (h *APIHandler) getShardsMetric(id string, min, max int64, bucketSize int) map[string]*common.MetricItem {
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
								"value": "cluster_health",
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
			},
		},
	}
	metricItem := newMetricItem("shard_count", 7, StorageGroupKey)
	metricItem.AddAxi("counts", "group1", common.PositionLeft, "num", "0,0", "0,0.[00]", 5, false)
	metricItem.AddLine("Active Primary Shards", "Active Primary Shards", "", "group1", "payload.elasticsearch.cluster_health.active_primary_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	metricItem.AddLine("Active Shards", "Active Shards", "", "group1", "payload.elasticsearch.cluster_health.active_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	metricItem.AddLine("Relocating Shards", "Relocating Shards", "", "group1", "payload.elasticsearch.cluster_health.relocating_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	metricItem.AddLine("Initializing Shards", "Initializing Shards", "", "group1", "payload.elasticsearch.cluster_health.initializing_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	metricItem.AddLine("Unassigned Shards", "Unassigned Shards", "", "group1", "payload.elasticsearch.cluster_health.unassigned_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	metricItem.AddLine("Delayed Unassigned Shards", "Delayed Unassigned Shards", "", "group1", "payload.elasticsearch.cluster_health.delayed_unassigned_shards", "max", bucketSizeStr, "", "num", "0,0.[00]", "0,0.[00]", false, false)
	var clusterHealthMetrics []*common.MetricItem
	clusterHealthMetrics = append(clusterHealthMetrics, metricItem)
	return h.getSingleMetrics(clusterHealthMetrics, query, bucketSize)
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
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	var minDate, maxDate int64
	metricData := []interface{}{}
	metricItem:=newMetricItem("cluster_health", 1, MemoryGroupKey)
	metricItem.AddLine("status","Status","","group1","payload.elasticsearch.cluster_stats.status","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)

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
							metricData = append(metricData, map[string]interface{}{
								"x": dateTime,
								"y": count/totalCount * 100,
								"g": statusKey,
							})
						}
					}
				}
			}
		}
	}
	metricItem.Lines[0].Data = metricData
	metricItem.Lines[0].Type = common.GraphTypeBar
	return metricItem, nil
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
				"config": map[string]interface{}{
					"monitored": meta.Config.Monitored,
				},
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
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
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
		log.Error( "test_connection ", "request error: ", err)
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	b := fres.Body()
	clusterInfo := &elastic.ClusterInformation{}
	err = json.Unmarshal(b, clusterInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster info decode error: %v", err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
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
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var statusCode = fres.StatusCode()
	if statusCode == http.StatusUnauthorized {
		resBody["error"] = fmt.Sprintf("required authentication credentials")
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	healthInfo := &elastic.ClusterHealth{}
	err = json.Unmarshal(fres.Body(), &healthInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster health info decode error: %v", err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
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
			//m["indices"]=v.Indices
			m["health"]=v.Health
			m["aliases"]=v.Aliases
			//m["primary_shards"]=v.PrimaryShards
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
				"dead":v.IsDead(),
				"last_check":v.LastCheck(),
				"last_success":v.LastSuccess(),
				"failure_tickets":v.FailureTickets(),
			}
		}
		return true
	})

	h.WriteJSON(w, result, http.StatusOK)

}

func getAllMetricsIndex() string{
	return orm.GetWildcardIndexName(event.Event{})
}

func (h *APIHandler) HandleGetStorageMetricAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := util.MapStr{}
	clusterID := ps.ByName("id")
	client := elastic.GetClient(clusterID)
	shardRes, err := client.CatShards()
	if err != nil {
		resBody["error"] = fmt.Sprintf("cat shards error: %v", err)
		log.Errorf("cat shards error: %v", err)
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var metricData = TreeMapNode{
		Name:    fmt.Sprintf("%s:Storage", clusterID),
		SubKeys: map[string]int{},
	}
	for _, shardInfo := range shardRes {
		if shardInfo.ShardType != "p" {
			continue
		}
		nodeName := fmt.Sprintf("%s:%s", shardInfo.NodeIP, shardInfo.NodeName)
		//node level
		if _, ok := metricData.SubKeys[nodeName]; !ok {
			metricData.Children = append(metricData.Children, &TreeMapNode{
				Name: nodeName,
				SubKeys: map[string]int{
				},
			})
			metricData.SubKeys[nodeName] = len(metricData.Children) - 1
		}
		//index level
		nodeIdx := metricData.SubKeys[nodeName]
		if _, ok := metricData.Children[nodeIdx].SubKeys[shardInfo.Index]; !ok {
			metricData.Children[nodeIdx].Children = append(metricData.Children[nodeIdx].Children, &TreeMapNode{
				Name: shardInfo.Index,
				SubKeys: map[string]int{},
			})
			metricData.Children[nodeIdx].SubKeys[shardInfo.Index] = len(metricData.Children[nodeIdx].Children) - 1
		}
		//shard level
		indexIdx := metricData.Children[nodeIdx].SubKeys[shardInfo.Index]
		value, err := util.ConvertBytesFromString(shardInfo.Store)
		if err != nil {
			log.Warn(err)
		}
		metricData.Children[nodeIdx].Children[indexIdx].Children = append(metricData.Children[nodeIdx].Children[indexIdx].Children, &TreeMapNode{
			Name: fmt.Sprintf("shard %s", shardInfo.ShardID),
			Value: value,
		})
	}
	var (
		totalStoreSize float64 = 0
		nodeSize float64 = 0
		indexSize float64 = 0
	)
	for _, node := range metricData.Children {
		nodeSize = 0
		for _, index := range node.Children {
			indexSize = 0
			for _, shard := range index.Children {
				indexSize += shard.Value
			}
			index.Value =  math.Trunc(indexSize * 100)/100
			nodeSize += indexSize
		}
		node.Value = math.Trunc(nodeSize * 100)/100
		totalStoreSize += nodeSize
	}
	metricData.Value = math.Trunc(totalStoreSize * 100)/100
	h.WriteJSON(w, metricData, http.StatusOK)
}

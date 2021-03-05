package api

import (
	"fmt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/elastic/common"
	"math/rand"
	"net/http"
	log "github.com/cihub/seelog"
	"strings"
	"time"
)

type APIHandler struct {
	api.Handler
	Config common.ModuleConfig
}

func (h *APIHandler)Client() elastic.API {
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
	//conf.ID = id
	index:=orm.GetIndexName(elastic.ElasticsearchConfig{})
	_, err = esClient.Index(index, "", id, conf)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	//conf.ID = ir.ID

	resBody["_source"] = conf
	resBody["_id"] = id
	resBody["result"] = "created"

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

	h.WriteJSON(w, resBody,http.StatusOK)}

func (h *APIHandler) HandleDeleteClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	id := ps.ByName("id")
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	response, err := esClient.Delete(orm.GetIndexName(elastic.ElasticsearchConfig{}), "", id)

	if err != nil {
		resBody["error"] = err.Error()
		if response!=nil{
			h.WriteJSON(w, resBody, response.StatusCode)
		}else{
			h.WriteJSON(w, resBody, http.StatusInternalServerError)
		}
		return
	}

	resBody["_id"] = id
	resBody["result"] = response.Result
	h.WriteJSON(w, resBody, response.StatusCode)
}

func (h *APIHandler) HandleSearchClusterAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	var (
		name = h.GetParameterOrDefault(req, "name", "")
		enabled = h.GetParameterOrDefault(req, "enabled", "")
		queryDSL = `{"query":{"bool":{"must":[%s]}}}`
		mustBuilder = &strings.Builder{}
	)
	if name != ""{
		mustBuilder.WriteString(fmt.Sprintf(`{"match":{"name": "%s"}}`, name))
	}
	if enabled != "" {
		if enabled != "true" {
			enabled = "false"
		}
		if mustBuilder.Len() > 0 {
			mustBuilder.WriteString(",")
		}
		mustBuilder.WriteString(fmt.Sprintf(`{"match":{"enabled": %s}}`, enabled))
	}

	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String())
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	res, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.ElasticsearchConfig{}), []byte(queryDSL))

	fmt.Println(err)

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

	status:=client.GetClusterStats()

	summary:=map[string]interface{}{}
	summary["cluster_name"]=status.ClusterName
	summary["status"]=status.Status
	summary["indices_count"]=status.Indices["count"]
	summary["total_shards"]=status.Indices["shards"].(map[string]interface{})["total"]
	summary["primary_shards"]=status.Indices["shards"].(map[string]interface{})["primaries"]
	summary["replication_shards"]=status.Indices["shards"].(map[string]interface{})["replication"]
	//summary["unassigned_shards"]=status.Indices["shards"].(map[string]interface{})["primaries"]



	summary["document_count"]=status.Indices["docs"].(map[string]interface{})["count"]
	summary["deleted_document_count"]=status.Indices["docs"].(map[string]interface{})["deleted"]

	summary["used_store_bytes"]=status.Indices["store"].(map[string]interface{})["size_in_bytes"]


	summary["max_store_bytes"]=status.Nodes["fs"].(map[string]interface{})["total_in_bytes"]
	summary["available_store_bytes"]=status.Nodes["fs"].(map[string]interface{})["available_in_bytes"]


	summary["fielddata_bytes"]=status.Indices["fielddata"].(map[string]interface{})["memory_size_in_bytes"]
	summary["fielddata_evictions"]=status.Indices["fielddata"].(map[string]interface{})["evictions"]


	summary["query_cache_bytes"]=status.Indices["query_cache"].(map[string]interface{})["memory_size_in_bytes"]
	summary["query_cache_total_count"]=status.Indices["query_cache"].(map[string]interface{})["total_count"]
	summary["query_cache_hit_count"]=status.Indices["query_cache"].(map[string]interface{})["hit_count"]
	summary["query_cache_miss_count"]=status.Indices["query_cache"].(map[string]interface{})["miss_count"]
	summary["query_cache_evictions"]=status.Indices["query_cache"].(map[string]interface{})["evictions"]


	summary["segments_count"]=status.Indices["segments"].(map[string]interface{})["count"]
	summary["segments_memory_in_bytes"]=status.Indices["segments"].(map[string]interface{})["memory_in_bytes"]


	summary["nodes_count"]=status.Nodes["count"].(map[string]interface{})["total"]
	summary["version"]=status.Nodes["versions"]

	summary["mem_total_in_bytes"]=status.Nodes["os"].(map[string]interface{})["mem"].(map[string]interface{})["total_in_bytes"]
	summary["mem_used_in_bytes"]=status.Nodes["os"].(map[string]interface{})["mem"].(map[string]interface{})["used_in_bytes"]
	summary["mem_used_percent"]=status.Nodes["os"].(map[string]interface{})["mem"].(map[string]interface{})["used_percent"]


	summary["uptime"]=status.Nodes["jvm"].(map[string]interface{})["max_uptime_in_millis"]
	summary["used_jvm_bytes"]=status.Nodes["jvm"].(map[string]interface{})["mem"].(map[string]interface{})["heap_used_in_bytes"]
	summary["max_jvm_bytes"]=status.Nodes["jvm"].(map[string]interface{})["mem"].(map[string]interface{})["heap_max_in_bytes"]


	resBody["summary"] = summary


	metrics:=h.GetClusterMetrics(id)
	resBody["metrics"] = metrics

	err=h.WriteJSON(w, resBody,http.StatusOK)
	if err!=nil{
		log.Error(err)
	}



}


//TODO, use expired hash
var clusters = map[string]elastic.ElasticsearchConfig{}

func (h *APIHandler) GetClusterClient(id string) (bool,elastic.API,error) {

	config,ok:=clusters[id]
	if !ok{
		indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
		getResponse, err := h.Client().Get(indexName, "", id)
		if err != nil {
			return false, nil, err
		}

		bytes:=util.MustToJSONBytes(getResponse.Source)
		cfg:=elastic.ElasticsearchConfig{}
		err=util.FromJSONBytes(bytes,&cfg)
		if err != nil {
			return false, nil, err
		}

		if getResponse.StatusCode==http.StatusNotFound{
			return false, nil, err
		}

		cfg.ID=id
		clusters[id]=cfg
	}

	client:=common.InitClientWithConfig(config)
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

const PositionLeft = "left"
const PositionRight = "right"
const PositionTop = "top"
const PositionBottom = "bottom"

func (h *APIHandler) GetClusterMetrics(id string) map[string]MetricItem {
	result:=map[string]MetricItem{}

	metricKey:="cluster_throughput"
	metricItem:=MetricItem{}

	//axis
	metricItem.Axis=[]MetricAxis{}
	axis:=MetricAxis{}

	axis.ID=util.GetUUID()
	axis.Title="indexing"
	axis.Group="group-1"
	axis.Position=PositionLeft
	axis.FormatType="num"
	axis.LabelFormat="0,0"
	axis.TickFormat="0,0.[00]"
	axis.Ticks=5
	axis.ShowGridLines=true

	metricItem.Axis=append(metricItem.Axis,axis)

	//lines
	metricItem.Lines=[]MetricLine{}
	line:=MetricLine{}

	line.BucketSize="30 seconds"
	line.TimeRange=TimeRange{Min: 1551438000000,Max: 1551441600000}
	line.Metric=MetricSummary{
		App: "elasticsearch",
		Title: "Indexing Rate",
		Group: "group-1",
		Field: "indices_stats._all.total.indexing.index_total",
		MetricAgg: "max",
		Label: "Total Indexing",
		Description: "Number of documents being indexed for primary and replica shards.",
		Units: "e/s",
		FormatType: "num",
		Format: "0,0.[00]",
		TickFormat: "0,0.[00]",
		HasCalculation: false,
		IsDerivative: true,
	}

	data:=[][]interface{}{}

	start:=1551438000000
	for i:=0;i<120;i++{
		point:=rand.Intn(100)
		points:=[]interface{}{start,point}
		data=append(data,points)
		start+=30000
	}


	line.Data=data

	metricItem.Lines=append(metricItem.Lines,line)


	result[metricKey]=metricItem

	return result
}


type MetricAxis struct{
	ID string  `json:"id"`
	Group string  `json:"group"`
	Title string  `json:"title"`

	FormatType string  `json:"formatType"`
	Position string  `json:"position"`
	TickFormat string  `json:"tickFormat"`
	Ticks int  `json:"ticks"`
	LabelFormat string  `json:"labelFormat"`
	ShowGridLines bool  `json:"showGridLines"`
}
type TimeRange struct{
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type MetricLine struct {
	TimeRange TimeRange`json:"timeRange"`
	Data [][]interface{} `json:"data"`
	BucketSize string `json:"bucket_size"`
	Metric MetricSummary `json:"metric"`
}

type MetricSummary struct {
	App string `json:"app"`
	Group string `json:"group"`
	Title string `json:"title"`
	Label string `json:"label"`
	Description string `json:"description"`

	MetricAgg string `json:"metricAgg"`
	Field string `json:"field"`

	FormatType string `json:"formatType"`
	Format string `json:"format"`
	TickFormat string `json:"tickFormat"`
	Units string `json:"units"`

	HasCalculation bool `json:"hasCalculation"`
	IsDerivative bool `json:"isDerivative"`
}

type MetricItem struct {
	Axis []MetricAxis  `json:"axis"`
	Lines []MetricLine `json:"lines"`
}
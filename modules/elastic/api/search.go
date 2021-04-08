package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"strconv"
	"strings"
	"time"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

func (h *APIHandler) HandleCreateSearchTemplateAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	var template = &elastic.SearchTemplate{}

	err = h.DecodeJSON(req, template)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var body = map[string]interface{}{
		"script": map[string]interface{}{
			"lang": "mustache",
			"source": template.Source,
		},
	}
	bodyBytes, _ := json.Marshal(body)

	err = client.SetSearchTemplate(template.Name, bodyBytes)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	esClient := elastic.GetClient(h.Config.Elasticsearch)
	id := util.GetUUID()
	template.Created = time.Now()
	template.Updated = template.Created
	template.ClusterID = targetClusterID
	index:=orm.GetIndexName(elastic.SearchTemplate{})
	insertRes, err := esClient.Index(index, "", id, template)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	resBody["_source"] = template
	resBody["_id"] = id
	resBody["result"] = insertRes.Result

	h.WriteJSON(w, resBody,http.StatusOK)
}

func (h *APIHandler) HandleUpdateSearchTemplateAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	var template = &elastic.SearchTemplate{}

	err = h.DecodeJSON(req, template)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	templateID := ps.ByName("template_id")
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	index:=orm.GetIndexName(elastic.SearchTemplate{})
	getRes, err := esClient.Get(index, "",templateID)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if getRes.Found == false {
		resBody["error"] = fmt.Sprintf("template %s can not be found", templateID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}
	originTemplate := getRes.Source
	originName := originTemplate["name"].(string)
	if template.Name != "" && template.Name != originName {
		err = client.DeleteSearchTemplate(originName)
		if err != nil {
			resBody["error"] = err.Error()
			h.WriteJSON(w, resBody, http.StatusInternalServerError)
			return
		}
		originTemplate["name"] = template.Name
		originName = template.Name
	}
	if template.Source != "" {
		originTemplate["source"] = template.Source
	}
	var body = map[string]interface{}{
		"script": map[string]interface{}{
			"lang":   "mustache",
			"source": originTemplate["source"],
		},
	}
	bodyBytes, _ := json.Marshal(body)

	err = client.SetSearchTemplate(originName, bodyBytes)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	originTemplate["updated"] = time.Now()
	insertRes, err := esClient.Index(index, "", templateID, originTemplate)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	resBody["_source"] = originTemplate
	resBody["_id"] = templateID
	resBody["result"] = insertRes.Result

	h.WriteJSON(w, resBody,http.StatusOK)
}

func (h *APIHandler) HandleDeleteSearchTemplateAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	templateID := ps.ByName("template_id")

	index:=orm.GetIndexName(elastic.SearchTemplate{})
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	res, err := esClient.Get(index, "", templateID)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	err = client.DeleteSearchTemplate(res.Source["name"].(string))
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	delRes, err := esClient.Delete(index, "", res.ID)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	resBody["_id"] = templateID
	resBody["result"] = delRes.Result
	h.WriteJSON(w, resBody, delRes.StatusCode)

}

func (h *APIHandler) HandleSearchSearchTemplateAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
	}
	var (
		name = h.GetParameterOrDefault(req, "name", "")
		strFrom = h.GetParameterOrDefault(req, "from", "0")
		strSize = h.GetParameterOrDefault(req, "size", "20")
		queryDSL = `{"query":{"bool":{"must":[%s]}},"from": %d, "size": %d}`
		mustBuilder = &strings.Builder{}
	)
	from, _ := strconv.Atoi(strFrom)
	size, _ := strconv.Atoi(strSize)
	targetClusterID := ps.ByName("id")
	mustBuilder.WriteString(fmt.Sprintf(`{"match":{"cluster_id": "%s"}}`, targetClusterID))
	if name != ""{
		mustBuilder.WriteString(fmt.Sprintf(`,{"match":{"name": "%s"}}`, name))
	}

	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), from, size)
	esClient := elastic.GetClient(h.Config.Elasticsearch)
	res, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.SearchTemplate{}), []byte(queryDSL))

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, res, http.StatusOK)
}

func (h *APIHandler) HandleGetSearchTemplateAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{}

	id := ps.ByName("template_id")
	indexName := orm.GetIndexName(elastic.SearchTemplate{})
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
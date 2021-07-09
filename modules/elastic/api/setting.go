package api

import (
	"fmt"
	"net/http"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"time"
)

func (h *APIHandler) HandleSettingAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")

	esClient := elastic.GetClient(h.Config.Elasticsearch)
	var reqParams = elastic.Setting{
		UpdatedAt: time.Now(),
		ClusterID: targetClusterID,
	}

	err := h.DecodeJSON(req, &reqParams)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	indexName := orm.GetIndexName(reqParams)
	queryDSL := fmt.Sprintf(`{"size":1,"query":{"bool":{"must":[{"match":{"key":"%s"}},{"match":{"cluster_id":"%s"}}]}}}`, reqParams.Key, targetClusterID)
	searchRes, err := esClient.SearchWithRawQueryDSL(indexName, []byte(queryDSL))
	if len(searchRes.Hits.Hits) > 0 {
		_, err = esClient.Index(indexName, "", searchRes.Hits.Hits[0].ID, reqParams)
	}else{
		reqParams.ID = util.GetUUID()
		_, err = esClient.Index(indexName, "", reqParams.ID, reqParams)
	}


	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	resBody["acknowledged"] = true
	h.WriteJSON(w, resBody ,http.StatusOK)
}

func (h *APIHandler) HandleGetSettingAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")

	esClient := elastic.GetClient(h.Config.Elasticsearch)
	var key = ps.ByName("key")

	queryDSL := fmt.Sprintf(`{"size":1,"query":{"bool":{"must":[{"match":{"key":"%s"}},{"match":{"cluster_id":"%s"}}]}}}`, key, targetClusterID)
	searchRes, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.Setting{}), []byte(queryDSL))

	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var value interface{}
	if len(searchRes.Hits.Hits) > 0 {
		value = searchRes.Hits.Hits[0].Source["value"]
	}else{
		value = ""
	}
	h.WriteJSON(w, value ,http.StatusOK)
}

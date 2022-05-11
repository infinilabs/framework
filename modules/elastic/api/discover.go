package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
)

func (h *APIHandler) HandleEseSearchAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}

	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	var reqParams = struct{
		Index string `json:"index"`
		Body map[string]interface{} `json:"body"`
		DistinctByField map[string]interface{} `json:"distinct_by_field"`
	}{}

	err = h.DecodeJSON(req, &reqParams)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if _, ok := reqParams.Body["track_total_hits"]; ok {
		vr, _ := util.VersionCompare(client.GetVersion(), "7.0")
		if vr < 0 {
			delete(reqParams.Body, "track_total_hits")
		}
	}
	if reqParams.DistinctByField != nil {
		if query, ok := reqParams.Body["query"]; ok {
			if qm, ok := query.(map[string]interface{}); ok {

				filter, _ := util.MapStr(qm).GetValue("bool.filter")
				if fv, ok := filter.([]interface{}); ok{
					fv = append(fv, util.MapStr{
						"script": util.MapStr{
							"script": util.MapStr{
								"source": "distinct_by_field",
								"lang": "infini",
								"params": reqParams.DistinctByField,
							},
						},
					})
					util.MapStr(qm).Put("bool.filter", fv)
				}

			}
		}
	}
	vr, err := util.VersionCompare(client.GetVersion(), "7.2")
	if err != nil {
		resBody["error"] = fmt.Sprintf("version compare error: %v",err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if vr < 0 {
		if aggs, ok := reqParams.Body["aggs"]; ok {
			if maggs, ok := aggs.(map[string]interface{}); ok {
				if aggsCounts, ok := maggs["counts"].(map[string]interface{}); ok {
					if aggVals, ok := aggsCounts["date_histogram"].(map[string]interface{}); ok {
						var interval interface{}
						if calendarInterval, ok := aggVals["calendar_interval"]; ok {
							interval = calendarInterval
							delete(aggVals, "calendar_interval")
						}
						if fixedInterval, ok := aggVals["fixed_interval"]; ok {
							interval = fixedInterval
							delete(aggVals, "fixed_interval")
						}
						aggVals["interval"] = interval
					}
				}
			}
		}
	}

	reqDSL, _ := json.Marshal(reqParams.Body)

	searchRes, err := client.SearchWithRawQueryDSL(reqParams.Index, reqDSL)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, searchRes,http.StatusOK)
}


func (h *APIHandler) HandleValueSuggestionAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	var reqParams = struct{
		BoolFilter interface{} `json:"boolFilter"`
		FieldName string `json:"field"`
		Query string `json:"query"`
	}{}

	err = h.DecodeJSON(req, &reqParams)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	indexName := ps.ByName("index")
	var queryBody = `{"size": 0,"query": {"bool": {"filter": %s}},"aggs": {"suggestions": {
        "terms": {
          "field": "%s",
          "include": "%s.*",
          "execution_hint": "map",
          "shard_size": 10
        }
      }
    }
  }`
	byteFilters, _ := json.Marshal(reqParams.BoolFilter)

	reqDSL := fmt.Sprintf(queryBody, string(byteFilters), reqParams.FieldName, reqParams.Query)

	searchRes, err := client.SearchWithRawQueryDSL(indexName, []byte(reqDSL))
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var values = []interface{}{}
	for _, bucket := range searchRes.Aggregations["suggestions"].Buckets {
		values = append(values, bucket["key"])
	}
	h.WriteJSON(w, values,http.StatusOK)
}

func (h *APIHandler) HandleTraceIDSearchAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := util.MapStr{}
	traceID := h.GetParameterOrDefault(req, "traceID", "")
	traceIndex := h.GetParameterOrDefault(req, "traceIndex", orm.GetIndexName(elastic.TraceMeta{}))
	traceField := h.GetParameterOrDefault(req, "traceField", "trace_id")
	targetClusterID := ps.ByName("id")
	exists,client,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}
	var queryDSL = util.MapStr{
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							traceField: traceID,
						},
					},
					{
						"term": util.MapStr{
							"cluster_id": targetClusterID,
						},
					},
				},
			},
		},
	}
	searchRes, err := client.SearchWithRawQueryDSL(traceIndex, util.MustToJSONBytes(queryDSL))
	if err != nil {
		log.Error(err)
		h.WriteJSON(w, util.MapStr{
			"error": err,
		}, http.StatusInternalServerError)
		return
	}
	if searchRes.GetTotal() == 0 {
		h.WriteJSON(w, []string{}, http.StatusOK)
		return
	}
	var indexNames []string
	for _, hit := range searchRes.Hits.Hits {
		indexNames = append(indexNames, hit.Source["index"].(string))
	}
	h.WriteJSON(w, indexNames, http.StatusOK)
}


package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	httprouter "infini.sh/framework/core/api/router"
)

func (h *APIHandler) HandleEseSearchAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
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

	var reqParams = struct{
		Index string `json:"index"`
		Body map[string]interface{} `json:"body"`
	}{}

	err = h.DecodeJSON(req, &reqParams)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	if client.ClusterVersion() < "7.2" {
		if aggs, ok := reqParams.Body["aggs"]; ok {
			if maggs, ok := aggs.(map[string]interface{}); ok {
				if aggs2, ok := maggs["2"].(map[string]interface{}); ok {
					if aggVals, ok := aggs2["date_histogram"].(map[string]interface{}); ok {
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
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
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

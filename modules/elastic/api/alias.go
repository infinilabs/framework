package api

import (
	"github.com/segmentio/encoding/json"
	"fmt"
	"net/http"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
)

func (h *APIHandler) HandleAliasAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
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

	var aliasReq = &elastic.AliasRequest{}

	err = h.DecodeJSON(req, aliasReq)
	if err != nil {
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	bodyBytes, _ := json.Marshal(aliasReq)

	err = client.Alias(bodyBytes)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	resBody["acknowledged"] = true

	h.WriteJSON(w, resBody,http.StatusOK)
}

func (h *APIHandler) HandleGetAliasAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")
	exists, client, err := h.GetClusterClient(targetClusterID)

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists {
		resBody["error"] = fmt.Sprintf("cluster [%s] not found", targetClusterID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}
	res, err := client.GetAliasesDetail()
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, res, http.StatusOK)
}
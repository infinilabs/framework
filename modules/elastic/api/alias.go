package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"net/http"
)

func (h *APIHandler) HandleAliasAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody := map[string] interface{}{
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

	var aliasReq = &elastic.AliasRequest{}

	err = h.DecodeJSON(req, aliasReq)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	esVersion := elastic.GetMetadata(targetClusterID).Config.Version
	if r, _ := util.VersionCompare(esVersion, "6.4"); r == -1 {
		for i := range aliasReq.Actions {
			for k, v := range aliasReq.Actions[i] {
				if v.IsWriteIndex {
					v.IsWriteIndex = false
					aliasReq.Actions[i][k] = v
					log.Warnf("elasticsearch aliases api of version [%s] not supports parameter is_write_index", esVersion)
				}
			}
		}
	}

	bodyBytes, _ := json.Marshal(aliasReq)

	err = client.Alias(bodyBytes)
	if err != nil {
		log.Error(err)
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
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists {
		resBody["error"] = fmt.Sprintf("cluster [%s] not found", targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}
	res, err := client.GetAliasesDetail()
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, res, http.StatusOK)
}
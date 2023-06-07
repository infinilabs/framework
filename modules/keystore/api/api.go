/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/util"
	"net/http"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) setKeystoreValue(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	reqBody := struct {
		Key string `json:"key"`
		Value string `json:"value"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if reqBody.Key == "" {
		h.WriteError(w, "key cannot be empty", http.StatusInternalServerError)
		return
	}
	ks, err := keystore.GetWriteableKeystore()
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = ks.Store(reqBody.Key, util.UnsafeStringToBytes(reqBody.Value))
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = ks.Save()
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"success": true,
	}, http.StatusOK)
}
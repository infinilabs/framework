/* Copyright © INFINI Ltd. All rights reserved.
* Web: https://infinilabs.com
* Email: hello#infini.ltd */

package filter

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/filter/nutsdb"
	"net/http"
)

type APIHandler struct {
	api.Handler
	DB *nutsdb.NutsdbKVFilter
}

func (h *APIHandler) merge(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	err := h.DB.Merge()
	if err != nil {
		h.WriteError(w, err.Error(),http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"success": true,
	},http.StatusOK)
}

func Init(db  *nutsdb.NutsdbKVFilter){
	handler := APIHandler{
		DB: db,
	}
	api.HandleAPIMethod(api.GET, "/_framework/api/kv/_merge", handler.merge)
}
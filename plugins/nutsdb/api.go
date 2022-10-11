/* Copyright © INFINI Ltd. All rights reserved.
* Web: https://infinilabs.com
* Email: hello#infini.ltd */

package nutsdb

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
)

type APIHandler struct {
	api.Handler
	DB *NutsdbFilter
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

func Init(db  *NutsdbFilter){
	handler := APIHandler{
		DB: db,
	}
	api.HandleAPIMethod(api.GET, "/_framework/api/nutsdb/_merge", handler.merge)
}
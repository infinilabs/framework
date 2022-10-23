/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
)

func (h APIHandler) ListPermission(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	typ := ps.MustGetParameter("type")
	err := rbac.IsAllowRoleType(typ)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	var permissions interface{}
	if typ == rbac.Elasticsearch {
		permissions = rbac.GetPermissions(typ)
	}
	h.WriteOKJSON(w, permissions)
	return
}

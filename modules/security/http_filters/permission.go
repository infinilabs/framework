/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http_filters

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	common "infini.sh/framework/core/api/common"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"net/http"
)

func init() {
	api.RegisterUIFilter(&PermissionFilter{})
}

type PermissionFilter struct {
	api.Handler
}

func (f *PermissionFilter) GetPriority() int {
	return 500
}

func (f *PermissionFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {

	if options == nil || options.RequirePermission == nil || len(options.RequirePermission) == 0 || !common.IsAuthEnable() {
		log.Debug(method, ",", pattern, ",skip permission check")
		return next
	}

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		reqUser, err := security.GetUserFromContext(r.Context())
		if reqUser == nil || err != nil {
			o := api.PrepareErrorJson("invalid login", 401)
			f.WriteJSON(w, o, 401)
			return
		}

		//bypass admin
		if reqUser.Roles != nil && util.AnyInArrayEquals(reqUser.Roles, security.RoleAdmin) {
			next(w, r, ps)
			return
		}

		if reqUser.UserAssignedPermission == nil || reqUser.UserAssignedPermission.NeedRefresh() {
			reqUser.UserAssignedPermission = security.GetUserPermissions(reqUser)
		}

		if reqUser.UserAssignedPermission == nil || options.RequirePermission == nil || len(options.RequirePermission) == 0 {
			panic("invalid permission state")
		}

		if global.Env().IsDebug {
			log.Tracef("perm key: %v", options.RequirePermission)
		}

		if reqUser.UserAssignedPermission.Validate(security.MustRegisterPermissionByKeys(options.RequirePermission)) {
			next(w, r, ps)
		} else {
			f.WriteErrorObject(w, errors.Errorf("permission [%v] not allowed", options.RequirePermission), 403)
		}
	}
}

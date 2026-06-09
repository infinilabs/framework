/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package filters

import (
	"infini.sh/framework/core/global"
	"net/http"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	common "infini.sh/framework/core/api/common"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

func init() {
	api.RegisterUIFilter(&AuthFilter{})
}

type AuthFilter struct {
	api.Handler
}

func (f *AuthFilter) GetPriority() int {
	return 200
}

func (f *AuthFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {
	// skip if auth not required on this route, or auth is globally disabled
	if options == nil || (!options.RequireLogin && !options.OptionLogin) || !common.IsAuthEnable() {
		log.Debug(method, ",", pattern, ",skip auth")
		return next
	}

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		claims, err := security.ValidateLogin(w, r)

		if global.Env().IsDebug {
			log.Debug(method, ",", pattern, ",", util.MustToJSON(claims), ",", err)
		}

		if claims != nil && claims.IsValid() {
			// Run request context resolvers (e.g. read X-Team-ID/X-Project-ID headers).
			if resolverErr := security.RunRequestContextResolvers(r.Context(), r, claims); resolverErr != nil {
				log.Warn("request context resolver error: ", resolverErr)
			}

			//proactive update permission on auth check
			if claims.UserAssignedPermission == nil || claims.UserAssignedPermission.NeedRefresh() {
				claims.UserAssignedPermission = security.GetUserPermissions(claims)
			}

			r = r.WithContext(security.AddUserToContext(r.Context(), claims))
		}

		if !options.OptionLogin {
			if claims == nil {
				o := api.PrepareErrorJson("invalid login", 401)
				f.WriteJSON(w, o, 401)
				return
			}

			if err != nil {
				f.WriteErrorObject(w, err, 401)
				return
			}
		}

		next(w, r, ps)
	}
}

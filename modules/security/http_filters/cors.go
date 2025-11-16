/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http_filters

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/security"
	"net/http"
)

func init() {
	api.RegisterUIFilter(&CORSFilter{})
}

type CORSFilter struct {
	api.Handler
}

func (f *CORSFilter) GetPriority() int {
	return 100
}

const FeatureCORS = "feature_cors"
const FeatureNotAllowCredentials = "feature_not_allow_credentials"
const FeatureByPassCORSCheck = "feature_bypass_cors_check"

func (f *CORSFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {

	//option not enabled
	if options == nil || !options.Feature(FeatureCORS) {
		log.Debug(method, ",", pattern, ",skip feature ", FeatureCORS)
		return next
	}

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		origin := r.Header.Get("Origin")
		if options.Feature(FeatureByPassCORSCheck) || (origin != "" && (r.Method == http.MethodOptions || security.IsAllowedOrigin(origin, r))) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-TOKEN, APP-INTEGRATION-ID, WEBSOCKET-SESSION-ID")
			if options.Feature(FeatureNotAllowCredentials) {
				w.Header().Set("Access-Control-Allow-Credentials", "false")
			} else {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// Handle preflight (OPTIONS) requests
			if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
				// Respond with 200 OK for OPTIONS requests
				w.WriteHeader(http.StatusOK)
				return
			}
		} else {
			if global.Env().IsDebug {
				log.Debug("skipping place CORS headers: ", method, ",", pattern, ",origin:", origin, ",", origin != "", ",", r.Method == http.MethodOptions, ",", security.IsAllowedOrigin(origin, r))
			}
		}

		next(w, r, ps)
	}
}

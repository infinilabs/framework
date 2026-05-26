/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http_filters

import (
	"net/http"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

func init() {
	api.RegisterUIFilter(&SecurityFilter{})
}

type SecurityFilter struct {
	api.Handler
}

func (f *SecurityFilter) GetPriority() int {
	return 450
}

func (f *SecurityFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {
	if options == nil || (!options.Feature(api.FeatureRequireSecureTransport) && !options.Feature(api.FeatureRequireReplayProtection)) {
		log.Debug(method, ",", pattern, ",skip security feature filters")
		return next
	}

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if options.Feature(api.FeatureRequireSecureTransport) {
			secureOptions := api.SecureTransportOptions{
				TrustForwardHeaders: trustForwardHeadersFromOptions(options),
			}
			if !api.RequestUsesSecureTransport(r, secureOptions) {
				f.WriteError(w, "sensitive endpoints require HTTPS or a trusted HTTPS reverse proxy", http.StatusUpgradeRequired)
				return
			}
		}

		if options.Feature(api.FeatureRequireReplayProtection) {
			if err := replaysecurity.ValidateAndConsumeReplayNonce(r); err != nil {
				f.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}
		}

		next(w, r, ps)
	}
}

func trustForwardHeadersFromOptions(options *api.HandlerOptions) bool {
	if options == nil || options.Labels == nil {
		return false
	}
	trustValue, ok := options.Labels[api.LabelTrustForwardHeaders]
	if !ok {
		return false
	}
	trustForwardHeaders, ok := trustValue.(bool)
	return ok && trustForwardHeaders
}

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package account

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/security/http_filters"
	"net/http"
)

func init() {
	api.HandleUIMethod(api.GET, "/account/logout", Logout, api.OptionLogin(), api.Feature(http_filters.FeatureCORS))
	api.HandleUIMethod(api.POST, "/account/logout", Logout, api.OptionLogin(), api.Feature(http_filters.FeatureCORS))
	api.HandleUIMethod(api.OPTIONS, "/account/logout", Logout, api.OptionLogin(), api.Feature(http_filters.FeatureCORS))
}

func  Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	api.DestroySession(w, r)
	api.WriteOKJSON(w, util.MapStr{
		"status": "ok",
	})
}

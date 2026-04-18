/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package account

import (
	"infini.sh/coco/core"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
)

func init() {
	api.HandleUIMethod(api.GET, "/account/logout", Logout, api.OptionLogin(), api.Feature(core.FeatureCORS))
	api.HandleUIMethod(api.POST, "/account/logout", Logout, api.OptionLogin(), api.Feature(core.FeatureCORS))
	api.HandleUIMethod(api.OPTIONS, "/account/logout", Logout, api.OptionLogin(), api.Feature(core.FeatureCORS))
}

func  Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	api.DestroySession(w, r)
	api.WriteOKJSON(w, util.MapStr{
		"status": "ok",
	})
}
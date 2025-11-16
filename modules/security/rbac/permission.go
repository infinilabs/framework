/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"net/http"
)

func ListPermission(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	items := security.GetPermissionItems()
	api.WriteJSON(w, items, 200)
}

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
)

var ReadPermissionLists = GetOrInitPermission("generic", "security:permission", Read)

func init() {
	api.HandleUIMethod(api.GET, "/security/permission/", ListPermission, api.RequirePermission(ReadPermissionLists))
}

func ListPermission(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	items := GetPermissionItems()
	api.WriteJSON(w, items, 200)
}

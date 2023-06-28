/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package oauth

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
)

type APIHandler struct {
	api.Handler
	rbac.Adapter
}


const adapterType = "native"

var apiHandler = APIHandler{Adapter: rbac.GetAdapter(adapterType)} //TODO handle hard coded


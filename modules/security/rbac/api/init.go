/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/security/rbac"
	"infini.sh/framework/modules/elastic/common"
)

type APIHandler struct {
	api.Handler
	Config common.ModuleConfig
	rbac.Adapter
}

const adapterType = "elasticsearch"
var RoleMap = make(map[string]rbac.Role)
func Init(cfg common.ModuleConfig) {
	apiHandler := APIHandler{Config: cfg, Adapter: rbac.GetAdapter(adapterType)}
}
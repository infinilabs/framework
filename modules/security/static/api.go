/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package staticauth

import (
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

type APIHandler struct {
	api.Handler
}

func init() {
	// 静态账号回显与用户管理同权限面（generic#security:user/read）；
	// 角色回显同理。端点常驻: 未启用 static 认证时返回空列表。
	userRead := api.RequirePermission(security.GetOrInitPermission("generic", "security:user", security.Read))
	roleRead := api.RequirePermission(security.GetOrInitPermission("generic", "security:role", security.Read))

	handler := APIHandler{}
	api.HandleUIMethod(api.GET, "/security/static-users", handler.staticUsers, userRead)
	api.HandleUIMethod(api.GET, "/security/static-users/", handler.staticUsers, userRead)
	api.HandleUIMethod(api.GET, "/security/static-roles", handler.staticRoles, roleRead)
	api.HandleUIMethod(api.GET, "/security/static-roles/", handler.staticRoles, roleRead)
}

// GET /security/static-users — yml 静态账号只读回显。
// ListAccounts 返回脱敏副本，不含密码哈希。
func (h *APIHandler) staticUsers(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	accounts := ListAccounts()
	result := make([]util.MapStr, 0, len(accounts))
	for _, a := range accounts {
		result = append(result, util.MapStr{
			"id": a.ID, "name": a.Name, "email": a.Email,
			"roles": a.Roles,
		})
	}
	h.WriteJSON(w, util.MapStr{"count": len(result), "result": result}, http.StatusOK)
}

// GET /security/static-roles — yml 静态角色只读回显
// （authorization.static.roles；grantAll 即 permissions: ["*"]）。
func (h *APIHandler) staticRoles(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	defs := ListRoleDefinitions()
	result := make([]util.MapStr, 0, len(defs))
	for _, d := range defs {
		perms := make([]string, 0, len(d.Permissions))
		for _, p := range d.Permissions {
			perms = append(perms, string(p))
		}
		result = append(result, util.MapStr{
			"name": d.Name, "grant_all": d.GrantAll, "permissions": perms,
		})
	}
	h.WriteJSON(w, util.MapStr{"count": len(result), "result": result}, http.StatusOK)
}

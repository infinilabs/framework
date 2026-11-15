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
	// Static account listing shares the user-management permission surface
	// (generic#security:user/read); role listing is analogous. The
	// endpoints are always registered: they return empty lists when static
	// auth is not enabled.
	userRead := api.RequirePermission(security.GetOrInitPermission("generic", "security:user", security.Read))
	roleRead := api.RequirePermission(security.GetOrInitPermission("generic", "security:role", security.Read))

	handler := APIHandler{}
	api.HandleUIMethod(api.GET, "/security/static-users", handler.staticUsers, userRead)
	api.HandleUIMethod(api.GET, "/security/static-users/", handler.staticUsers, userRead)
	api.HandleUIMethod(api.GET, "/security/static-roles", handler.staticRoles, roleRead)
	api.HandleUIMethod(api.GET, "/security/static-roles/", handler.staticRoles, roleRead)
}

// GET /security/static-users — read-only echo of the yml static accounts.
// ListAccounts returns sanitized copies without the bcrypt password hashes.
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

// GET /security/static-roles — read-only echo of the yml static roles
// (authorization.static.roles; grantAll means permissions: ["*"]).
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

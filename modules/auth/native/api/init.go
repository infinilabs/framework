/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/api/rbac/enum"
	_ "infini.sh/framework/modules/auth/native"
)

type APIHandler struct {
	api.Handler
	rbac.Adapter
}

const adapterType = "elasticsearch"

func Init() {

	apiHandler := APIHandler{Adapter: rbac.GetAdapter(adapterType)}

	api.HandleAPIMethod(api.GET, "/permission/:type", apiHandler.ListPermission)
	api.HandleAPIMethod(api.POST, "/role/:type", apiHandler.RequirePermission(apiHandler.CreateRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.GET, "/role/:id", apiHandler.RequirePermission(apiHandler.GetRole, enum.RoleReadPermission...))
	api.HandleAPIMethod(api.DELETE, "/role/:id", apiHandler.RequirePermission(apiHandler.DeleteRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.PUT, "/role/:id", apiHandler.RequirePermission(apiHandler.UpdateRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.GET, "/role/_search", apiHandler.RequirePermission(apiHandler.SearchRole, enum.RoleReadPermission...))

	api.HandleAPIMethod(api.POST, "/user", apiHandler.RequirePermission(apiHandler.CreateUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.GET, "/user/:id", apiHandler.RequirePermission(apiHandler.GetUser, enum.UserReadPermission...))
	api.HandleAPIMethod(api.DELETE, "/user/:id", apiHandler.RequirePermission(apiHandler.DeleteUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.PUT, "/user/:id", apiHandler.RequirePermission(apiHandler.UpdateUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.GET, "/user/_search", apiHandler.RequirePermission(apiHandler.SearchUser, enum.UserReadPermission...))
	api.HandleAPIMethod(api.PUT, "/user/:id/password", apiHandler.RequirePermission(apiHandler.UpdateUserPassword, enum.UserAllPermission...))

	api.HandleAPIMethod(api.POST, "/account/login", apiHandler.Login)

	api.HandleAPIMethod(api.GET, "/account/current_user", apiHandler.RequireLogin(apiHandler.CurrentUser))

	api.HandleAPIMethod(api.DELETE, "/account/logout", apiHandler.Logout)
	api.HandleAPIMethod(api.GET, "/account/profile", apiHandler.RequireLogin(apiHandler.Profile))
	api.HandleAPIMethod(api.PUT, "/account/password", apiHandler.RequireLogin(apiHandler.UpdatePassword))
	api.HandleAPIMethod(api.GET, "/user/admin_disabled", apiHandler.RequirePermission(apiHandler.SetBuiltinUserAdminDisabled, enum.PermissionDisableBuiltinAdmin))

	api.HandleAPIMethod(api.GET, "/user/security/settings", apiHandler.RequireLogin(apiHandler.GetSecuritySettings))

	//load role from store
	apiHandler.loadRolePermission()
}

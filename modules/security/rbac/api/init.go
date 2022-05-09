/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/security/rbac"
	"infini.sh/framework/core/security/rbac/enum"
	m "infini.sh/framework/core/security/rbac/middleware"
	_ "infini.sh/framework/modules/security/rbac/adapter/elasticsearch"
)

type APIHandler struct {
	api.Handler
	rbac.Adapter
}

const adapterType = "elasticsearch"
func Init() {
	apiHandler := APIHandler{Adapter: rbac.GetAdapter(adapterType)}
	api.HandleAPIMethod(api.GET, "/permission/:type", apiHandler.ListPermission)
	api.HandleAPIMethod(api.POST, "/role/:type", m.PermissionRequired(apiHandler.CreateRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.GET, "/role/:id", m.PermissionRequired(apiHandler.GetRole, enum.RoleReadPermission...))
	api.HandleAPIMethod(api.DELETE, "/role/:id", m.PermissionRequired(apiHandler.DeleteRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.PUT, "/role/:id", m.PermissionRequired(apiHandler.UpdateRole, enum.RoleAllPermission...))
	api.HandleAPIMethod(api.GET, "/role/_search", m.PermissionRequired(apiHandler.SearchRole, enum.RoleReadPermission...))

	api.HandleAPIMethod(api.POST, "/user", m.PermissionRequired(apiHandler.CreateUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.GET, "/user/:id", m.PermissionRequired(apiHandler.GetUser, enum.UserReadPermission...))
	api.HandleAPIMethod(api.DELETE, "/user/:id", m.PermissionRequired(apiHandler.DeleteUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.PUT, "/user/:id", m.PermissionRequired(apiHandler.UpdateUser, enum.UserAllPermission...))
	api.HandleAPIMethod(api.GET, "/user/_search", m.PermissionRequired(apiHandler.SearchUser, enum.UserReadPermission...))
	api.HandleAPIMethod(api.PUT, "/user/:id/password", m.PermissionRequired(apiHandler.UpdateUserPassword, enum.UserAllPermission...))

	api.HandleAPIMethod(api.POST, "/account/login", apiHandler.Login)

	api.HandleAPIMethod(api.GET, "/account/current_user", apiHandler.CurrentUser)

	api.HandleAPIMethod(api.DELETE, "/account/logout", apiHandler.Logout)
	api.HandleAPIMethod(api.GET, "/account/profile", m.LoginRequired(apiHandler.Profile))
	api.HandleAPIMethod(api.PUT, "/account/password", m.LoginRequired(apiHandler.UpdatePassword))

	//load role from store
	apiHandler.loadRolePermission()
}

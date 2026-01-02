/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/entity_card"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/modules/security/http_filters"
)

func Init() {

	provider := SecurityBackendProvider{}
	security.RegisterAuthenticationProvider(security.DefaultNativeAuthBackend, &provider)
	security.RegisterAuthorizationProvider(security.DefaultNativeAuthBackend, &provider)

	orm.MustRegisterSchemaWithIndexName(&security.UserAccount{}, "app-users")
	orm.MustRegisterSchemaWithIndexName(&security.UserRole{}, "app-roles")
	ReadPermissionLists := security.GetSimplePermission("generic", "security:permission", security.Read)

	CreateRolePermission := security.GetSimplePermission("generic", "security:role", security.Create)
	UpdateRolePermission := security.GetSimplePermission("generic", "security:role", security.Update)
	DeleteRolePermission := security.GetSimplePermission("generic", "security:role", security.Delete)
	ReadRolePermission := security.GetSimplePermission("generic", "security:role", security.Read)
	SearchRolePermission := security.GetSimplePermission("generic", "security:role", security.Search)

	CreateUserPermission := security.GetSimplePermission("generic", "security:user", security.Create)
	UpdateUserPermission := security.GetSimplePermission("generic", "security:user", security.Update)
	DeleteUserPermission := security.GetSimplePermission("generic", "security:user", security.Delete)
	ReadUserPermission := security.GetSimplePermission("generic", "security:user", security.Read)
	SearchUserPermission := security.GetSimplePermission("generic", "security:user", security.Search)

	SearchPrincipalPermission := security.GetSimplePermission("generic", "security:principal", security.Search)

	security.RegisterPermissionsToRole(security.RoleAdmin, ReadPermissionLists,
		CreateRolePermission, UpdateRolePermission, DeleteRolePermission, ReadRolePermission, SearchRolePermission,
		CreateUserPermission, UpdateUserPermission, DeleteUserPermission, ReadUserPermission, SearchUserPermission,
		SearchPrincipalPermission)

	api.HandleUIMethod(api.GET, "/security/permission/", ListPermission, api.RequirePermission(ReadPermissionLists))

	api.HandleUIMethod(api.POST, "/security/role/", CreateRole, api.RequirePermission(CreateRolePermission))
	api.HandleUIMethod(api.GET, "/security/role/_search", SearchRole, api.RequirePermission(SearchRolePermission))
	api.HandleUIMethod(api.PUT, "/security/role/:id", UpdateRole, api.RequirePermission(UpdateRolePermission))
	api.HandleUIMethod(api.DELETE, "/security/role/:id", DeleteRole, api.RequirePermission(DeleteRolePermission))
	api.HandleUIMethod(api.GET, "/security/role/:id", GetRole, api.RequirePermission(ReadRolePermission))

	if !global.Env().SystemConfig.WebAppConfig.Security.Managed {

		entity := UserEntityProvider{}
		entity_card.RegisterEntityProvider("user", &entity)

		api.HandleUIMethod(api.POST, "/security/user/", CreateUser, api.RequirePermission(CreateUserPermission))
		api.HandleUIMethod(api.GET, "/security/user/_search", SearchUser, api.RequirePermission(SearchUserPermission), api.Feature(http_filters.FeatureMaskSensitiveField))
		api.HandleUIMethod(api.PUT, "/security/user/:id", UpdateUser, api.RequirePermission(UpdateUserPermission))
		api.HandleUIMethod(api.DELETE, "/security/user/:id", DeleteUser, api.RequirePermission(DeleteUserPermission))
		api.HandleUIMethod(api.GET, "/security/user/:id", GetUser, api.RequirePermission(ReadUserPermission), api.Feature(http_filters.FeatureMaskSensitiveField))

		//search users or teams
		api.HandleUIMethod(api.GET, "/security/principal/_search", SearchPrincipals, api.RequirePermission(SearchPrincipalPermission))
	}

}

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package native

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

	RegisterPublicUIAuthRoutes()

	orm.MustRegisterSchemaWithIndexName(&security.UserAccount{}, "app-users")
	orm.MustRegisterSchemaWithIndexName(&security.UserRole{}, "app-roles")

	CreateRolePermission := security.GetOrInitPermission("generic", "security:role", security.Create)
	UpdateRolePermission := security.GetOrInitPermission("generic", "security:role", security.Update)
	DeleteRolePermission := security.GetOrInitPermission("generic", "security:role", security.Delete)
	ReadRolePermission := security.GetOrInitPermission("generic", "security:role", security.Read)
	SearchRolePermission := security.GetOrInitPermission("generic", "security:role", security.Search)

	CreateUserPermission := security.GetOrInitPermission("generic", "security:user", security.Create)
	UpdateUserPermission := security.GetOrInitPermission("generic", "security:user", security.Update)
	DeleteUserPermission := security.GetOrInitPermission("generic", "security:user", security.Delete)
	ReadUserPermission := security.GetOrInitPermission("generic", "security:user", security.Read)
	SearchUserPermission := security.GetOrInitPermission("generic", "security:user", security.Search)

	SearchPrincipalPermission := security.GetOrInitPermission("generic", "security:principal", security.Search)

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

func RegisterPublicUIAuthRoutes() {
	secureViaProxy := api.SecureTransportOptions{TrustForwardHeaders: true}
	api.HandleUIMethod(api.POST, "/account/replay_nonce",
		api.RequireSecureTransport(IssueReplayNonce, secureViaProxy),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))

	api.HandleUIMethod(api.POST, "/account/login/challenge",
		api.RequireSecureTransport(LoginChallenge, secureViaProxy),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))

	api.HandleUIMethod(api.POST, "/account/login",
		api.RequireSecureTransport(Login, secureViaProxy),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))
}

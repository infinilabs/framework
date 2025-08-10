package security

import "context"

type PermissionScope string

const PermissionScopePlatform PermissionScope = "permission_scope_platform"
const PermissionScopeTenant PermissionScope = "permission_scope_tenant"
const PermissionScopeGroup PermissionScope = "permission_scope_group"
const PermissionScopeProject PermissionScope = "permission_scope_project"
const PermissionScopeUser PermissionScope = "permission_scope_user"
const PermissionScopePublic PermissionScope = "permission_scope_public"

func MustGetPermissionCheckingScopeFromContext(ctx context.Context) *UserSessionInfo {
	user, err := GetUserFromContext(ctx)
	if user == nil || err != nil {
		panic(err)
	}
	return user
}

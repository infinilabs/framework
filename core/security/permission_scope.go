package security

import "context"

const PermissionScopePublic int = 1
const PermissionScopePlatform int = 2
const PermissionScopeTenant int = 3
const PermissionScopeGroup int = 4
const PermissionScopeProject int = 5
const PermissionScopeUser int = 6

const PermissionScopeDefault int = 10000

func MustGetPermissionCheckingScopeFromContext(ctx context.Context) *UserSessionInfo {
	user, err := GetUserFromContext(ctx)
	if user == nil || err != nil {
		panic(err)
	}
	return user
}

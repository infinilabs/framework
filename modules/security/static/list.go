/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package staticauth

import (
	"sort"

	"infini.sh/framework/core/security"
)

// RoleDefinition is a read-only view of one statically configured role
// (authorization.static.roles): either grant_all, or an explicit permission
// list.
type RoleDefinition struct {
	Name        string
	GrantAll    bool
	Permissions []security.PermissionKey
}

// ListAccounts returns sanitized copies of the statically configured
// accounts (authentication.static.users). The bcrypt password hash is
// cleared, so the result is safe to echo back to clients. Ordered by ID for
// stable output.
func ListAccounts() []security.UserAccount {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]security.UserAccount, 0, len(authUsersByID))
	for _, account := range authUsersByID {
		sanitized := *account
		sanitized.Password = ""
		out = append(out, sanitized)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ListRoleDefinitions returns the statically configured role definitions,
// name-sorted for stable output. Permissions are copied so callers cannot
// mutate the live registry.
func ListRoleDefinitions() []RoleDefinition {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]RoleDefinition, 0, len(roles))
	for name, def := range roles {
		out = append(out, RoleDefinition{
			Name:        name,
			GrantAll:    def.grantAll,
			Permissions: append([]security.PermissionKey(nil), def.permissions...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

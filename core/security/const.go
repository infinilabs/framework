// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package security

import (
	"sync"

	"infini.sh/framework/core/api"
	"infini.sh/framework/core/util"
)

// PermissionID is re-exported from api package for backward compatibility
type PermissionID = api.PermissionID

// PermissionKey is re-exported from api package for backward compatibility
type PermissionKey = api.PermissionKey

const RoleReadOnly = "readonly"
const RoleAdmin = "admin"

const DefaultNativeAuthBackend = "default_native_auth_backend"

// adminRoles holds the set of role names considered as admin.
// "admin" is always included. Applications can register additional admin roles.
var adminRoles = []string{RoleAdmin}
var adminRolesMu sync.Mutex

// RegisterAdminRole adds a role name to the admin role set.
// Call this during init to register application-specific admin roles (e.g. "Administrator").
func RegisterAdminRole(role string) {
	adminRolesMu.Lock()
	defer adminRolesMu.Unlock()
	for _, r := range adminRoles {
		if r == role {
			return
		}
	}
	adminRoles = append(adminRoles, role)
}

// IsAdminUser returns true if any of the user's roles is an admin role.
func IsAdminUser(roles []string) bool {
	adminRolesMu.Lock()
	defer adminRolesMu.Unlock()
	for _, ar := range adminRoles {
		if util.ContainsAnyInArray(ar, roles) {
			return true
		}
	}
	return false
}

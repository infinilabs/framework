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
	"infini.sh/framework/core/errors"
	"sync"
)

// RoleRegistry manages roles and their associated permissions
type RoleRegistry struct {
	roleMap     map[string]map[PermissionKey]struct{} // Role -> Set of permissions
	roleVersion map[string]int32
	locker      sync.RWMutex
}

func (rr *RoleRegistry) IncrementVersion(role string) {
	rr.locker.Lock()
	defer rr.locker.Unlock()

	rr.roleVersion[role]++
}

func (rr *RoleRegistry) GetVersion(role string) int32 {
	rr.locker.RLock()
	defer rr.locker.RUnlock()

	return rr.roleVersion[role]
}

// NewRoleRegistry creates a new role registry
func NewRoleRegistry() *RoleRegistry {
	return &RoleRegistry{
		roleMap: make(map[string]map[PermissionKey]struct{}),
	}
}

// Global instance of the RoleRegistry
var roleRegistry = NewRoleRegistry()

// GetOrInitRole ensures a role exists; if not, it initializes it
func GetOrInitRole(role string) {
	roleRegistry.GetOrInitRole(role)
}

// MustGetRole ensures a role exists, panics if not registered
func MustGetRole(role string) {
	roleRegistry.MustGetRole(role)
}

// RegisterPermissionsToRole assigns permissions to a role
func RegisterPermissionsToRole(role string, permissions ...PermissionKey) {
	roleRegistry.RegisterPermissionsForRole(role, permissions)
}

func AssignPermissionsToRoles(permission PermissionKey, roles ...string) {
	for _, role := range roles {
		roleRegistry.RegisterPermissionsForRole(role, []PermissionKey{permission})
	}
}

// GetPermissionsForRole retrieves permissions assigned to a role
func GetPermissionsForRole(role string) ([]PermissionKey, bool) {
	return roleRegistry.GetPermissionsForRole(role)
}

func ReplacePermissionsForRole(role string, permissions []PermissionKey) {
	roleRegistry.ReplacePermissionsForRole(role, permissions)
}

// ---- RoleRegistry Methods ----

// GetOrInitRole ensures a role exists, initializing it if necessary
func (rr *RoleRegistry) GetOrInitRole(role string) {
	if _, exists := rr.roleMap[role]; !exists {
		rr.roleMap[role] = make(map[PermissionKey]struct{})
	}
}

// MustGetRole ensures a role exists; panics if not found
func (rr *RoleRegistry) MustGetRole(role string) {
	rr.locker.RLock()
	defer rr.locker.RUnlock()

	if _, exists := rr.roleMap[role]; !exists {
		panic(errors.Errorf("invalid role, role: %v not registered", role))
	}
}

// RegisterPermissionsToRole associates permissions with a role
func (rr *RoleRegistry) RegisterPermissionsForRole(role string, permissions []PermissionKey) {
	rr.locker.Lock()
	defer rr.locker.Unlock()

	rr.GetOrInitRole(role)
	for _, perm := range permissions {
		GetOrInitPermissionKey(perm)
		rr.roleMap[role][perm] = struct{}{}
	}
	IncreasePermissionVersion()
}

func (rr *RoleRegistry) ReplacePermissionsForRole(role string, permissions []PermissionKey) {
	rr.locker.Lock()
	defer rr.locker.Unlock()

	rr.GetOrInitRole(role)
	newP := map[PermissionKey]struct{}{}
	for _, perm := range permissions {
		GetOrInitPermissionKey(perm)
		newP[perm] = struct{}{}
	}
	rr.roleMap[role] = newP
	IncreasePermissionVersion()
}

// GetPermissionsForRole retrieves permissions assigned to a role
func (rr *RoleRegistry) GetPermissionsForRole(role string) ([]PermissionKey, bool) {
	rr.locker.RLock()
	defer rr.locker.RUnlock()

	permissions, exists := rr.roleMap[role]
	if !exists {
		return nil, false
	}

	permList := make([]PermissionKey, 0, len(permissions))
	for perm := range permissions {
		permList = append(permList, perm)
	}
	return permList, true
}

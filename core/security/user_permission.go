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
	"github.com/RoaringBitmap/roaring"
	log "github.com/cihub/seelog"
)

// UserAssignedPermission represents role and direct permissions for a user in a tenant
type UserAssignedPermission struct {
	PermissionVersion  int32
	AllowedPermissions *roaring.Bitmap // Allowed merged bitmap from roles + direct permits
	DeniedPermissions  *roaring.Bitmap // Explicit deny bitmap for the user
}

func NewUserAssignedPermission(allowed []string, denied []string) *UserAssignedPermission {
	ver := GetPermissionVersion()

	var allowedBitmap *roaring.Bitmap
	if allowed != nil && len(allowed) > 0 {
		allowedBitmap = roaring.NewBitmap()
		for _, permID := range allowed {
			allowedBitmap.Add(uint32(permissionRegistry.MustGetPermissionIDByKey(permID)))
		}
	}

	var deniedBitmap *roaring.Bitmap
	if denied != nil && len(denied) > 0 {
		deniedBitmap = roaring.NewBitmap()
		for _, permID := range denied {
			deniedBitmap.Add(uint32(permissionRegistry.MustGetPermissionIDByKey(permID)))
		}

		if allowedBitmap != nil {
			allowedBitmap.AndNot(deniedBitmap)
		}
	}

	per := UserAssignedPermission{
		PermissionVersion:  ver,
		AllowedPermissions: allowedBitmap,
		DeniedPermissions:  deniedBitmap,
	}
	return &per
}

// HasPermission checks if the user has a specific permission for a tenant
func (p *UserAssignedPermission) Dump() {
	if p.AllowedPermissions != nil {
		log.Info("allow:", p.AllowedPermissions.String())
	}
	if p.DeniedPermissions != nil {
		log.Info("deny:", p.DeniedPermissions.String())
	}
}

func (p *UserAssignedPermission) NeedRefresh() bool {
	return NeedRefreshPermission(p.PermissionVersion)
}

func (p *UserAssignedPermission) Validate(permIDs []PermissionID) bool {
	for _, id := range permIDs {
		if !p.ValidateFor(id) {
			return false
		}
	}
	return true
}

func (p *UserAssignedPermission) ValidateFor(permID PermissionID) bool {

	// First priority: Deny overrides everything
	if p.DeniedPermissions != nil && p.DeniedPermissions.Contains(uint32(permID)) {
		return false
	}

	// Allowed permissions (role-based + direct grants minus denies)
	if p.AllowedPermissions != nil {
		return p.AllowedPermissions.Contains(uint32(permID))
	}

	//deny by default
	return false
}

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
	"github.com/emirpasic/gods/sets/hashset"
	"infini.sh/framework/core/orm"
	"time"
)

// UserAssignedPermission represents role and direct permissions for a user in a tenant
type UserAssignedPermission struct {
	PermissionVersion  int32
	AllowedPermissions *roaring.Bitmap // Allowed merged bitmap from roles + direct permits
	DeniedPermissions  *roaring.Bitmap // Explicit deny bitmap for the user
}

func NewUserAssignedPermission(allowed []PermissionKey, denied []PermissionKey) *UserAssignedPermission {
	ver := GetPermissionVersion()

	var allowedBitmap *roaring.Bitmap
	if allowed != nil && len(allowed) > 0 {
		allowedBitmap = roaring.NewBitmap()
		for _, permID := range allowed {

			allowedBitmap.Add(uint32(permissionRegistry.GetOrInitPermissionIDByKey(permID)))
		}
	}

	var deniedBitmap *roaring.Bitmap
	if denied != nil && len(denied) > 0 {
		deniedBitmap = roaring.NewBitmap()
		for _, permID := range denied {
			deniedBitmap.Add(uint32(permissionRegistry.GetOrInitPermissionIDByKey(permID)))
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
		log.Debug("allow:", p.AllowedPermissions.String())
	}
	if p.DeniedPermissions != nil {
		log.Debug("deny:", p.DeniedPermissions.String())
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

	if p == nil {
		return false
	}

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

type ResourceType string

const DocumentResource ResourceType = "document"
const FileResource ResourceType = "file"
const FolderResource ResourceType = "folder"

func init() {
	orm.MustRegisterSchemaWithIndexName(&ExternalPermission{}, "external_permission")
}

// ExternalPermission represents a permission entry from an external system.
type ExternalPermission struct {
	orm.ORMObjectBase
	BatchNumber string `json:"batch_number,omitempty"` // Indicate the update batch

	Source       string       `json:"source,omitempty"`        // Data source identifier, e.g., "google_drive", "github"
	ExternalID   string       `json:"external_id,omitempty"`   // Raw ID from external system, if different from ResourceID
	ResourceID   string       `json:"resource_id,omitempty"`   // Internal Unique ID of the resource, e.g., file or folder ID
	ResourceType ResourceType `json:"resource_type,omitempty"` // Type of the resource: file, folder, repo, issue, etc.
	ResourcePath string       `json:"resource_path,omitempty"` // Optional: full path, e.g., /parent/folder/file.ext

	Permissions []ExternalPermissionEntry `json:"permissions,omitempty"` // List of users or groups with access to the resource

	Explicit bool   `json:"explicit,omitempty"`  // True if this is an explicit permission (not inherited)
	ParentID string `json:"parent_id,omitempty"` // ID of the parent resource (for inferring inheritance)

	ExternalUpdatedAt *time.Time `json:"external_updated_at,omitempty"` // Timestamp from the external system
}

// ExternalPermissionEntry represents a single ACL entry for a resource.
type ExternalPermissionEntry struct {
	PrincipalType   string `json:"principal_type,omitempty"`   // "user" or "group"
	PrincipalID     string `json:"principal_id,omitempty"`     // External user or group ID
	PrimaryIdentity string `json:"primary_identity,omitempty"` // Optional: username, email, user or group email for matching/mapping
	DisplayName     string `json:"display_name,omitempty"`     // Optional: "Alice Zhang", from external system
	Role            string `json:"role,omitempty"`             // External role: reader, editor, owner, commenter, etc.
	Inherited       bool   `json:"inherited,omitempty"`        // True if the permission is inherited
	InheritedFromID string `json:"inherited_from_id,omitempty"`
}

// ExternalPermissionMapping defines how external roles map to internal roles.
type ExternalPermissionMapping struct {
	orm.ORMObjectBase

	Source       string // Source system, e.g., "google_drive"
	ExternalRole string // External system role, e.g., "reader"
	MappedRole   string // Mapped internal role, e.g., "viewer"
	Default      bool   // Whether this mapping is the default fallback
	Visibility   string // Visibility: searchable, private, team, etc.
}

// ExternalUserMapping maps an external user ID/email to an internal user ID.
type ExternalUserMapping struct {
	orm.ORMObjectBase

	ExternalID string // e.g., abc@company.com or external user UUID
	Email      string // Optional for matching convenience
	Source     string // e.g., "google_drive", "github"
	UserID     string // Internal user ID in the system
}

// ExternalGroupMapping maps an external group ID to an internal group.
type ExternalGroupMapping struct {
	orm.ORMObjectBase

	ExternalID string // e.g., "group:marketing@company.com"
	Source     string // e.g., "google_drive", "github"
	GroupID    string // Internal group ID in the system
}

// Internal ACL entry resolved after mapping
type AccessControlEntry struct {
	orm.ORMObjectBase

	ResourceID   string       // ID of the protected resource
	ResourceType ResourceType // e.g., "document", "folder"
	SubjectID    string       // Internal user or group ID
	SubjectType  string       // "user" | "group"

	// Instead of SubjectID/Type strings, use bitsets for all users and groups with access
	AllowedUsers   *roaring.Bitmap // bitmap of allowed user IDs
	AllowedGroups  *roaring.Bitmap // bitmap of allowed group IDs
	PermissionBits *roaring.Bitmap // computed from roleRegistry

	Role      string // e.g. "reader", "editor"
	Source    string // Original source, e.g., "google_drive"
	Inherited bool   // Whether this ACL is inherited
}

type IDMapping struct {
	ExternalID string // e.g., "abc@company.com"
	InternalID uint64 // unique int ID used for bitset indexing
	Type       string // "user" or "group"
}

type GroupMembership struct {
	GroupID     string
	PrincipalID string // User ID
}

func ConvertPermissionKeysToHashSet(keys []PermissionKey) *hashset.Set {
	set := hashset.New()
	for _, v := range keys {
		set.Add(v)
	}
	return set
}

func ConvertPermissionHashSetToKeys(set *hashset.Set) []PermissionKey {
	if set == nil || set.Empty() {
		return nil
	}

	values := set.Values()
	keys := make([]PermissionKey, 0, len(values))
	for _, v := range values {
		if key, ok := v.(PermissionKey); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func IntersectSetsFast(a, b *hashset.Set) *hashset.Set {
	if a.Size() > b.Size() {
		a, b = b, a // iterate over smaller one
	}
	result := hashset.New()
	for _, v := range a.Values() {
		if b.Contains(v) {
			result.Add(v)
		}
	}
	return result
}

package security

import "infini.sh/framework/core/orm"

type UserRole struct {
	orm.ORMObjectBase
	Name        string                   `json:"name" elastic_mapping:"name: { type: keyword }" validate:"required|min_len:3"`
	Description string                   `json:"description"  elastic_mapping:"description: { type: text }"`
	Grants      PermissionAssignedToRole `json:"grants" elastic_mapping:"grants: { type: object }"`
}

type PermissionAssignedToRole struct {
	AllowedPermissions []Action `json:"permissions,omitempty" elastic_mapping:"permissions: { type: keyword }"`
	DeniedPermissions  []Action `json:"denied_permissions,omitempty" elastic_mapping:"denied_permissions: { type: keyword }"`
}

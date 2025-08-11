package security

import "infini.sh/framework/core/orm"

type UserRole struct {
	ID   string `json:"id" elastic_mapping:"id: { type: keyword }"`
	Name string `json:"name" elastic_mapping:"name: { type: keyword }"`
}

type PermissionAssignedToRole struct {
	orm.ORMObjectBase
	RoleID             string   `json:"role_id" elastic_mapping:"role_id: { type: keyword }"`
	ResourceID         string   `json:"resource_id" elastic_mapping:"resource_id: { type: keyword }"`
	AllowedPermissions []Action `json:"allowed_permissions" elastic_mapping:"allowed_permissions: { type: keyword }"`
}

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package rbac

import (
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
)

type User struct {
	orm.ORMObjectBase

	AuthProvider string `json:"auth_provider" protected:"true" elastic_mapping:"auth_provider: { type: keyword }"`
	Username     string `json:"username" protected:"true" elastic_mapping:"name: { type: keyword }"`
	Password     string `json:"password,omitempty"  elastic_mapping:"password: { type: keyword }"`

	Nickname string `json:"nickname,omitempty"  elastic_mapping:"nickname: { type: keyword }"`

	Email         string `json:"email,omitempty" protected:"true" elastic_mapping:"email: { type: keyword }"`
	EmailVerified bool   `json:"email_verified" protected:"true" protected:"true" elastic_mapping:"email_verified: { type: keyword }"`
	Phone         string `json:"phone,omitempty" protected:"true" elastic_mapping:"phone: { type: keyword }"`

	Tags []string `json:"tags,omitempty" elastic_mapping:"mobile: { type: keyword }"`

	AvatarUrl string `json:"avatar_url,omitempty" elastic_mapping:"avatar_url: { type: keyword }"`

	Tenant *model.TenantInfo `json:"tenant,omitempty" protected:"true" elastic_mapping:"tenant: { type: object }"` //tenant info for multi-tenant platform user
	Roles  []UserRole        `json:"roles" protected:"true" elastic_mapping:"roles: { type: object }"`

	Payload interface{} `json:"payload,omitempty" elastic_mapping:"payload: { type: object }"` //used for storing additional data derived from auth provider
}

func (user *User) GetPermissions() (roles []string, privileges []string) {
	for _, v := range user.Roles {
		role, ok := RoleMap[v.Name]
		if ok {
			roles = append(roles, v.Name)
			privileges = append(privileges, role.Privilege.Platform...)
		}
	}
	return roles, privileges
}

type UserRole struct {
	ID   string `json:"id" elastic_mapping:"id: { type: keyword }"`
	Name string `json:"name" elastic_mapping:"name: { type: keyword }"`
}

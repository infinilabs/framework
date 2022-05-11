/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package rbac

import (
	"infini.sh/framework/core/orm"
)

type User struct {
	orm.ORMObjectBase

	Name     string     `json:"name"  elastic_mapping:"name: { type: keyword }"`
	NickName string     `json:"nick_name"  elastic_mapping:"nick_name: { type: keyword }"`
	Password string     `json:"password"  elastic_mapping:"password: { type: keyword }"`
	Email    string     `json:"email" elastic_mapping:"email: { type: keyword }"`
	Phone    string     `json:"phone" elastic_mapping:"phone: { type: keyword }"`
	Tags     []string   `json:"tags" elastic_mapping:"mobile: { type: keyword }"`
	Roles    []UserRole `json:"roles" elastic_mapping:"roles: { type: object }"`
}

type UserRole struct {
	ID string `json:"id" elastic_mapping:"id: { type: keyword }"`
	Name string `json:"name" elastic_mapping:"name: { type: keyword }"`
}

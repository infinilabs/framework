/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/orm"
)

// abstract layer for user or teams
type OrganizationPrincipal struct {
	orm.ORMObjectBase
	Type        string `json:"type,omitempty" elastic_mapping:"type:{type:keyword}"` //  "type": "user", // or "group"
	Name        string `json:"name,omitempty" elastic_mapping:"name:{type:keyword,copy_to:combined_fulltext}"`
	Description string `json:"description,omitempty" elastic_mapping:"description:{type:keyword,copy_to:combined_fulltext}"`
	Avatar  string `json:"avatar,omitempty" elastic_mapping:"avatar:{type:keyword}"`
}

const PrincipalTypeUser = "user"
const PrincipalTypeTeam = "team"

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/orm"
)

// abstract layer for user or teams
type OrganizationPrincipal struct {
	ID   string `json:"id,omitempty" elastic_mapping:"id:{type:keyword}"`
	Type string `json:"type,omitempty" elastic_mapping:"type:{type:keyword}"` //  "type": "user", // or "team"
	Name        string `json:"name,omitempty" elastic_mapping:"name:{type:text,copy_to:combined_fulltext,fields:{keyword: {type: keyword}, pinyin: {type: text, analyzer: pinyin_analyzer}}}"`
	Description string `json:"description,omitempty" elastic_mapping:"description:{type:keyword,copy_to:combined_fulltext}"`
	Avatar      string `json:"avatar,omitempty" elastic_mapping:"avatar:{type:keyword}"`
}

type OrganizationPrincipalCache struct {
	orm.ORMObjectBase
	Principal OrganizationPrincipal `json:"principal,omitempty" elastic_mapping:"principal:{type:object}"`
	Version   string                `json:"version,omitempty" elastic_mapping:"version:{type:keyword}"`
}

const PrincipalTypeUser = "user"
const PrincipalTypeTeam = "team"

func init() {
	orm.MustRegisterSchemaWithIndexName(&OrganizationPrincipalCache{}, "principals-cache")
}

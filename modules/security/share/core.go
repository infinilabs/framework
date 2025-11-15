/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	"fmt"
	"infini.sh/framework/core/orm"
)


type SharingRecord struct {
	orm.ORMObjectBase
	SimplifySharingRecord
	GrantedBy string `json:"granted_by,omitempty" elastic_mapping:"granted_by:{type:keyword}"`

	// Path-based fields
	ResourceIsFolder    bool   `json:"resource_is_folder,omitempty" elastic_mapping:"resource_is_folder:{type:boolean}"`       //eg: the resource is a folder, means we are sharing a folder
	PathPattern         string `json:"path_pattern,omitempty" elastic_mapping:"path_pattern:{type:keyword}"`                   // e.g., "/documents/*" for wildcards
	Recursive           bool   `json:"recursive,omitempty" elastic_mapping:"recursive:{type:boolean}"`                         // Apply to sub-paths
	InheritedFrom       string `json:"inherited_from,omitempty" elastic_mapping:"inherited_from:{type:keyword}"`               // Parent share ID
	InheritedFromFolder string `json:"inherited_from_folder,omitempty" elastic_mapping:"inherited_from_folder:{type:keyword}"` // Parent share ID
	Via                 string `json:"via,omitempty" elastic_mapping:"via:{type:keyword}"`                                     // via: direct / inherit
}

const ViaInherit = "inherit"

type SimplifySharingRecord struct {
	ResourceEntity
	PrincipalType        string            `json:"principal_type,omitempty" elastic_mapping:"principal_type:{type:keyword}"` //  "type": "user", // or "group" or "link"
	PrincipalID          string            `json:"principal_id,omitempty" elastic_mapping:"principal_id:{type:keyword}"`
	PrincipalDisplayName string            `json:"display_name,omitempty" elastic_mapping:"display_name:{type:keyword,copy_to:combined_fulltext}"`
	Permission           SharingPermission `json:"permission" elastic_mapping:"permission:{type:byte}"`
}

func (r *SimplifySharingRecord) GetPrincipalKey() string {
	return fmt.Sprintf("%v-%v", r.PrincipalType, r.PrincipalID)
}

type ResourceEntity struct {
	ResourceCategoryType string `json:"resource_category_type,omitempty" elastic_mapping:"resource_category_type:{type:keyword}"` //eg: datasource
	ResourceCategoryID   string `json:"resource_category_id,omitempty" elastic_mapping:"resource_category_id:{type:keyword}"`     //eg: datasource's id

	ResourceType               string `json:"resource_type,omitempty" elastic_mapping:"resource_type:{type:keyword}"`                                 //eg: document
	ResourceID                 string `json:"resource_id,omitempty" elastic_mapping:"resource_id:{type:keyword}"`                                     //eg: document's id
	ResourceParentPath         string `json:"resource_parent_path,omitempty" elastic_mapping:"resource_parent_path:{type:keyword}"`                   // resource belongs to which folder/path. e.g., "/documents/2023/reports"
	ResourceParentPathReversed string `json:"resource_parent_path_reversed,omitempty" elastic_mapping:"resource_parent_path_reversed:{type:keyword}"` // reversed for faster path filtering.
	ResourceFullPath           string `json:"resource_full_path,omitempty" elastic_mapping:"resource_full_path:{type:keyword}"`                       // resource's full path. e.g., "/documents/2023/reports/xxx.pdf"
}

func (r *ResourceEntity) GetResourceKey() string {
	return fmt.Sprintf("%v-%v-%v-%v", r.ResourceCategoryType, r.ResourceCategoryID, r.ResourceType, r.ResourceID)
}

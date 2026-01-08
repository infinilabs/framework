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

package orm

// const Term QueryType = "term"
const PrefixQueryType QueryType = "prefix"
const Wildcard QueryType = "wildcard"
const Regexp QueryType = "regexp"
const Match QueryType = "match"
const QueryStringType QueryType = "query_string"
const RangeGt QueryType = "gt"
const RangeGte QueryType = "gte"
const RangeLt QueryType = "lt"
const RangeLte QueryType = "lte"

const StringTerms QueryType = "string_terms"
const Terms QueryType = "terms"

const WaitForRefresh = "wait_for"
const ImmediatelyRefresh = "true"
const KeepSystemFields = "keep_system_fields"
const NoAutoUpdateUpdatedField = "not_auto_update_updated_field"
const MergePartialFieldsBeforeUpdate = "merge_partial_fields_before_update"
const CheckExistsBeforeDelete = "check_exists_before_delete"
const CheckExistsBeforeUpdate = "check_exists_before_update"
const CreateIfNotExistsForUpdate = "create_if_not_exists_for_update"
const AssignToCurrentUserIfNotExists = "assign_to_current_user_if_not_exists"

const PermissionCheckingScope = "permission_checking_scope"
const DirectReadWithoutPermissionCheck = "direct_read_without_permission_check"
const DirectWriteWithoutPermissionCheck = "direct_write_without_permission_check"

const SharingEnabled = "sharing_enabled"
const SharingResourceType = "sharing_resource_type"
const SharingResourceParentPath = "sharing_resource_parent_path"

const SharingCheckingResourceCategoryEnabled = "sharing_resource_category_filter_enabled"
const SharingCheckingInheritedRulesEnabled = "sharing_checking_inherited_rules_enabled"
const SharingResourceCategoryType = "sharing_resource_category_type"
const SharingResourceCategoryID = "sharing_resource_category_id"
const SharingResourceCategoryFilterField = "sharing_resource_category_filter_field"

const SharingCategoryCheckingChildrenEnabled = "sharing_resource_is_category" //if it is a category, can be show up by children rules

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"sort"
	"strings"
)

// SharingService handles core sharing operations
type SharingService struct{}

// NewSharingService creates a new sharing service
func NewSharingService() *SharingService {
	return &SharingService{}
}

func NewResourceEntity(resourceType string, resourceID string, resourcePath string) ResourceEntity {
	r := ResourceEntity{}
	r.ResourceType = resourceType
	r.ResourceID = resourceID
	r.ResourceFullPath = resourcePath
	return r
}

// BuildEffectiveInheritedRules returns the most specific (effective) permission rules
// for all principals that apply to the given targetPath.
func BuildEffectiveInheritedRules(rules []SharingRecord, targetPath string, debug bool) *SharingRecord {
	if len(rules) == 0 {
		return nil
	}

	// Only consider rules that apply to (or contain) the target path
	applicable := make([]SharingRecord, 0, len(rules))
	for _, r := range rules {
		if !r.ResourceIsFolder {
			continue
		}
		// Example: rule.ResourceFullPath = "/Users/medcl/Downloads/"
		// targetPath = "/Users/medcl/Downloads/AngryBots/Release/"
		if strings.HasPrefix(targetPath, r.ResourceFullPath) {
			applicable = append(applicable, r)
		} else if debug {
			log.Debugf("Skipping unrelated rule %v (does not apply to %v)", r.ResourceFullPath, targetPath)
		}
	}

	if len(applicable) == 0 {
		if debug {
			log.Debugf("No applicable rules found for targetPath: %v", targetPath)
		}
		return nil
	}

	// Sort by longest path first (most specific)
	sort.SliceStable(applicable, func(i, j int) bool {
		return len(applicable[i].ResourceFullPath) > len(applicable[j].ResourceFullPath)
	})

	// Take the most specific applicable rule
	base := applicable[0]

	// If it’s exactly for this folder, it’s a direct rule
	if base.ResourceFullPath == targetPath {
		if debug {
			log.Debugf("🏁 direct rule matched for %v: %v", targetPath, util.ToJson(base, true))
		}
		return &base
	}

	// Otherwise, create an inherited version of the rule
	rule := base
	rule.ResourceType = base.ResourceType
	rule.ResourceID = base.ResourceID
	rule.ResourceParentPath = base.ResourceParentPath
	rule.Via = ViaInherit
	rule.InheritedFromFolder = base.ResourceFullPath
	rule.InheritedFrom = base.ResourceID
	rule.ResourceFullPath = targetPath
	rule.GrantedBy = ""
	rule.ResourceParentPathReversed = ""
	rule.ResourceIsFolder = true
	rule.System = nil
	rule.ID = "N/A"

	if debug {
		log.Debugf("🌿 inherited rule generated for %v from %v", targetPath, base.ResourceFullPath)
	}

	return &rule
}

// GetUserExplicitEffectivePermission gets the effective permission for a user on a resource, NOTICE: user can be owner, so there is no sharing rules
func (s *SharingService) GetUserExplicitEffectivePermission(userID string, r ResourceEntity) (SharingPermission, error) {

	ctx := orm.NewContext()
	req := []ResourceEntity{}
	req = append(req, r)
	shares, err := s.BatchGetShares(ctx, userID, req)
	if err != nil {
		log.Errorf("get user %v permission %v for resource %v/%v", userID, 0, r.ResourceType, r.ResourceID)
		return 0, err
	}

	// Return the highest permission level
	maxPermission := None // Default to none
	for _, share := range shares {
		//let's double check
		if share.ResourceID != r.ResourceID {
			panic("invalid sharing record, resource_id is not correct")
		}
		if share.Permission > maxPermission {
			maxPermission = share.Permission
		}
	}

	log.Debugf("user: %v, permission: %v for resource:%v/%v /# %v share records", userID, maxPermission, r.ResourceType, r.ResourceID, len(shares))

	return maxPermission, err
}

// ListShares returns all current shares for a resource, if the userID is not empty, then only filter permissions for this user only
func (s *SharingService) BatchGetShares(ctx *orm.Context, userID string, req []ResourceEntity) ([]SharingRecord, error) {

	orm.WithModel(ctx, &SharingRecord{})
	ctx.DirectReadAccess()
	// Handle URL query args, convert to query builder
	builder := orm.NewQuery()
	builder.Size(1000)
	paths := map[string]map[string][]ResourceEntity{} //datasourceID -> Path

	//resources:=map[string]bool{}
	datasourceIDs := []string{}
	reqs := map[string]ResourceEntity{}
	for _, v := range req {

		if v.ResourceType == "" || v.ResourceID == "" {
			panic("resource type can't be empty")
		}

		//group resources by resource's parent path
		//only handle `datasource` for nested inherited permission
		if v.ResourceCategoryType == "datasource" {
			if v.ResourceCategoryID != "" && v.ResourceParentPath != "" {
				x, ok := paths[v.ResourceCategoryID]
				if !ok {
					datasourceIDs = append(datasourceIDs, v.ResourceCategoryID)
					x = map[string][]ResourceEntity{}
				}
				list := x[v.ResourceParentPath]
				list = append(list, v)
				x[v.ResourceParentPath] = list
				paths[v.ResourceCategoryID] = x
			}
		}

		reqs[v.ResourceID] = v
		x := orm.MustQuery(orm.TermQuery("resource_type", v.ResourceType), orm.TermQuery("resource_id", v.ResourceID))

		if v.ResourceParentPath != "" {
			x.MustClauses = append(x.MustClauses, orm.TermQuery("resource_parent_path", v.ResourceParentPath))
		}

		if userID != "" {
			x.MustClauses = append(x.MustClauses, orm.TermQuery("principal_id", userID))
			x.MustClauses = append(x.MustClauses, orm.TermQuery("principal_type", security.PrincipalTypeUser))
		}

		builder.Should(x)
	}
	builder.MinimumShouldMatch(1)

	//directly sharing rules
	docs := []SharingRecord{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &docs, builder, nil)
	if err != nil {
		return nil, errors.Errorf("failed to search shares: %v", err)
	}

	//for datasource only right now
	datasourceLevelShares := map[string][]SharingRecord{} //datasource_id -> rules
	if len(datasourceIDs) > 0 {
		//get all rules for these datasources
		rules, _ := s.GetResourcePermissions(userID, "datasource", datasourceIDs)
		for _, v := range rules {
			ruleArray, ok := datasourceLevelShares[v.ResourceID]
			if !ok {
				ruleArray = []SharingRecord{}
			}
			ruleArray = append(ruleArray, v)
			datasourceLevelShares[v.ResourceID] = ruleArray
		}
	}

	//if we have paths need to check
	if len(paths) > 0 {

		resourcesDirectPermission := map[string]SharingRecord{} //resource_key->user_key->SharingRecord

		//handle directly shared rules, it should be have only record for each resource
		for _, doc := range docs {
			resourcesDirectPermission[doc.GetResourceKey()+doc.GetPrincipalKey()] = doc
		}

		//now we need to get all path based permissions under these folder
		//datasource -> path -> path -> path ...
		for datasourceID, v := range paths {
			//log.Error("let's walk these paths for datasource:", datasourceID)
			for p, list := range v {
				var rules []SharingRecord
				resourceType := "document" //it should be documents only, since we only support this kind of logic for documents inside datasource
				var globalShareMustFilters = []*orm.Clause{}
				globalShareMustFilters = append(globalShareMustFilters, orm.TermQuery("resource_category_type", "datasource")) //fixed use case
				globalShareMustFilters = append(globalShareMustFilters, orm.TermQuery("resource_category_id", datasourceID))
				globalShareMustFilters = append(globalShareMustFilters, orm.TermQuery("resource_is_folder", true))
				rules, _ = GetSharingRulesV2(security.PrincipalTypeUser, userID, resourceType, "", p, globalShareMustFilters)
				inheritedPermissionRules := map[string]SharingRecord{}
				if len(rules) > 0 {
					// Sort by longest path first → most specific
					sort.Slice(rules, func(i, j int) bool {
						return len(rules[i].ResourceFullPath) > len(rules[j].ResourceFullPath)
					})

					//keep all inherited permissions all the way
					for _, r := range rules {
						_, ok1 := inheritedPermissionRules[r.GetPrincipalKey()]
						if !ok1 {
							inheritedPermissionRules[r.GetPrincipalKey()] = r
						}
					}
				}

				for _, x := range list {

					//handle inherited permissions
					if len(inheritedPermissionRules) > 0 {
						for principalKey, rule := range inheritedPermissionRules {
							_, ok := resourcesDirectPermission[x.GetResourceKey()+principalKey]
							if !ok { //check if there is already some rule for the same resource and the same PrincipalKey
								rule.ResourceType = x.ResourceType
								rule.ResourceID = x.ResourceID
								rule.ResourceParentPath = x.ResourceParentPath
								rule.Via = ViaInherit
								rule.InheritedFromFolder = rule.ResourceFullPath
								rule.InheritedFrom = rule.ResourceID
								rule.ResourceFullPath = x.ResourceFullPath
								rule.GrantedBy = ""
								rule.ResourceParentPathReversed = ""
								rule.ResourceIsFolder = false
								rule.System = nil
								rule.ID = "N/A"

								//save this rule back to map
								resourcesDirectPermission[x.GetResourceKey()+principalKey] = rule
								docs = append(docs, rule)
							}
						}
					}

					//if the resource contains datasource level rules, we need to merge into results
					//no matter if have any inherit or direct associated with these resource or not
					if len(datasourceLevelShares) > 0 {
						if perm, ok := datasourceLevelShares[datasourceID]; ok {
							for _, rule := range perm {
								_, ok := resourcesDirectPermission[x.GetResourceKey()+rule.GetPrincipalKey()]
								if !ok {
									rule.ResourceType = x.ResourceType
									rule.ResourceID = x.ResourceID
									rule.ResourceParentPath = x.ResourceParentPath
									rule.Via = ViaInherit
									rule.InheritedFromFolder = rule.ResourceFullPath
									rule.InheritedFrom = rule.ResourceID
									rule.ResourceFullPath = x.ResourceFullPath
									rule.GrantedBy = ""
									rule.ResourceParentPathReversed = ""
									rule.ResourceIsFolder = false
									rule.System = nil
									rule.ID = "N/A"
									docs = append(docs, rule)
								}
							}
						}
					}
				}
			}
		}
	}

	return docs, nil
}

type SharingResponse struct {
}

// CreateOrUpdateShares handles sharing resources with users/groups
func (s *SharingService) CreateOrUpdateShares(ctx *orm.Context, userID string, req *ShareRequest) (*BulkOpResponses[SharingRecord], error) {
	list := NewBulkOpResponses[SharingRecord]()

	//TODO, verify these share records, check the resource paths and the resource are well and correctly aligned
	// Handle revokes
	for _, revoke := range req.Revokes {
		if revoke.ID != "" {
			// TODO: permission check, validate current user's operation
			// 1. if the resource is owned by current user
			// 2. current user with `share` permission
			ctx.DirectAccess() // TODO remove this line
			err := orm.Delete(ctx, &revoke)
			if err != nil {
				return nil, errors.Errorf("failed to revoke share: %v", err)
			}
			list.AddDeleted(&revoke)
		}
	}

	// Create records for each share
	for _, share := range req.Shares {
		share.ResourceParentPath = util.NormalizeFolderPath(share.ResourceParentPath)
		if share.ResourceIsFolder {
			share.ResourceFullPath = util.NormalizeFolderPath(share.ResourceFullPath)
		}

		// Check if a share already exists for this combination
		resourceParentPath := s.getResourcePath(ctx, share.ResourceType, share.ResourceID, share.ResourceParentPath)
		existingShare, err := s.checkExistingShare(share.ResourceID, share.ResourceType, share.PrincipalID, share.PrincipalType, resourceParentPath)
		if err != nil {
			return list, errors.Errorf("failed to check existing share: %v", err)
		}

		if existingShare != nil {
			// Check if permission is actually different
			if existingShare.Permission == share.Permission {
				log.Debugf("Share already exists with same permission for principal %s on resource %s", share.PrincipalID, share.ResourceID)
				list.AddUnchanged(existingShare)
				continue // Skip if permission is the same
			}

			// Update existing share with new permission
			err := s.updateExistingShare(existingShare, share.Permission, userID)
			if err != nil {
				return list, errors.Errorf("failed to update existing share: %v", err)
			}

			list.AddUpdated(existingShare)
			log.Debugf("Updated existing share for principal %s on resource %s (permission changed from %v to %v)",
				share.PrincipalID, share.ResourceID, existingShare.Permission, share.Permission)
		} else {
			share.ResourceParentPathReversed = util.ReverseString(share.ResourceParentPath)
			if err := orm.Create(ctx, &share); err != nil {
				return list, errors.Errorf("failed to create share: %v", err)
			}
			list.AddCreated(&share)
			log.Debugf("Created new share for principal %s on resource %s", share.PrincipalID, share.ResourceID)
		}
	}

	return list, nil
}

// Helper method to get resource path
func (s *SharingService) getResourcePath(ctx *orm.Context, resourceType, resourceID, resourcePath string) string {
	// This is a placeholder - in real implementation, you would fetch the resource
	// from the appropriate data source and extract its path
	// For now, return root as default
	if resourcePath != "" {
		return resourcePath
	}
	return "/"
}

// GetInheritOrCurrentPathSharingRules
func GetSharingRules(principalType string, principalID string, resourceType string, resourceID string, resourceParentPath string, filters []*orm.Clause) ([]SharingRecord, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Size(1000)

	//filter directly shared objects or folder
	clauses := []*orm.Clause{}
	if resourceType != "" {
		clauses = append(clauses, orm.TermQuery("resource_type", resourceType))
	}

	if principalID != "" {
		clauses = append(clauses, orm.TermQuery("principal_id", principalID))
	}

	if principalType != "" {
		clauses = append(clauses, orm.TermQuery("principal_type", principalType))
	}

	if resourceID != "" {
		clauses = append(clauses, orm.TermQuery("resource_id", resourceID))
	}

	if len(clauses) == 0 {
		panic("invalid clauses, should not be empty")
	}

	if resourceParentPath != "" {
		itemsClauses := make([]*orm.Clause, len(clauses))
		copy(itemsClauses, clauses)

		//add more filters
		clauses = append(clauses, orm.TermQuery("resource_is_folder", true))
		clauses = append(clauses, orm.TermsQuery("resource_full_path", util.GetPathAncestors(resourceParentPath)))

		//filter specify items only in this path
		itemsClauses = append(itemsClauses, orm.TermQuery("resource_parent_path", resourceParentPath))
		qb.Should(orm.MustQuery(itemsClauses...))
		qb.MinimumShouldMatch(1)
	}

	//get all possible folder rules, all parent folder with share rules
	qb.Should(orm.MustQuery(clauses...)).MinimumShouldMatch(1)

	qb.Must(filters...)

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}
	return shares, nil
}

// only check for full path, no self path's items
func GetSharingRulesV2(principalType string, principalID string, resourceType string, resourceID string, resourceParentPath string, filters []*orm.Clause) ([]SharingRecord, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Size(1000)

	//filter directly shared objects or folder
	clauses := []*orm.Clause{}
	if resourceType != "" {
		clauses = append(clauses, orm.TermQuery("resource_type", resourceType))
	}

	if principalID != "" {
		clauses = append(clauses, orm.TermQuery("principal_id", principalID))
	}

	if principalType != "" {
		clauses = append(clauses, orm.TermQuery("principal_type", principalType))
	}

	if resourceID != "" {
		clauses = append(clauses, orm.TermQuery("resource_id", resourceID))
	}

	if len(clauses) == 0 {
		panic("invalid clauses, should not be empty")
	}

	//check current resource's parent paths
	if resourceParentPath != "" {
		//add more filters
		clauses = append(clauses, orm.TermQuery("resource_is_folder", true))
		clauses = append(clauses, orm.TermsQuery("resource_full_path", util.GetPathAncestors(resourceParentPath)))
	}

	//get all possible folder rules, all parent folder with share rules
	qb.Must(clauses...)
	qb.Must(filters...)

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}
	return shares, nil
}

func (s *SharingService) GetCategoryObjectFromSharedObjects(userID string, resourceType string) ([]string, error) {

	out := []string{}
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_category_type", resourceType))
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))
	qb.Size(1000)

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithCollapseField(ctx, "resource_category_id")
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return out, err
	}

	if len(shares) > 0 {
		for _, v := range shares {
			out = append(out, v.ResourceCategoryID)
		}
	}

	return out, nil
}

// get flat level resources, no nested path hierarchy
func (s *SharingService) GetResourceIDsByResourceTypeAndUserID(userID string, resourceType string) ([]string, error) {

	out := []string{}
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_type", resourceType))
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))
	qb.Size(1000)
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithCollapseField(ctx, "resource_category_id")
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return out, err
	}

	if len(shares) > 0 {
		for _, v := range shares {
			if v.Permission > None {
				out = append(out, v.ResourceID)
			}
		}
	}

	return out, nil
}

// read+ permit rules
func (s *SharingService) GetDirectResourceRulesByResourceTypeAndUserID(userID string, resourceType string, resourceID []string, miniPermission SharingPermission) ([]SharingRecord, error) {

	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_type", resourceType))
	if len(resourceID) > 0 {
		qb.Must(orm.TermsQuery("resource_id", resourceID))
	}
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))
	if miniPermission > -1 {
		qb.Must(orm.Range("permission").Gte(miniPermission))
	}
	qb.Size(10000)
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}
	return shares, nil
}

func (s *SharingService) GetDirectResourceRulesByResourceCategoryAndUserID(userID string, resourceType string, resourceCategoryType string, resourceCategoryID []string, miniPermission SharingPermission) ([]SharingRecord, error) {

	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_type", resourceType))
	qb.Must(orm.TermQuery("resource_category_type", resourceCategoryType))
	if len(resourceCategoryID) > 0 {
		qb.Must(orm.TermsQuery("resource_category_id", resourceCategoryID))
	}
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))
	if miniPermission > -1 {
		qb.Must(orm.Range("permission").Gte(miniPermission))
	}
	qb.Size(10000)

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}
	return shares, nil
}

func (s *SharingService) GetCategoryVisibleWithChildrenSharedObjects(userID string, resourceType, resourceID string) (SharingPermission, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Size(1000)
	qb.Must(orm.TermQuery("resource_category_type", resourceType))
	qb.Must(orm.TermQuery("resource_category_id", resourceID))
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithCollapseField(ctx, "resource_category_id")
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return None, err
	}

	if len(shares) > 0 {
		return View, err
	}

	return None, nil
}

func (s *SharingService) GetAllCategoryVisibleWithChildrenSharedObjects(userID string, resourceType string) ([]SharingRecord, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Size(1000)
	qb.Must(orm.TermQuery("resource_category_type", resourceType))
	qb.Must(orm.TermQuery("principal_id", userID))
	qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithCollapseField(ctx, "resource_category_id")
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}
	return shares, nil
}

func (s *SharingService) GetResourcePermissions(userID string, resourceType string, resourceIDs []string) ([]SharingRecord, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Size(1000)
	qb.Must(orm.TermQuery("resource_type", resourceType))
	qb.Must(orm.TermsQuery("resource_id", resourceIDs))

	if userID != "" {
		qb.Must(orm.TermQuery("principal_id", userID))
		qb.Must(orm.TermQuery("principal_type", security.PrincipalTypeUser))
	}

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return shares, err
	}

	return shares, nil
}

// checkExistingShare checks if a share already exists for the same resource, principal, and path
func (s *SharingService) checkExistingShare(resourceID, resourceType, principalID, principalType, resourceParentPath string) (*SharingRecord, error) {
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_id", resourceID))
	qb.Must(orm.TermQuery("resource_type", resourceType))
	qb.Must(orm.TermQuery("principal_id", principalID))
	qb.Must(orm.TermQuery("principal_type", principalType))
	qb.Must(orm.TermQuery("resource_parent_path", resourceParentPath))
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return nil, err
	}

	if len(shares) > 0 {
		return &shares[0], nil
	}

	return nil, nil
}

// updateExistingShare updates an existing share record
func (s *SharingService) updateExistingShare(existingShare *SharingRecord, newPermission SharingPermission, grantedBy string) error {
	ctx := orm.NewContext()
	ctx.DirectAccess()
	existingShare.Permission = newPermission
	existingShare.GrantedBy = grantedBy

	return orm.Save(ctx, existingShare)
}

// permissionToString converts SharingPermission to string representation
func permissionToString(perm SharingPermission) string {
	switch perm {
	case None:
		return "None"
	case View:
		return "view"
	case Comment:
		return "comment"
	case Edit:
		return "edit"
	case Share:
		return "share"
	case Owner:
		return "owner"
	default:
		return "unknown"
	}
}

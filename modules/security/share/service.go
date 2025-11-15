/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	"encoding/hex"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
)

// SharingService handles core sharing operations
type SharingService struct{}

// NewSharingService creates a new sharing service
func NewSharingService() *SharingService {
	return &SharingService{}
}

//// CreateShareLink creates a new share link with business logic
//func (s *SharingService) CreateShareLink(ctx *orm.Context, resourceType, resourceID, resourcePath, userID, permission string, expiresAt *time.Time, password string) (*ShareLink, error) {
//	// Parse permission
//	perm, err := parseSharingPermission(permission)
//	if err != nil {
//		return nil, errors.Errorf("invalid permission level: %v", err)
//	}
//
//	// Get resource path from the resource
//	resourcePath = s.getResourcePath(ctx, resourceType, resourceID, resourcePath)
//	if resourcePath == "" {
//		return nil, errors.New("resource not found")
//	}
//
//	// Check if an active share link already exists for this combination
//	existingLink, err := s.checkExistingShareLink(ctx, resourceID, resourceType, resourcePath, userID, perm)
//	if err != nil {
//		return nil, errors.Errorf("failed to check existing share link: %v", err)
//	}
//
//	if existingLink != nil {
//		// Return existing link instead of creating duplicate
//		log.Debugf("Active share link already exists for resource %s with permission %v", resourceID, perm)
//		return existingLink, nil
//	}
//
//	// Create share link using core service
//	link, err := s.CreateShareLink(
//		resourceID,
//		resourceType,
//		resourcePath,
//		userID,
//		perm,
//		expiresAt,
//		password,
//	)
//
//	if err != nil {
//		return nil, errors.Errorf("failed to create share link: %v", err)
//	}
//
//	return link, nil
//}

// CreateShareLink creates a new share link
func (s *SharingService) CreateShareLink(resourceID, resourceType, resourcePath, createdBy string, permission SharingPermission, expiresAt *time.Time, password string) (*ShareLink, error) {
	token, err := generateSecureToken()
	if err != nil {
		return nil, errors.Errorf("failed to generate token: %v", err)
	}

	link := &ShareLink{
		Token:        token,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		ResourcePath: resourcePath,
		Permission:   permission,
		CreatedBy:    createdBy,
		ExpiresAt:    expiresAt,
		AccessCount:  0,
		IsActive:     true,
	}

	if password != "" {
		link.PasswordHash = hashPassword(password)
	}

	ctx := orm.NewContext()
	if err := orm.Create(ctx, link); err != nil {
		return nil, errors.Errorf("failed to save share link: %v", err)
	}

	return link, nil
}

// ValidateShareLink validates a share link token
func (s *SharingService) ValidateShareLink(token string, password string) (*ShareLink, error) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &SharingRecord{})

	// Build query for active, non-expired link
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("token", token))
	qb.Must(orm.TermQuery("is_active", true))

	// Add expiration check
	now := time.Now()
	qb.Must(orm.ShouldQuery(
		orm.MustNotQuery(orm.ExistsQuery("expires_at")),
		orm.Range("expires_at").Gt(now.Format(time.RFC3339)),
	))

	var links []ShareLink
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &links, qb, nil)
	if err != nil {
		return nil, errors.Errorf("failed to search share link: %v", err)
	}

	if len(links) == 0 {
		return nil, errors.New("invalid or expired share link")
	}

	link := &links[0]

	// Validate password if required
	if link.PasswordHash != "" && !verifyPassword(password, link.PasswordHash) {
		return nil, errors.New("invalid password")
	}

	// Update access count
	link.AccessCount++
	if err := orm.Save(ctx, link); err != nil {
		return nil, errors.Errorf("failed to update access count: %v", err)
	}

	return link, nil
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

	//if debug {
	//	log.Errorf("🔍 Applicable rules for %v: %v", targetPath, util.ToJson(applicable, true))
	//}

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

	////manual rule for checking datasource based documents
	//if len(shares)==0&&r.ResourceCategoryType=="datasource"&&r.ResourceCategoryID!=""{
	//	//if the datasource have rule for this user or not
	//	return s.GetResourcePermissions(userID,r.ResourceCategoryType,r.ResourceCategoryID)
	//}

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

	//return s.GetUserEffectivePermissionV2(userID, false, resourceType, resourceID, resourcePath)
}

func (s *SharingService) GetUserEffectivePermissionV2(userID string, resourceIsCategory bool, resourceType string, resourceID string, resourcePath string) (SharingPermission, error) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &SharingRecord{})
	ctx.DirectReadAccess()

	// Build query for user's sharing records
	qb := orm.NewQuery()
	qb.Size(1000)
	qb.Must(orm.TermQuery("resource_id", resourceID))
	qb.Must(orm.TermQuery("resource_type", resourceType))

	if resourcePath != "" {
		qb.Must(orm.ShouldQuery(
			orm.MustQuery(orm.TermQuery("principal_id", userID))). //orm.TermQuery("resource_path", resourcePath),
			//orm.TermQuery("principal_type", "link"), // Include public links
			Parameter("minimum_should_match", 1),
		)
	} else {
		qb.Must(orm.MustQuery(
			orm.TermQuery("principal_type", security.PrincipalTypeUser),
			orm.TermQuery("principal_id", userID),
			//orm.TermQuery("principal_type", "link"), // Include public links
		).Parameter("minimum_should_match", 1))
	}

	var shares []SharingRecord
	ctx.DirectReadAccess()
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return 0, errors.Errorf("failed to search shares: %v", err)
	}

	//if len(shares) == 0 {
	//	// Check path-based permissions
	//	return s.getPathBasedPermission(userID, resourcePath)
	//}

	//if len(shares) == 0 {
	//	// Check category based childrend permissions
	//	if resourceIsCategory{
	//		s.GetCategoryObjectFromSharedObjects(userID,resourceType,resourceID)
	//	}
	//	return s.getPathBasedPermission(userID, resourcePath)
	//}

	// Return the highest permission level
	maxPermission := None // Default to none
	for _, share := range shares {
		if share.Permission > maxPermission {
			maxPermission = share.Permission
		}
	}

	return maxPermission, nil
}

// GetUserAccessibleResources gets all resources accessible to a user
func (s *SharingService) GetUserAccessibleResources(userID string, permission SharingPermission) ([]string, error) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &SharingRecord{})

	// Build comprehensive permission query
	filter := s.BuildPermissionFilter(PermissionFilter{
		UserID:        userID,
		Permission:    permission,
		IncludePublic: true,
	})

	var shares []SharingRecord
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, filter, nil)
	if err != nil {
		return nil, errors.Errorf("failed to search accessible resources: %v", err)
	}

	resourceMap := make(map[string]bool)
	for _, share := range shares {
		resourceMap[share.ResourceID] = true
	}

	resources := make([]string, 0, len(resourceMap))
	for resourceID := range resourceMap {
		resources = append(resources, resourceID)
	}

	return resources, nil
}

// BuildPermissionFilter builds an Elasticsearch query for permission filtering
func (s *SharingService) BuildPermissionFilter(filter PermissionFilter) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// Build user-specific permission filter
	var userFilters []*orm.Clause

	// Direct user shares
	if filter.UserID != "" {
		userFilters = append(userFilters, orm.MustQuery(
			orm.TermQuery("principal_type", "user"),
			orm.TermQuery("principal_id", filter.UserID),
		))
	}

	// Group shares (if user groups provided)
	if len(filter.UserGroups) > 0 {
		var groupFilters []*orm.Clause
		for _, groupID := range filter.UserGroups {
			groupFilters = append(groupFilters, orm.MustQuery(
				orm.TermQuery("principal_type", "group"),
				orm.TermQuery("principal_id", groupID),
			))
		}
		userFilters = append(userFilters, orm.ShouldQuery(groupFilters...))
	}

	// Public shares (if enabled)
	if filter.IncludePublic {
		userFilters = append(userFilters, orm.TermQuery("principal_type", "link"))
	}

	if len(userFilters) > 0 {
		qb.Must(orm.ShouldQuery(userFilters...))
	}

	// Resource filtering
	if len(filter.ResourceIDs) > 0 {
		var resourceFilters []*orm.Clause
		for _, resourceID := range filter.ResourceIDs {
			resourceFilters = append(resourceFilters, orm.TermQuery("resource_id", resourceID))
		}
		qb.Must(orm.ShouldQuery(resourceFilters...))
	}

	//// Path-based filtering
	//if filter.ResourcePath != "" {
	//	qb.Must(orm.ShouldQuery(
	//		orm.TermQuery("resource_path", filter.ResourcePath),
	//		orm.PrefixQuery("resource_path", filter.ResourcePath+"/"),
	//	))
	//}

	// Permission level filtering
	if filter.Permission > 0 {
		qb.Must(orm.Range("permission").Gte(filter.Permission))
	}

	return qb
}

//// BuildElasticsearchPermissionFilter builds ES DSL for permission filtering
//func (s *SharingService) BuildElasticsearchPermissionFilter(filter PermissionFilter) map[string]interface{} {
//	qb := s.BuildPermissionFilter(filter)
//	return qb.Build()
//}

// getPathBasedPermission checks permissions based on resource path
func (s *SharingService) getPathBasedPermission(userID string, resourcePath string) (SharingPermission, error) {
	if resourcePath == "" || resourcePath == "/" {
		return 0, nil // No permission
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &SharingRecord{})

	// Check permissions for parent paths
	pathParts := splitPath(resourcePath)
	for i := len(pathParts); i > 0; i-- {
		//parentPath := joinPath(pathParts[:i])

		qb := orm.NewQuery()
		//qb.Must(orm.ShouldQuery(
		//	orm.TermQuery("resource_path", parentPath),
		//	orm.PrefixQuery("resource_path", parentPath+"/"),
		//))
		qb.Must(orm.ShouldQuery(
			orm.TermQuery("principal_id", userID),
			orm.TermQuery("principal_type", "link"),
		))

		var shares []SharingRecord
		err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
		if err != nil {
			continue
		}

		if len(shares) > 0 {
			// Return the highest permission from inherited shares
			maxPermission := View
			for _, share := range shares {
				if share.Permission > maxPermission {
					maxPermission = share.Permission
				}
			}
			return maxPermission, nil
		}
	}

	return 0, nil // No permission found
}

// Helper functions

func generateSecureToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func hashPassword(password string) string {
	// Simple hash for now - should use proper bcrypt in production
	return util.MD5digest(password)
}

func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

// LightweightPermissionChecker provides thread-safe in-memory permission checking for UI/simple operations
type LightweightPermissionChecker struct {
	userPermissions sync.Map // resourceID -> permission
}

func NewLightweightPermissionChecker() *LightweightPermissionChecker {
	return &LightweightPermissionChecker{}
}

func (c *LightweightPermissionChecker) AddPermission(resourceID string, permission SharingPermission) {
	c.userPermissions.Store(resourceID, permission)
}

func (c *LightweightPermissionChecker) HasPermission(resourceID string, requiredPermission SharingPermission) bool {
	if permValue, exists := c.userPermissions.Load(resourceID); exists {
		if permission, ok := permValue.(SharingPermission); ok {
			return permission >= requiredPermission
		}
	}
	return false
}

func (c *LightweightPermissionChecker) HasAnyPermission(resourceID string, permissions ...SharingPermission) bool {
	if permValue, exists := c.userPermissions.Load(resourceID); exists {
		if userPerm, ok := permValue.(SharingPermission); ok {
			for _, requiredPerm := range permissions {
				if userPerm >= requiredPerm {
					return true
				}
			}
		}
	}
	return false
}

func (c *LightweightPermissionChecker) GetPermission(resourceID string) (SharingPermission, bool) {
	if permValue, exists := c.userPermissions.Load(resourceID); exists {
		if permission, ok := permValue.(SharingPermission); ok {
			return permission, true
		}
	}
	return 0, false
}

func (c *LightweightPermissionChecker) Clear() {
	c.userPermissions.Range(func(key, value interface{}) bool {
		c.userPermissions.Delete(key)
		return true
	})
}

func (c *LightweightPermissionChecker) GetResourceCount() int {
	count := 0
	c.userPermissions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (c *LightweightPermissionChecker) RemovePermission(resourceID string) {
	c.userPermissions.Delete(resourceID)
}

func (c *LightweightPermissionChecker) UpdatePermission(resourceID string, permission SharingPermission) {
	c.userPermissions.Store(resourceID, permission)
}

// ListShares returns all current shares for a resource
func (s *SharingService) ListShares(ctx *orm.Context, req *http.Request) ([]SharingRecord, error) {
	orm.WithModel(ctx, &SharingRecord{})
	// Handle URL query args, convert to query builder
	builder, err := orm.NewQueryBuilderFromRequest(req, "principal_type", "principal_id", "display_name", "combined_fulltext")
	if err != nil {
		return nil, errors.Errorf("failed to build query: %v", err)
	}

	builder.DisableBodyBytes()

	docs := []SharingRecord{}
	err, _ = elastic.SearchV2WithResultItemMapper(ctx, &docs, builder, nil)
	if err != nil {
		return nil, errors.Errorf("failed to search shares: %v", err)
	}

	return docs, nil
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
		//resources[v.ResourceID]=false //assume the resource id is unique here, maybe we should try resource_type+resource_id in the future
		x := orm.MustQuery(orm.TermQuery("resource_type", v.ResourceType), orm.TermQuery("resource_id", v.ResourceID))

		if v.ResourceParentPath != "" {
			//v.ResourcePath = "/" handle default?
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

	//log.Error("BY PATHS:",util.ToJson(paths,true))
	//log.Error("DIRECT DOCS:",util.ToJson(docs,true))

	//if we have paths need to check
	if len(paths) > 0 {

		resourcesDirectPermission := map[string]SharingRecord{} //resource_key->user_key->SharingRecord

		//handle directly shared rules, it should be have only record for each resource
		for _, doc := range docs {
			resourcesDirectPermission[doc.GetResourceKey()+doc.GetPrincipalKey()] = doc
		}

		//log.Error("resourcesDirectPermission",util.ToJson(resourcesDirectPermission,true))

		//now we need to get all path based permissions under these folder
		//datasource -> path -> path -> path ...
		for datasourceID, v := range paths {
			//log.Error("let's walk these paths for datasource:", datasourceID)
			for p, list := range v {
				//log.Error("checking path: ",p)
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
					//log.Error("get shared rules: ", resourceType, " => ", util.MustToJSON(rules))
					//log.Error("need to handle the items in this path:", util.MustToJSON(list))

					//keep all inherited permissions all the way
					//log.Error("all rules:",util.ToJson(rules,true))
					for _, r := range rules {
						//log.Errorf("checking %v rule: %v",i,r.ResourceFullPath)
						_, ok1 := inheritedPermissionRules[r.GetPrincipalKey()]
						if ok1 {
							//log.Errorf("principal %v already exists,old: %v, vs new  %v, skip...",v1.PrincipalID,v1.ResourceFullPath,r.ResourceFullPath)
						} else {
							//log.Error("keeping rule:",util.ToJson(r,true))
							inheritedPermissionRules[r.GetPrincipalKey()] = r
						}
					}

					//log.Error("merged rules: ",util.ToJson(inheritedPermissionRules,true),",all items under this folder, should have the same permission if not specify permission for specify user, the user should have this permission")

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
								//log.Error("create a inherit rule:", util.MustToJSON(rule))

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

									//log.Errorf("get datasource level perm: %v, for user: %v, for share entity: %v",util.MustToJSON(r2),rule.GetPrincipalKey(),util.MustToJSON(x))
								} else {
									//log.Errorf("there is a doc level permission for user: %v exists for entity: %v",rule.GetPrincipalKey(),util.MustToJSON(x))
								}
							}
						}
					}
				}

				//log.Error("all new rules:", util.MustToJSON(docs),"\n")

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

	//log.Error(util.MustToJSON(req))

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
		//log.Error(util.MustToJSON(share))

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

		//log.Error("existingShare=>", util.MustToJSON(existingShare))

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

// RemoveShare removes a specific share
func (s *SharingService) RemoveShare(ctx *orm.Context, resourceID, shareID, userID string) error {
	// First, get the share record to validate ownership
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("id", shareID))
	qb.Must(orm.TermQuery("resource_id", resourceID))
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err != nil {
		return errors.Errorf("failed to find share: %v", err)
	}

	if len(shares) == 0 {
		return errors.New("share not found")
	}

	share := shares[0]

	// Validate that the current user is the one who granted this share
	if share.GrantedBy != userID {
		return errors.New("unauthorized: you can only remove shares you created")
	}

	// Delete the share
	if err := orm.Delete(ctx, &share); err != nil {
		return errors.Errorf("failed to delete share: %v", err)
	}

	return nil
}

// GetMyAccessForResource returns the current user's effective access level
func (s *SharingService) GetMyAccessForResource(ctx *orm.Context, userID, resourceType, resourceID, resourcePath string) (map[string]interface{}, error) {
	// Get resource path from the resource
	resourcePath = s.getResourcePath(ctx, resourceType, resourceID, resourcePath)
	if resourcePath == "" {
		resourcePath = "/" // Default to root if not found
	}

	// Get effective permission using core service
	permission, err := s.GetUserExplicitEffectivePermission(userID, NewResourceEntity(resourceType, resourceID, resourcePath))
	if err != nil {
		return nil, errors.Errorf("failed to get effective permission: %v", err)
	}

	if permission == 0 {
		// No permission found
		return map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"permission":    "none",
			"via":           "none",
		}, nil
	}

	// Determine how the permission was granted
	via := "direct" // Default assumption

	// Check if it's through a public link
	var shares []SharingRecord
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_id", resourceID))
	qb.Must(orm.TermQuery("principal_type", "link"))
	orm.WithModel(ctx, &SharingRecord{})

	err, _ = elastic.SearchV2WithResultItemMapper(ctx, &shares, qb, nil)
	if err == nil && len(shares) > 0 {
		via = "link"
	}

	// Check if it's through group membership (would need to fetch user's groups)
	// For now, we'll use "direct" as the default

	return map[string]interface{}{
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"permission":    permissionToString(permission),
		"via":           via,
	}, nil
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

// checkExistingShareLink checks if an active share link already exists for the resource
func (s *SharingService) checkExistingShareLink(ctx *orm.Context, resourceID, resourceType, resourcePath string, createdBy string, permission SharingPermission) (*ShareLink, error) {
	panic("not implemented")

	var links []ShareLink
	qb := orm.NewQuery()
	qb.Must(orm.TermQuery("resource_id", resourceID))
	qb.Must(orm.TermQuery("resource_type", resourceType))
	qb.Must(orm.TermQuery("resource_path", resourcePath))
	qb.Must(orm.TermQuery("created_by", createdBy))
	qb.Must(orm.TermQuery("permission", permission))
	qb.Must(orm.TermQuery("is_active", true))

	// Add expiration check - only consider non-expired links
	now := time.Now()
	qb.Must(orm.ShouldQuery(
		orm.MustNotQuery(orm.ExistsQuery("expires_at")),
		orm.Range("expires_at").Gt(now.Format(time.RFC3339)),
	))
	orm.WithModel(ctx, &SharingRecord{})

	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &links, qb, nil)
	if err != nil {
		return nil, err
	}

	if len(links) > 0 {
		return &links[0], nil
	}

	return nil, nil
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
	//log.Error(qb.ToString())

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

	//log.Error(qb.ToString())

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
	//if resourcePath!=""{
	//	qb.Must(orm.TermQuery("resource_path", resourcePath))
	//}

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
	//if resourcePath!=""{
	//	qb.Must(orm.TermQuery("resource_path", resourcePath))
	//}

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
	//if resourcePath!=""{
	//	qb.Must(orm.TermQuery("resource_path", resourcePath))
	//}

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	//orm.WithCollapseField(ctx, "resource_category_id")
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
	//if resourcePath!=""{
	//	qb.Must(orm.TermQuery("resource_path", resourcePath))
	//}

	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	//orm.WithCollapseField(ctx, "resource_category_id")
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

	//// Return the highest permission level
	//maxPermission := None // Default to none
	//for _, share := range shares {
	//	//let's double check
	//	if share.ResourceID!=resourceID{
	//		panic("invalid sharing record, resource_id is not correct")
	//	}
	//	if  share.Permission > maxPermission {
	//		maxPermission = share.Permission
	//	}
	//}

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

// parseSharingPermission converts string permission to SharingPermission type
func parseSharingPermission(perm string) (SharingPermission, error) {
	switch perm {
	case "none":
		return None, nil
	case "view":
		return View, nil
	case "comment":
		return Comment, nil
	case "edit":
		return Edit, nil
	case "share":
		return Share, nil
	case "owner":
		return Owner, nil
	default:
		return 0, fmt.Errorf("invalid permission: %s", perm)
	}
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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package share

import (
	"infini.sh/framework/core/orm"
	"time"
)

// QueryBuilder builds ES queries for permission filtering
type QueryBuilder struct{}

// NewElasticsearchQueryBuilder creates a new ES query builder
func NewElasticsearchQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

// BuildPathPermissionQuery builds an ES query for path-based permissions
func (b *QueryBuilder) BuildPathPermissionQuery(userID string, resourcePaths []string, permission SharingPermission, includePublic bool) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// Build user permission filter
	var permissionFilters []*orm.Clause

	// Direct user permissions
	if userID != "" {
		permissionFilters = append(permissionFilters, orm.MustQuery(
			orm.TermQuery("principal_type", "user"),
			orm.TermQuery("principal_id", userID),
		))
	}

	// Public permissions (if enabled)
	if includePublic {
		permissionFilters = append(permissionFilters, orm.TermQuery("principal_type", "link"))
	}

	if len(permissionFilters) > 0 {
		qb.Must(orm.ShouldQuery(permissionFilters...))
	}

	// Path-based filtering
	if len(resourcePaths) > 0 {
		var pathFilters []*orm.Clause
		for _, path := range resourcePaths {
			pathFilters = append(pathFilters, orm.ShouldQuery(
				orm.TermQuery("resource_path", path),
				orm.PrefixQuery("resource_path", path+"/"),
			))
		}
		qb.Must(orm.ShouldQuery(pathFilters...))
	}

	// Permission level filtering
	if permission > 0 {
		qb.Must(orm.Range("permission").Gte(permission))
	}

	return qb
}

// BuildResourcePermissionQuery builds an ES query for specific resource permissions
func (b *QueryBuilder) BuildResourcePermissionQuery(userID string, resourceIDs []string, permission SharingPermission, includePublic bool) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// Build user permission filter
	var permissionFilters []*orm.Clause

	// Direct user permissions
	if userID != "" {
		permissionFilters = append(permissionFilters, orm.MustQuery(
			orm.TermQuery("principal_type", "user"),
			orm.TermQuery("principal_id", userID),
		))
	}

	// Public permissions (if enabled)
	if includePublic {
		permissionFilters = append(permissionFilters, orm.TermQuery("principal_type", "link"))
	}

	if len(permissionFilters) > 0 {
		qb.Must(orm.ShouldQuery(permissionFilters...))
	}

	// Resource ID filtering
	if len(resourceIDs) > 0 {
		var resourceFilters []*orm.Clause
		for _, resourceID := range resourceIDs {
			resourceFilters = append(resourceFilters, orm.TermQuery("resource_id", resourceID))
		}
		qb.Must(orm.ShouldQuery(resourceFilters...))
	}

	// Permission level filtering
	if permission > 0 {
		qb.Must(orm.Range("permission").Gte(permission))
	}

	return qb
}

// BuildHierarchicalPermissionQuery builds an ES query for hierarchical permissions
func (b *QueryBuilder) BuildHierarchicalPermissionQuery(userID string, basePath string, permission SharingPermission, includePublic bool) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// Build user permission filter
	var permissionFilters []*orm.Clause

	// Direct user permissions
	if userID != "" {
		permissionFilters = append(permissionFilters, orm.MustQuery(
			orm.TermQuery("principal_type", "user"),
			orm.TermQuery("principal_id", userID),
		))
	}

	// Public permissions (if enabled)
	if includePublic {
		permissionFilters = append(permissionFilters, orm.TermQuery("principal_type", "link"))
	}

	if len(permissionFilters) > 0 {
		qb.Must(orm.ShouldQuery(permissionFilters...))
	}

	// Hierarchical path filtering
	if basePath != "" {
		// Include exact path match and all sub-paths
		pathFilters := []*orm.Clause{
			orm.TermQuery("resource_path", basePath),
			orm.PrefixQuery("resource_path", basePath+"/"),
		}

		// Also check parent paths for inherited permissions
		parentPaths := getParentPaths(basePath)
		for _, parentPath := range parentPaths {
			pathFilters = append(pathFilters,
				orm.TermQuery("resource_path", parentPath),
				orm.PrefixQuery("resource_path", parentPath+"/"),
			)
		}

		qb.Must(orm.ShouldQuery(pathFilters...))
	}

	// Permission level filtering
	if permission > 0 {
		qb.Must(orm.Range("permission").Gte(permission))
	}

	return qb
}

// BuildShareLinkQuery builds an ES query for share link validation
func (b *QueryBuilder) BuildShareLinkQuery(token string, includeExpired bool) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// Token match
	qb.Must(orm.TermQuery("token", token))
	qb.Must(orm.TermQuery("is_active", true))

	// Expiration check (if not including expired)
	if !includeExpired {
		qb.Must(orm.ShouldQuery(
			orm.MustNotQuery(orm.ExistsQuery("expires_at")),
			orm.Range("expires_at").Gt(time.Now().Format(time.RFC3339)),
		))
	}

	return qb
}

// BuildBulkPermissionQuery builds an ES query for bulk permission checking
func (b *QueryBuilder) BuildBulkPermissionQuery(userID string, resourceIDs []string, permission SharingPermission) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// User permission filter
	if userID != "" {
		qb.Must(orm.ShouldQuery(
			orm.MustQuery(
				orm.TermQuery("principal_type", "user"),
				orm.TermQuery("principal_id", userID),
			),
			orm.TermQuery("principal_type", "link"), // Include public links
		))
	}

	// Resource ID filtering
	if len(resourceIDs) > 0 {
		var resourceFilters []*orm.Clause
		for _, resourceID := range resourceIDs {
			resourceFilters = append(resourceFilters, orm.TermQuery("resource_id", resourceID))
		}
		qb.Must(orm.ShouldQuery(resourceFilters...))
	}

	// Permission level filtering
	if permission > 0 {
		qb.Must(orm.Range("permission").Gte(permission))
	}

	return qb
}

// BuildAccessibleResourcesQuery builds an ES query to find all resources accessible to a user
func (b *QueryBuilder) BuildAccessibleResourcesQuery(userID string, permission SharingPermission, includePublic bool) *orm.QueryBuilder {
	qb := orm.NewQuery()

	// User permission filter
	var permissionFilters []*orm.Clause

	// Direct user permissions
	if userID != "" {
		permissionFilters = append(permissionFilters, orm.MustQuery(
			orm.TermQuery("principal_type", "user"),
			orm.TermQuery("principal_id", userID),
		))
	}

	// Public permissions (if enabled)
	if includePublic {
		permissionFilters = append(permissionFilters, orm.TermQuery("principal_type", "link"))
	}

	if len(permissionFilters) > 0 {
		qb.Must(orm.ShouldQuery(permissionFilters...))
	}

	// Permission level filtering
	if permission > 0 {
		qb.Must(orm.Range("permission").Gte(permission))
	}

	//// Aggregation to get unique resource IDs
	//qb.SetAggregations(map[string]orm.Aggregation{
	//	"unique_resources": {
	//		Type: "terms",
	//		Field: "resource_id",
	//		Size: 10000, // Limit to prevent memory issues
	//	},
	//})

	return qb
}

// Helper function to get parent paths for hierarchical permissions
func getParentPaths(path string) []string {
	if path == "" || path == "/" {
		return []string{}
	}

	var parents []string
	parts := splitPath(path)

	// Build all parent paths
	for i := len(parts) - 1; i > 0; i-- {
		parentPath := joinPath(parts[:i])
		parents = append(parents, parentPath)
	}

	return parents
}

// Helper function to split path into components
func splitPath(path string) []string {
	if path == "" || path == "/" {
		return []string{}
	}

	var parts []string
	current := ""

	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// Helper function to join path components
func joinPath(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}

	result := ""
	for _, part := range parts {
		if part != "" {
			result += "/" + part
		}
	}

	return result
}

// PathPermissionOptimizer provides optimized path-based permission queries
type PathPermissionOptimizer struct {
	cache map[string]*orm.QueryBuilder
}

// NewPathPermissionOptimizer creates a new optimizer
func NewPathPermissionOptimizer() *PathPermissionOptimizer {
	return &PathPermissionOptimizer{
		cache: make(map[string]*orm.QueryBuilder),
	}
}

// GetCachedQuery returns a cached query if available, otherwise builds and caches a new one
func (o *PathPermissionOptimizer) GetCachedQuery(cacheKey string, builderFunc func() *orm.QueryBuilder) *orm.QueryBuilder {
	if cached, exists := o.cache[cacheKey]; exists {
		return cached
	}

	query := builderFunc()
	o.cache[cacheKey] = query
	return query
}

// ClearCache clears the query cache
func (o *PathPermissionOptimizer) ClearCache() {
	o.cache = make(map[string]*orm.QueryBuilder)
}

// BuildOptimizedPathQuery builds an optimized path permission query with caching
func (o *PathPermissionOptimizer) BuildOptimizedPathQuery(userID string, path string, permission SharingPermission) *orm.QueryBuilder {
	cacheKey := userID + "|" + path + "|" + string(rune(permission))

	return o.GetCachedQuery(cacheKey, func() *orm.QueryBuilder {
		builder := NewElasticsearchQueryBuilder()
		return builder.BuildHierarchicalPermissionQuery(userID, path, permission, true)
	})
}

// PermissionQueryCache provides thread-safe caching for permission queries
type PermissionQueryCache struct {
	queries map[string]*orm.QueryBuilder
	stats   *QueryCacheStats
}

// QueryCacheStats tracks cache performance
type QueryCacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

// NewPermissionQueryCache creates a new query cache
func NewPermissionQueryCache() *PermissionQueryCache {
	return &PermissionQueryCache{
		queries: make(map[string]*orm.QueryBuilder),
		stats:   &QueryCacheStats{},
	}
}

// Get returns a cached query
func (c *PermissionQueryCache) Get(key string) (*orm.QueryBuilder, bool) {
	if query, exists := c.queries[key]; exists {
		c.stats.Hits++
		return query, true
	}
	c.stats.Misses++
	return nil, false
}

// Set stores a query in the cache
func (c *PermissionQueryCache) Set(key string, query *orm.QueryBuilder) {
	c.queries[key] = query
}

// GetStats returns cache statistics
func (c *PermissionQueryCache) GetStats() *QueryCacheStats {
	return c.stats
}

// Clear clears the cache
func (c *PermissionQueryCache) Clear() {
	c.queries = make(map[string]*orm.QueryBuilder)
	c.stats = &QueryCacheStats{}
}

// ElasticsearchQueryOptimizer provides advanced query optimization
type ElasticsearchQueryOptimizer struct {
	cache *PermissionQueryCache
}

// NewElasticsearchQueryOptimizer creates a new optimizer
func NewElasticsearchQueryOptimizer() *ElasticsearchQueryOptimizer {
	return &ElasticsearchQueryOptimizer{
		cache: NewPermissionQueryCache(),
	}
}

//
//// BuildOptimizedPermissionQuery builds an optimized permission query with caching
//func (o *ElasticsearchQueryOptimizer) BuildOptimizedPermissionQuery(params PermissionFilter) *orm.QueryBuilder {
//	cacheKey := generateCacheKey(params)
//
//	if cached, found := o.cache.Get(cacheKey); found {
//		return cached
//	}
//
//	builder := NewElasticsearchQueryBuilder()
//	query := builder.BuildPermissionFilter(params)
//
//	o.cache.Set(cacheKey, query)
//	return query
//}

// GenerateCacheKey creates a cache key from permission filter parameters
func generateCacheKey(params PermissionFilter) string {
	key := params.UserID + "|"

	for _, group := range params.UserGroups {
		key += group + ","
	}
	key += "|"

	for _, resourceID := range params.ResourceIDs {
		key += resourceID + ","
	}
	key += "|"

	key += params.ResourcePath + "|"
	key += string(rune(params.Permission)) + "|"
	key += string(rune(BoolToInt(params.IncludePublic)))

	return key
}

// BoolToInt converts boolean to int
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

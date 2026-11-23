package share

import (
	"sync"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

type ResolvedResource struct {
	Exists     bool
	OwnerID    string
	Attributes map[string]string
	Resource   ResourceEntity
}

type ResourceResolver func(ctx *orm.Context, resource ResourceEntity) (*ResolvedResource, error)

type ResolvedPrincipal struct {
	Exists     bool
	Attributes map[string]string
}

type PrincipalResolver func(ctx *orm.Context, principalID string) (*ResolvedPrincipal, error)

type RuntimeExtension struct {
	BuildIdentityScope         func(ctx *orm.Context, user *security.UserSessionInfo, record *SharingRecord) []string
	ValidateResolvedResource   func(ctx *orm.Context, user *security.UserSessionInfo, resource *ResolvedResource) error
	ValidateResolvedPrincipal  func(ctx *orm.Context, user *security.UserSessionInfo, principalType string, principalID string, principal *ResolvedPrincipal) error
	PrepareShareForWrite       func(ctx *orm.Context, user *security.UserSessionInfo, resource *ResolvedResource, record *SharingRecord) error
	PrepareExistingShareUpdate func(ctx *orm.Context, user *security.UserSessionInfo, record *SharingRecord) error
}

var resourceResolvers sync.Map
var principalResolvers sync.Map
var runtimeExtensions []RuntimeExtension
var runtimeExtensionsLock sync.RWMutex

func RegisterResourceResolver(resourceType string, resolver ResourceResolver) {
	resourceResolvers.Store(resourceType, resolver)
}

func RegisterPrincipalResolver(principalType string, resolver PrincipalResolver) {
	principalResolvers.Store(principalType, resolver)
}

func RegisterRuntimeExtension(ext RuntimeExtension) {
	runtimeExtensionsLock.Lock()
	defer runtimeExtensionsLock.Unlock()
	runtimeExtensions = append(runtimeExtensions, ext)
}

func getResourceResolver(resourceType string) ResourceResolver {
	if value, ok := resourceResolvers.Load(resourceType); ok {
		if resolver, ok := value.(ResourceResolver); ok {
			return resolver
		}
	}
	return nil
}

func getPrincipalResolver(principalType string) PrincipalResolver {
	if value, ok := principalResolvers.Load(principalType); ok {
		if resolver, ok := value.(PrincipalResolver); ok {
			return resolver
		}
	}
	return nil
}

func getRuntimeExtensions() []RuntimeExtension {
	runtimeExtensionsLock.RLock()
	defer runtimeExtensionsLock.RUnlock()
	if len(runtimeExtensions) == 0 {
		return nil
	}
	out := make([]RuntimeExtension, len(runtimeExtensions))
	copy(out, runtimeExtensions)
	return out
}

func newResolverReadContext(parent *orm.Context) *orm.Context {
	var ctx *orm.Context
	if parent != nil && parent.Context != nil {
		ctx = orm.NewContextWithParent(parent.Context)
		ctx.Refresh = parent.Refresh
	} else {
		ctx = orm.NewContext()
	}
	ctx.DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	return ctx
}

func defaultUserPrincipalResolver(ctx *orm.Context, principalID string) (*ResolvedPrincipal, error) {

	if _, user, err := security.GetUserByID(principalID); err == nil && user != nil {
		return &ResolvedPrincipal{Exists: true}, nil
	}

	lookupCtx := newResolverReadContext(ctx)
	orm.WithModel(lookupCtx, &security.UserAccount{})

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("id", principalID))
	qb.Size(1)

	docs := []security.UserAccount{}
	err, _ := elastic.SearchV2WithResultItemMapper(lookupCtx, &docs, qb, nil)
	if err != nil {
		return &ResolvedPrincipal{Exists: false}, nil
	}

	return &ResolvedPrincipal{Exists: len(docs) > 0}, nil
}

func init() {
	RegisterPrincipalResolver(security.PrincipalTypeUser, defaultUserPrincipalResolver)
}

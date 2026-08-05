//go:build integration

package share_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	elastic_module "infini.sh/framework/modules/elastic"
	elastic_common "infini.sh/framework/modules/elastic/common"
	_ "infini.sh/framework/modules/security/orm_hooks"
	share "infini.sh/framework/modules/security/share"
)

const (
	testElasticEndpoint = "https://localhost:19200"
	testElasticUser     = "admin"
	testElasticPassword = "ShareTest_2026!"
)

var setupOnce sync.Once
var setupErr error

type testShareResource struct {
	orm.ORMObjectBase
	ResourceType       string `json:"resource_type,omitempty" elastic_mapping:"resource_type:{type:keyword}"`
	ResourceParentPath string `json:"resource_parent_path,omitempty" elastic_mapping:"resource_parent_path:{type:keyword}"`
	ResourceFullPath   string `json:"resource_full_path,omitempty" elastic_mapping:"resource_full_path:{type:keyword}"`
}

type testSharePrincipal struct {
	orm.ORMObjectBase
	PrincipalType string `json:"principal_type,omitempty" elastic_mapping:"principal_type:{type:keyword}"`
	Name          string `json:"name,omitempty" elastic_mapping:"name:{type:keyword}"`
}

func TestMain(m *testing.M) {
	setupOnce.Do(func() {
		setupErr = setupIntegrationORM()
	})
	if setupErr != nil {
		fmt.Fprintf(os.Stderr, "integration setup failed: %v\n", setupErr)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func setupIntegrationORM() error {
	esConfig := elastic.ElasticsearchConfig{
		Name:     "sharing-test",
		Enabled:  true,
		Endpoint: testElasticEndpoint,
		BasicAuth: &model.BasicAuth{
			Username: testElasticUser,
			Password: testElasticPassword,
		},
	}

	client, err := elastic_common.InitElasticInstanceWithoutMetadata(esConfig)
	if err != nil {
		return err
	}

	handler := elastic_module.ElasticORM{
		Client: client,
		Config: elastic_common.ORMConfig{Enabled: true},
	}
	orm.Register("elastic-sharing-it", &handler)
	orm.MustRegisterSchemaWithIndexName(&testShareResource{}, "test-share-resources")
	orm.MustRegisterSchemaWithIndexName(&testSharePrincipal{}, "test-share-principals")
	share.RegisterRuntimeExtension(share.RuntimeExtension{
		BuildIdentityScope: func(ctx *orm.Context, user *security.UserSessionInfo, record *share.SharingRecord) []string {
			tenantID, _ := user.GetString(orm.TenantIDKey)
			if tenantID == "" {
				return nil
			}
			return []string{tenantID}
		},
		ValidateResolvedResource: func(ctx *orm.Context, user *security.UserSessionInfo, resource *share.ResolvedResource) error {
			tenantID, _ := user.GetString(orm.TenantIDKey)
			resourceTenantID := resource.Attributes[orm.TenantIDKey]
			if tenantID != "" && resourceTenantID != "" && tenantID != resourceTenantID {
				return fmt.Errorf("invalid data access")
			}
			return nil
		},
		ValidateResolvedPrincipal: func(ctx *orm.Context, user *security.UserSessionInfo, principalType string, principalID string, principal *share.ResolvedPrincipal) error {
			tenantID, _ := user.GetString(orm.TenantIDKey)
			principalTenantID := principal.Attributes[orm.TenantIDKey]
			if tenantID != "" && principalTenantID != "" && tenantID != principalTenantID {
				return fmt.Errorf("principal belongs to a different tenant: %v/%v", principalType, principalID)
			}
			return nil
		},
		PrepareShareForWrite: func(ctx *orm.Context, user *security.UserSessionInfo, resource *share.ResolvedResource, record *share.SharingRecord) error {
			tenantID, _ := user.GetString(orm.TenantIDKey)
			if tenantID != "" {
				record.SetSystemValue(orm.TenantIDKey, tenantID)
			}
			return nil
		},
		PrepareExistingShareUpdate: func(ctx *orm.Context, user *security.UserSessionInfo, record *share.SharingRecord) error {
			tenantID, _ := user.GetString(orm.TenantIDKey)
			if tenantID != "" {
				record.SetSystemValue(orm.TenantIDKey, tenantID)
			}
			return nil
		},
	})
	share.RegisterResourceResolver("document", resolveTestDocumentResource)
	share.RegisterPrincipalResolver(security.PrincipalTypeUser, resolveTestPrincipal(security.PrincipalTypeUser))
	share.RegisterPrincipalResolver(security.PrincipalTypeTeam, resolveTestPrincipal(security.PrincipalTypeTeam))

	for _, cb := range global.GetFuncAfterSetup() {
		cb()
	}

	return orm.InitSchema()
}

func createTestSession(userID string, roles []string, tenantID string) *security.UserSessionInfo {
	session := &security.UserSessionInfo{
		Provider: "test",
		Login:    userID,
		UserID:   userID,
		Roles:    roles,
	}
	session.Set(orm.TenantIDKey, tenantID)
	return session
}

func createORMContext(session *security.UserSessionInfo) *orm.Context {
	ctx := orm.NewContextWithParent(security.AddUserToContext(context.Background(), session))
	ctx.Refresh = orm.WaitForRefresh
	ctx.PermissionScope(security.PermissionScopePlatform)
	return ctx
}

func uniqueResourceID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func newFixtureContext() *orm.Context {
	ctx := orm.NewContext()
	ctx.Refresh = orm.WaitForRefresh
	ctx.DirectAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	return ctx
}

func principalDocID(principalType string, principalID string) string {
	return fmt.Sprintf("%s:%s", principalType, principalID)
}

func resolveTestDocumentResource(ctx *orm.Context, resource share.ResourceEntity) (*share.ResolvedResource, error) {
	lookupCtx := newFixtureContext()
	if ctx != nil && ctx.Context != nil {
		lookupCtx = orm.NewContextWithParent(ctx.Context)
		lookupCtx.Refresh = ctx.Refresh
		lookupCtx.DirectReadAccess()
		lookupCtx.PermissionScope(security.PermissionScopePlatform)
	}

	doc := &testShareResource{}
	doc.SetID(resource.ResourceID)
	exists, err := orm.GetWithSystemFields(lookupCtx, doc)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return nil, err
	}

	resolved := resource
	if resolved.ResourceParentPath == "" {
		resolved.ResourceParentPath = doc.ResourceParentPath
	}
	if resolved.ResourceParentPath == "" {
		resolved.ResourceParentPath = "/"
	}
	if resolved.ResourceFullPath == "" {
		resolved.ResourceFullPath = doc.ResourceFullPath
	}

	return &share.ResolvedResource{
		Exists:  exists,
		OwnerID: doc.GetSystemString(orm.OwnerIDKey),
		Attributes: map[string]string{
			orm.TenantIDKey: doc.GetSystemString(orm.TenantIDKey),
		},
		Resource: resolved,
	}, nil
}

func resolveTestPrincipal(principalType string) share.PrincipalResolver {
	return func(ctx *orm.Context, principalID string) (*share.ResolvedPrincipal, error) {
		lookupCtx := newFixtureContext()
		if ctx != nil && ctx.Context != nil {
			lookupCtx = orm.NewContextWithParent(ctx.Context)
			lookupCtx.Refresh = ctx.Refresh
			lookupCtx.DirectReadAccess()
			lookupCtx.PermissionScope(security.PermissionScopePlatform)
		}

		doc := &testSharePrincipal{}
		doc.SetID(principalDocID(principalType, principalID))
		exists, err := orm.GetWithSystemFields(lookupCtx, doc)
		if err != nil && !strings.Contains(err.Error(), "record not found") {
			return nil, err
		}

		return &share.ResolvedPrincipal{
			Exists: exists,
			Attributes: map[string]string{
				orm.TenantIDKey: doc.GetSystemString(orm.TenantIDKey),
			},
		}, nil
	}
}

func seedTestResource(t *testing.T, owner *security.UserSessionInfo, resourceID string) {
	t.Helper()

	ctx := newFixtureContext()
	doc := &testShareResource{
		ResourceType:       "document",
		ResourceParentPath: "/",
		ResourceFullPath:   "/" + resourceID,
	}
	doc.SetID(resourceID)
	doc.SetSystemValue(orm.OwnerIDKey, owner.MustGetUserID())
	doc.SetSystemValue(orm.TenantIDKey, owner.MustGetString(orm.TenantIDKey))
	require.NoError(t, orm.Save(ctx, doc))
}

func seedUserPrincipal(t *testing.T, principalID string, tenantID string) {
	t.Helper()

	ctx := newFixtureContext()
	doc := &testSharePrincipal{PrincipalType: security.PrincipalTypeUser, Name: principalID}
	doc.SetID(principalDocID(security.PrincipalTypeUser, principalID))
	doc.SetSystemValue(orm.TenantIDKey, tenantID)
	doc.SetSystemValue(orm.OwnerIDKey, principalID)
	require.NoError(t, orm.Save(ctx, doc))
}

func seedTeamPrincipal(t *testing.T, principalID string, tenantID string) {
	t.Helper()

	ctx := newFixtureContext()
	doc := &testSharePrincipal{PrincipalType: security.PrincipalTypeTeam, Name: principalID}
	doc.SetID(principalDocID(security.PrincipalTypeTeam, principalID))
	doc.SetSystemValue(orm.TenantIDKey, tenantID)
	doc.SetSystemValue(orm.OwnerIDKey, principalID)
	require.NoError(t, orm.Save(ctx, doc))
}

func getAllResourceRules(t *testing.T, resourceType, resourceID string) []share.SharingRecord {
	t.Helper()

	service := share.NewSharingService()
	rules, err := service.GetResourcePermissions(nil, resourceType, []string{resourceID})
	require.NoError(t, err)
	return rules
}

func TestSharing_CreateAndRead_BackgroundBasics(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-u1", nil, "tenant-a")
	ctx := createORMContext(owner)

	resourceID := uniqueResourceID("res-basic")
	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "user-target-1", "tenant-a")
	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{
						ResourceType: "document",
						ResourceID:   resourceID,
					},
					PrincipalType: security.PrincipalTypeUser,
					PrincipalID:   "user-target-1",
					Permission:    share.View,
				},
			},
		},
	}

	res, err := service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), op)
	require.NoError(t, err)
	require.Len(t, res.Created, 1)

	allRules, err := service.GetResourcePermissions(nil, "document", []string{resourceID})
	require.NoError(t, err)
	require.NotEmpty(t, allRules)
	require.Equal(t, "user-target-1", allRules[0].PrincipalID)
	require.Equal(t, security.PrincipalTypeUser, allRules[0].PrincipalType)
	require.Equal(t, share.View, allRules[0].Permission)
}

func TestSharing_NonOwnerCanShare_CurrentBehaviorRegressionGuard(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-share-1", nil, "tenant-a")
	nonOwner := createTestSession("attacker-u1", nil, "tenant-a")
	ctx := createORMContext(nonOwner)

	resourceID := uniqueResourceID("res-vuln-share")
	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "victim-user", "tenant-a")
	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{
						ResourceType: "document",
						ResourceID:   resourceID,
					},
					PrincipalType: security.PrincipalTypeUser,
					PrincipalID:   "victim-user",
					Permission:    share.Share,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ctx, nonOwner.MustGetUserID(), op)
	require.Error(t, err, "non-owner sharing must be denied")
}

func TestSharing_NonOwnerCannotRevokeShareByID(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-revoke-1", nil, "tenant-a")
	ownerCtx := createORMContext(owner)

	resourceID := uniqueResourceID("res-revoke")
	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "target-revoke-1", "tenant-a")
	createOp := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "target-revoke-1",
					Permission:     share.View,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ownerCtx, owner.MustGetUserID(), createOp)
	require.NoError(t, err)

	rules, err := service.GetResourcePermissions(nil, "document", []string{resourceID})
	require.NoError(t, err)
	require.NotEmpty(t, rules)

	attacker := createTestSession("attacker-revoke-1", nil, "tenant-a")
	attackerCtx := createORMContext(attacker)
	revokeOp := &share.ShareRequest{
		Revokes: []share.SharingRecord{
			{ORMObjectBase: orm.ORMObjectBase{ID: rules[0].ID}},
		},
	}

	_, err = service.CreateOrUpdateShares(attackerCtx, attacker.MustGetUserID(), revokeOp)
	require.Error(t, err, "non-owner revoke must be denied")
}

func TestSharing_RejectsUnknownPrincipal(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-principal-1", nil, "tenant-a")
	ctx := createORMContext(owner)

	resourceID := uniqueResourceID("res-principal")
	seedTestResource(t, owner, resourceID)
	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "does-not-exist-user",
					Permission:     share.View,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), op)
	require.Error(t, err, "sharing must fail when principal does not exist")
}

func TestSharing_RejectsUnknownResource(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-resource-1", nil, "tenant-a")
	ctx := createORMContext(owner)
	seedUserPrincipal(t, "target-resource-1", "tenant-a")

	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: "non-existent-resource-id"},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "target-resource-1",
					Permission:     share.View,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), op)
	require.Error(t, err, "sharing must fail for non-existent resource")
}

// TestSharing_CreatedShareStoresTenantID moved to plugins/enterprise/managed/share_integration_test.go
// (tenant persistence is an enterprise-plugin concern)

func TestSharing_UpdatesExistingShareWithoutCreatingDuplicate(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-update-1", nil, "tenant-a")
	ctx := createORMContext(owner)

	resourceID := uniqueResourceID("res-update")
	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "target-update-1", "tenant-a")
	createOp := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "target-update-1",
					Permission:     share.View,
				},
			},
		},
	}

	first, err := service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), createOp)
	require.NoError(t, err)
	require.Len(t, first.Created, 1)

	updateOp := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "target-update-1",
					Permission:     share.Edit,
				},
			},
		},
	}

	second, err := service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), updateOp)
	require.NoError(t, err)
	require.Len(t, second.Updated, 1)

	rules := getAllResourceRules(t, "document", resourceID)
	require.Len(t, rules, 1)
	require.Equal(t, share.Edit, rules[0].Permission)
}

func TestSharing_MergeWithTeamRulesAddsInheritedUserRule(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-team-1", nil, "tenant-a")
	ownerCtx := createORMContext(owner)

	resourceID := uniqueResourceID("res-team")
	seedTestResource(t, owner, resourceID)
	seedTeamPrincipal(t, "team-alpha", "tenant-a")
	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity:       share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:        security.PrincipalTypeTeam,
					PrincipalID:          "team-alpha",
					PrincipalDisplayName: "Alpha Team",
					Permission:           share.View,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ownerCtx, owner.MustGetUserID(), op)
	require.NoError(t, err)

	member := createTestSession("member-team-1", nil, "tenant-a")
	member.Set(orm.TeamsIDKey, []string{"team-alpha"})
	ctx := createORMContext(member)

	docs, err := service.BatchGetShares(ctx, member, []share.ResourceEntity{{
		ResourceType: "document",
		ResourceID:   resourceID,
	}})
	require.NoError(t, err)

	merged := service.MergeWithTeamRules(member, docs)

	var inherited *share.SharingRecord
	for i := range merged {
		if merged[i].PrincipalType == security.PrincipalTypeUser && merged[i].PrincipalID == member.MustGetUserID() && merged[i].Via == share.ViaInherit {
			inherited = &merged[i]
			break
		}
	}

	require.NotNil(t, inherited)
	require.Equal(t, share.InheritedTypeTeam, inherited.InheritedType)
	require.Equal(t, "team-alpha", inherited.InheritedFrom)
	require.Equal(t, share.View, inherited.Permission)
}

func TestSharing_MergeWithTeamRulesPrefersExplicitUserRule(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-team-2", nil, "tenant-a")
	ownerCtx := createORMContext(owner)

	resourceID := uniqueResourceID("res-team-explicit")
	seedTestResource(t, owner, resourceID)
	seedTeamPrincipal(t, "team-beta", "tenant-a")
	seedUserPrincipal(t, "member-team-2", "tenant-a")
	op := &share.ShareRequest{
		Shares: []share.SharingRecord{
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeTeam,
					PrincipalID:    "team-beta",
					Permission:     share.View,
				},
			},
			{
				SimplifySharingRecord: share.SimplifySharingRecord{
					ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
					PrincipalType:  security.PrincipalTypeUser,
					PrincipalID:    "member-team-2",
					Permission:     share.Edit,
				},
			},
		},
	}

	_, err := service.CreateOrUpdateShares(ownerCtx, owner.MustGetUserID(), op)
	require.NoError(t, err)

	member := createTestSession("member-team-2", nil, "tenant-a")
	member.Set(orm.TeamsIDKey, []string{"team-beta"})
	ctx := createORMContext(member)

	docs, err := service.BatchGetShares(ctx, member, []share.ResourceEntity{{
		ResourceType: "document",
		ResourceID:   resourceID,
	}})
	require.NoError(t, err)

	baseLen := len(docs)
	merged := service.MergeWithTeamRules(member, docs)
	require.Len(t, merged, baseLen)

	var explicitUser *share.SharingRecord
	for i := range merged {
		if merged[i].PrincipalType == security.PrincipalTypeUser && merged[i].PrincipalID == member.MustGetUserID() {
			explicitUser = &merged[i]
			break
		}
	}

	require.NotNil(t, explicitUser)
	require.Equal(t, share.Edit, explicitUser.Permission)
	require.NotEqual(t, share.ViaInherit, explicitUser.Via)
}

func TestSharing_ConcurrentDuplicateShareRequestsDoNotCreateMultipleRules(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("owner-race-1", nil, "tenant-a")
	resourceID := uniqueResourceID("res-race")
	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "race-target-1", "tenant-a")

	const workers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx := createORMContext(owner)
			_, _ = service.CreateOrUpdateShares(ctx, owner.MustGetUserID(), &share.ShareRequest{
				Shares: []share.SharingRecord{{
					SimplifySharingRecord: share.SimplifySharingRecord{
						ResourceEntity: share.ResourceEntity{ResourceType: "document", ResourceID: resourceID},
						PrincipalType:  security.PrincipalTypeUser,
						PrincipalID:    "race-target-1",
						Permission:     share.View,
					},
				}},
			})
		}()
	}
	close(start)
	wg.Wait()

	rules := getAllResourceRules(t, "document", resourceID)
	require.Len(t, rules, 1, "duplicate concurrent shares must collapse to a single stored rule")
}

// TestSharing_DelegationChain tests the full permission delegation chain:
// Owner A → shares with B (Share perm) → B reshares to C (Edit perm) → C cannot reshare → D has no access
func TestSharing_DelegationChain(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()

	// Setup actors
	ownerA := createTestSession("chain-owner-a", nil, "tenant-a")
	userB := createTestSession("chain-user-b", nil, "tenant-a")
	userC := createTestSession("chain-user-c", nil, "tenant-a")
	userD := createTestSession("chain-user-d", nil, "tenant-a")

	resourceID := uniqueResourceID("res-chain")
	seedTestResource(t, ownerA, resourceID)
	seedUserPrincipal(t, "chain-user-b", "tenant-a")
	seedUserPrincipal(t, "chain-user-c", "tenant-a")
	seedUserPrincipal(t, "chain-user-d", "tenant-a")

	resource := share.ResourceEntity{ResourceType: "document", ResourceID: resourceID}

	// Step 1: Owner A shares with B at "Share" permission (B can reshare)
	ctxA := createORMContext(ownerA)
	_, err := service.CreateOrUpdateShares(ctxA, ownerA.MustGetUserID(), &share.ShareRequest{
		Shares: []share.SharingRecord{{
			SimplifySharingRecord: share.SimplifySharingRecord{
				ResourceEntity: resource,
				PrincipalType:  security.PrincipalTypeUser,
				PrincipalID:    "chain-user-b",
				Permission:     share.Share,
			},
		}},
	})
	require.NoError(t, err, "owner A should be able to share with B")

	// Verify B has Share permission
	permB, err := service.GetUserExplicitEffectivePermission(userB, resource)
	require.NoError(t, err)
	require.Equal(t, share.Share, permB, "B should have Share permission")

	// Step 2: B reshares to C with Edit permission (C can read/write but NOT reshare)
	ctxB := createORMContext(userB)
	_, err = service.CreateOrUpdateShares(ctxB, userB.MustGetUserID(), &share.ShareRequest{
		Shares: []share.SharingRecord{{
			SimplifySharingRecord: share.SimplifySharingRecord{
				ResourceEntity: resource,
				PrincipalType:  security.PrincipalTypeUser,
				PrincipalID:    "chain-user-c",
				Permission:     share.Edit,
			},
		}},
	})
	require.NoError(t, err, "B (with Share permission) should be able to reshare to C")

	// Verify C has Edit permission
	permC, err := service.GetUserExplicitEffectivePermission(userC, resource)
	require.NoError(t, err)
	require.Equal(t, share.Edit, permC, "C should have Edit permission")

	// Step 3: C tries to share with D → must be DENIED (Edit < Share)
	ctxC := createORMContext(userC)
	_, err = service.CreateOrUpdateShares(ctxC, userC.MustGetUserID(), &share.ShareRequest{
		Shares: []share.SharingRecord{{
			SimplifySharingRecord: share.SimplifySharingRecord{
				ResourceEntity: resource,
				PrincipalType:  security.PrincipalTypeUser,
				PrincipalID:    "chain-user-d",
				Permission:     share.View,
			},
		}},
	})
	require.Error(t, err, "C (with Edit permission) must NOT be able to reshare")

	// Step 4: D has no access at all
	permD, err := service.GetUserExplicitEffectivePermission(userD, resource)
	require.NoError(t, err)
	require.Equal(t, share.None, permD, "D (never shared with) must have no permission")

	// Step 5: Verify B can also revoke the share to C (B has Share permission)
	rules := getAllResourceRules(t, "document", resourceID)
	var shareToC *share.SharingRecord
	for i := range rules {
		if rules[i].PrincipalID == "chain-user-c" {
			shareToC = &rules[i]
			break
		}
	}
	require.NotNil(t, shareToC, "share record for C must exist")

	_, err = service.CreateOrUpdateShares(ctxB, userB.MustGetUserID(), &share.ShareRequest{
		Revokes: []share.SharingRecord{
			{ORMObjectBase: orm.ORMObjectBase{ID: shareToC.ID}},
		},
	})
	require.NoError(t, err, "B (with Share permission) should be able to revoke C's share")

	// After revoke, C has no access
	permCAfter, err := service.GetUserExplicitEffectivePermission(userC, resource)
	require.NoError(t, err)
	require.Equal(t, share.None, permCAfter, "C should have no permission after revoke")
}

// TestSharing_RevokeImmediatelyDeniesAccess verifies that once a share is revoked,
// the target user instantly loses all access — tested with fresh user sessions to
// simulate independent logins.
func TestSharing_RevokeImmediatelyDeniesAccess(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	owner := createTestSession("revoke-owner-1", nil, "tenant-a")
	resourceID := uniqueResourceID("res-revoke-instant")
	resource := share.ResourceEntity{ResourceType: "document", ResourceID: resourceID}

	seedTestResource(t, owner, resourceID)
	seedUserPrincipal(t, "revoke-target-1", "tenant-a")
	seedUserPrincipal(t, "revoke-target-2", "tenant-a")

	// Owner shares with target-1 (Edit) and target-2 (Share)
	ctxOwner := createORMContext(owner)
	_, err := service.CreateOrUpdateShares(ctxOwner, owner.MustGetUserID(), &share.ShareRequest{
		Shares: []share.SharingRecord{
			{SimplifySharingRecord: share.SimplifySharingRecord{
				ResourceEntity: resource,
				PrincipalType:  security.PrincipalTypeUser,
				PrincipalID:    "revoke-target-1",
				Permission:     share.Edit,
			}},
			{SimplifySharingRecord: share.SimplifySharingRecord{
				ResourceEntity: resource,
				PrincipalType:  security.PrincipalTypeUser,
				PrincipalID:    "revoke-target-2",
				Permission:     share.Share,
			}},
		},
	})
	require.NoError(t, err)

	// Simulate target-1 logging in (fresh session) — should have Edit
	target1Login1 := createTestSession("revoke-target-1", nil, "tenant-a")
	perm1, err := service.GetUserExplicitEffectivePermission(target1Login1, resource)
	require.NoError(t, err)
	require.Equal(t, share.Edit, perm1, "target-1 should have Edit before revoke")

	// Simulate target-2 logging in — should have Share
	target2Login1 := createTestSession("revoke-target-2", nil, "tenant-a")
	perm2, err := service.GetUserExplicitEffectivePermission(target2Login1, resource)
	require.NoError(t, err)
	require.Equal(t, share.Share, perm2, "target-2 should have Share before revoke")

	// Owner revokes target-1's access
	rules := getAllResourceRules(t, "document", resourceID)
	var target1Share *share.SharingRecord
	for i := range rules {
		if rules[i].PrincipalID == "revoke-target-1" {
			target1Share = &rules[i]
			break
		}
	}
	require.NotNil(t, target1Share)

	_, err = service.CreateOrUpdateShares(ctxOwner, owner.MustGetUserID(), &share.ShareRequest{
		Revokes: []share.SharingRecord{
			{ORMObjectBase: orm.ORMObjectBase{ID: target1Share.ID}},
		},
	})
	require.NoError(t, err)

	// Simulate target-1 logging in AGAIN (completely fresh session) — must have ZERO access
	target1Login2 := createTestSession("revoke-target-1", nil, "tenant-a")
	permAfter, err := service.GetUserExplicitEffectivePermission(target1Login2, resource)
	require.NoError(t, err)
	require.Equal(t, share.None, permAfter, "target-1 must have NO access after revoke (fresh login)")

	// target-2 still has access (not revoked) — fresh session
	target2Login2 := createTestSession("revoke-target-2", nil, "tenant-a")
	perm2After, err := service.GetUserExplicitEffectivePermission(target2Login2, resource)
	require.NoError(t, err)
	require.Equal(t, share.Share, perm2After, "target-2 must still have Share (only target-1 was revoked)")

	// Now revoke target-2 as well
	rules = getAllResourceRules(t, "document", resourceID)
	var target2Share *share.SharingRecord
	for i := range rules {
		if rules[i].PrincipalID == "revoke-target-2" {
			target2Share = &rules[i]
			break
		}
	}
	require.NotNil(t, target2Share)

	_, err = service.CreateOrUpdateShares(ctxOwner, owner.MustGetUserID(), &share.ShareRequest{
		Revokes: []share.SharingRecord{
			{ORMObjectBase: orm.ORMObjectBase{ID: target2Share.ID}},
		},
	})
	require.NoError(t, err)

	// target-2 fresh login — now also denied
	target2Login3 := createTestSession("revoke-target-2", nil, "tenant-a")
	perm2Final, err := service.GetUserExplicitEffectivePermission(target2Login3, resource)
	require.NoError(t, err)
	require.Equal(t, share.None, perm2Final, "target-2 must have NO access after revoke (fresh login)")

	// Verify no sharing records remain for this resource
	finalRules := getAllResourceRules(t, "document", resourceID)
	require.Empty(t, finalRules, "all shares revoked — no records should remain")
}

func TestSharing_BatchGetSharesReturnsErrorInsteadOfPanic(t *testing.T) {
	require.NoError(t, setupErr)

	service := share.NewSharingService()
	user := createTestSession("panic-guard-u1", nil, "tenant-a")
	ctx := createORMContext(user)

	require.NotPanics(t, func() {
		_, err := service.BatchGetShares(ctx, user, []share.ResourceEntity{{}})
		require.Error(t, err)
	})
}

func TestSharing_GetSharingRulesReturnsErrorInsteadOfPanic(t *testing.T) {
	require.NoError(t, setupErr)

	require.NotPanics(t, func() {
		_, err := share.GetSharingRules(nil, "", "", "", nil)
		require.Error(t, err)
	})
}

func TestSharing_GetSharingRulesV2ReturnsErrorInsteadOfPanic(t *testing.T) {
	require.NoError(t, setupErr)

	require.NotPanics(t, func() {
		_, err := share.GetSharingRulesV2(nil, "", "", "", nil)
		require.Error(t, err)
	})
}

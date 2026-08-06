/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package access_token

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"infini.sh/framework/core/log"

	"github.com/emirpasic/gods/sets/hashset"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/security/http_filters"
)

const ProviderName = "access_token"

const (
	// KVAccessTokenBucket stores token_string -> AccessToken JSON. Used by
	// byAPITokenHeader to authenticate inbound requests in both modes.
	KVAccessTokenBucket = ProviderName

	// kvAccessTokenIndexBucket is used only in non-native mode.
	//  - key "__ids__"      -> JSON []string of all known token IDs
	//  - key "<token_id>"   -> token_string (for delete/update by ID)
	kvAccessTokenIndexBucket = "access_token_index"
	kvIndexListKey           = "__ids__"

	HeaderAPIToken = "X-API-TOKEN"
)

var (
	indexLock sync.Mutex

	// Permission catalog. Registered unconditionally in init() so that roles
	// referencing these permissions remain valid even when the access_token
	// HTTP endpoints are disabled.
	createTokenPermission security.PermissionKey
	updateTokenPermission security.PermissionKey
	deleteTokenPermission security.PermissionKey
	searchTokenPermission security.PermissionKey
)

func init() {

	global.RegisterFuncBeforeSetup(func() {
		if global.Env().SystemConfig.WebAppConfig.Security.Authentication.AccessToken.Enabled {
			createTokenPermission = security.GetOrInitPermission("generic", "security:auth:api-token", security.Create)
			updateTokenPermission = security.GetOrInitPermission("generic", "security:auth:api-token", security.Update)
			deleteTokenPermission = security.GetOrInitPermission("generic", "security:auth:api-token", security.Delete)
			searchTokenPermission = security.GetOrInitPermission("generic", "security:auth:api-token", security.Search)

			// The auth filter provider is registered unconditionally so that inbound
			// requests carrying X-API-TOKEN can be authenticated even before Init() is
			// called (e.g. in embedded scenarios that never call Init explicitly).
			security.RegisterHTTPAuthFilterProviderWithPriority("api_token", byAPITokenHeader, 30)

			api.HandleUIMethod(api.POST, "/auth/access_token", RequestAccessToken, api.RequirePermission(createTokenPermission))
			api.HandleUIMethod(api.GET, "/auth/access_token/_search", SearchAccessToken, api.RequirePermission(searchTokenPermission), api.Feature(http_filters.FeatureMaskSensitiveField))
			api.HandleUIMethod(api.DELETE, "/auth/access_token/:token_id", DeleteAccessToken, api.RequirePermission(deleteTokenPermission))
			api.HandleUIMethod(api.PUT, "/auth/access_token/:token_id", UpdateAccessToken, api.RequirePermission(updateTokenPermission))

		}
	})
}

// isNative reports whether the access_token module should persist tokens via
// ORM. When false the module operates in KV-only mode (e.g. for agent-style
// deployments without an ORM backend).
func isNative() bool {
	return global.Env().SystemConfig.WebAppConfig.Security.Authentication.AccessToken.Native
}

func byAPITokenHeader(w http.ResponseWriter, r *http.Request) (claims *security.UserClaims, err error) {
	apiToken := r.Header.Get(HeaderAPIToken)

	accessToken, permissions, err := getTokenPermissions(apiToken)
	if err != nil {
		return nil, err
	}

	claims = security.NewUserClaims()
	claims.SetUserID(accessToken.GetOwnerID())
	claims.Provider = ProviderName
	claims.Login = apiToken
	claims.UserAssignedPermission = security.NewUserAssignedPermission(permissions, nil)
	claims.Data = accessToken.CloneData()

	return claims, nil
}

func getTokenPermissions(apiToken string) (*security.AccessToken, []security.PermissionKey, error) {
	if apiToken == "" {
		return nil, nil, errors.Error("API token not provided")
	}

	bytes, err := kv.GetValue(KVAccessTokenBucket, []byte(apiToken))
	if err != nil {
		return nil, nil, err
	}

	if len(bytes) == 0 {
		return nil, nil, errors.Errorf("invalid %s", HeaderAPIToken)
	}

	accessToken := security.AccessToken{}
	util.MustFromJSONBytes(bytes, &accessToken)

	if global.Env().IsDebug {
		log.Debug("get AccessToken from store:", string(bytes))
	}

	//-1 means never expire
	if accessToken.ExpireIn > 0 {
		expireAtTime := time.Unix(accessToken.ExpireIn, 0)
		if time.Now().After(expireAtTime) {
			return nil, nil, errors.Error("token expired")
		}
	}

	permissions := accessToken.Permissions

	//user may be revoked some permission after issued the api token
	if isNative() {
		// Effective permissions = token permissions ∩ owning user's current permissions.
		// Requires a user/role store.
		apiTokenLevelPermission := security.ConvertPermissionKeysToHashSet(accessToken.Permissions)

		userSessionInfo := security.UserSessionInfo{}
		userSessionInfo.Provider = security.DefaultNativeAuthBackend
		userSessionInfo.SetUserID(accessToken.GetOwnerID())

		userLevelTokenLevelPermission := security.ConvertPermissionKeysToHashSet(security.GetAllPermissionsForUser(&userSessionInfo))
		intersectedPermission := security.IntersectSetsFast(apiTokenLevelPermission, userLevelTokenLevelPermission)
		if global.Env().IsDebug {
			log.Trace("apiTokenLevelPermission:", apiTokenLevelPermission.Values())
			log.Trace("userLevelTokenLevelPermission:", userLevelTokenLevelPermission.Values())
			log.Trace("intersectedPermission:", intersectedPermission.Values())
		}

		permissions = security.ConvertPermissionHashSetToKeys(intersectedPermission)
	}
	return &accessToken, permissions, nil
}

func GetPermissionHashSet(u *security.UserSessionInfo) *hashset.Set {
	keys := security.GetAllPermissionsForUser(u)
	set := security.ConvertPermissionKeysToHashSet(keys)
	return set
}

func RequestAccessToken(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	// user already login
	reqUser, err := security.GetUserFromContext(req.Context())
	if reqUser == nil || err != nil {
		panic(err)
	}

	reqBody := struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Permissions []security.PermissionKey `json:"permissions,omitempty"`
	}{}
	err = api.DecodeJSON(req, &reqBody)
	if err != nil {
		panic(errors.NewWithHTTPCode(400, "invalid token"))
	}
	if reqBody.Name == "" {
		reqBody.Name = GenerateApiTokenName("")
	}

	var permissions []security.PermissionKey
	if isNative() {
		permissions = security.GetAllPermissionsForUser(reqUser)
		if len(reqBody.Permissions) > 0 {
			// requested permissions must be within the caller's own scope
			if !util.IsSuperset(security.ConvertPermissionKeysToHashSet(permissions), security.ConvertPermissionKeysToHashSet(reqBody.Permissions)) {
				panic(errors.NewWithHTTPCode(403, "invalid permissions"))
			}
			permissions = reqBody.Permissions
		}
	} else {
		// no user/role store: trust whatever the caller asked for, falling
		// back to the caller's session permissions when none are specified.
		if len(reqBody.Permissions) > 0 {
			permissions = reqBody.Permissions
		} else {
			permissions = append(permissions, reqUser.GetPermissionKeys()...)
		}
	}

	expiredAT := time.Now().Add(365 * 24 * time.Hour).Unix()
	res, err := CreateAPIToken(reqUser, reqBody.Name, reqBody.Description, "general", expiredAT, permissions)
	if err != nil {
		panic(err)
	}

	api.WriteJSON(w, res, 200)
}

func CreateAPIToken(user *security.UserSessionInfo, tokenName, tokenDesc, typeName string, expiredAT int64, permissions []security.PermissionKey) (util.MapStr, error) {

	if tokenName == "" {
		tokenName = GenerateApiTokenName("")
	}

	accessTokenStr := util.GetUUID() + util.GenerateRandomString(64)

	accessToken := security.AccessToken{}
	tokenID := util.GetUUID()
	accessToken.ID = tokenID
	accessToken.AccessToken = accessTokenStr
	if user != nil {
		user.Roles = nil
		accessToken.SetOwnerID(user.MustGetUserID())
		accessToken.Data = user.CloneData()
	}

	accessToken.Type = typeName
	accessToken.Permissions = permissions
	accessToken.ExpireIn = expiredAT
	accessToken.Name = tokenName
	accessToken.Description = tokenDesc

	if isNative() {
		ctx := orm.NewContext()
		ctx.DirectAccess()
		ctx.Refresh = orm.WaitForRefresh
		ctx.PermissionScope(security.PermissionScopePlatform)

		if err := orm.Create(ctx, &accessToken); err != nil {
			return nil, err
		}
	} else {
		if err := addTokenToIndex(tokenID, accessTokenStr); err != nil {
			return nil, err
		}
	}

	// persist token for fast lookup by token string (used by auth filter)
	if err := kv.AddValue(KVAccessTokenBucket, []byte(accessTokenStr), util.MustToJSONBytes(&accessToken)); err != nil {
		return nil, err
	}

	res := util.MapStr{
		"_id":          tokenID,
		"access_token": accessTokenStr,
		"expire_in":    expiredAT,
	}
	return res, nil
}

func SearchAccessToken(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	reqUser, err := security.GetUserFromContext(req.Context())
	if reqUser == nil || err != nil {
		api.WriteError(w, "permission denied", http.StatusForbidden)
		return
	}

	if isNative() {
		builder, err := orm.NewQueryBuilderFromRequest(req, "name")
		if err != nil {
			panic(err)
		}

		ctx := orm.NewContextWithParent(req.Context())
		orm.WithModel(ctx, &security.AccessToken{})

		res, err := orm.SearchV2(ctx, builder)
		if err != nil {
			panic(err)
		}

		if _, err = api.Write(w, res.Payload.([]byte)); err != nil {
			api.Error(w, err)
		}
		return
	}

	tokens, err := listAccessTokensFromKV(reqUser.MustGetUserID())
	if err != nil {
		api.Error(w, err)
		return
	}

	api.WriteJSON(w, util.MapStr{
		"hits": util.MapStr{
			"total":     util.MapStr{"value": len(tokens), "relation": "eq"},
			"max_score": 0,
			"hits":      tokens,
		},
	}, 200)
}

func canOperateToken(reqUser *security.UserSessionInfo, token *security.AccessToken) bool {
	if reqUser == nil || token == nil {
		return false
	}
	if util.ContainsAnyInArray(security.RoleAdmin, reqUser.Roles) {
		return true
	}
	ownerID := token.GetOwnerID()
	if ownerID == "" {
		return false
	}
	return ownerID == reqUser.MustGetUserID()
}

func DeleteAccessToken(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	reqUser, err := security.GetUserFromContext(req.Context())
	if reqUser == nil || err != nil {
		panic(err)
	}
	tokenID := ps.ByName("token_id")

	var tokenString string

	if isNative() {
		ctx := orm.NewContextWithParent(req.Context())

		// Load first so we know the token string (needed to clean up the KV
		// lookup entry below). Without this, orm.Delete only knows the ID and
		// leaves the KV row orphaned.
		token := security.AccessToken{}
		token.ID = tokenID
		exists, err := orm.GetV2(ctx, &token)
		if err != nil {
			panic(err)
		}
		if !exists {
			api.WriteError(w, "access token not found", 404)
			return
		}
		if !canOperateToken(reqUser, &token) {
			api.WriteError(w, "access token not found", 404)
			return
		}
		tokenString = token.AccessToken

		ctx.Refresh = orm.WaitForRefresh
		if err = orm.Delete(ctx, &token); err != nil {
			panic(err)
		}
	} else {
		token, err := getAccessTokenByIDFromKV(tokenID)
		if err != nil {
			api.Error(w, err)
			return
		}
		if !canOperateToken(reqUser, token) {
			api.WriteError(w, "access token not found", 404)
			return
		}

		s, err := removeTokenFromIndex(tokenID)
		if err != nil {
			api.Error(w, err)
			return
		}
		tokenString = s
	}

	if tokenString != "" {
		if err = kv.DeleteKey(KVAccessTokenBucket, []byte(tokenString)); err != nil {
			panic(err)
		}
	}

	// Invalidate the per-user permission cache so any request that authenticated
	// with this token (or any other token belonging to the same owner) re-derives
	// its UserAssignedPermission on the next call instead of using a stale entry.
	security.IncreasePermissionVersion()

	api.WriteDeletedOKJSON(w, tokenID)
}

func GetToken(token string) (*security.AccessToken, error) {
	tokenBytes, err := kv.GetValue(KVAccessTokenBucket, []byte(token))
	if err != nil {
		return nil, err
	}
	if len(tokenBytes) == 0 {
		return nil, errors.Errorf("token not found")
	}
	accessToken := security.AccessToken{}
	if err = util.FromJSONBytes(tokenBytes, &accessToken); err != nil {
		return nil, err
	}
	return &accessToken, nil
}

func UpdateAccessToken(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	reqUser, err := security.GetUserFromContext(req.Context())
	if reqUser == nil || err != nil {
		panic(err)
	}
	reqBody := struct {
		Name        string                   `json:"name,omitempty"`
		Description string                   `json:"description"`
		Permissions []security.PermissionKey `json:"permissions,omitempty"`
	}{}
	err = api.DecodeJSON(req, &reqBody)
	if err != nil {
		panic(err)
	}
	if reqBody.Name == "" {
		api.WriteError(w, "name is required", 400)
		return
	}
	tokenID := ps.ByName("token_id")

	var token *security.AccessToken

	if isNative() {
		ctx := orm.NewContextWithParent(req.Context())
		token = &security.AccessToken{}
		token.ID = tokenID
		exists, err := orm.GetV2(ctx, token)
		if err != nil {
			panic(err)
		}
		if !exists {
			api.WriteError(w, "access token not found", 404)
			return
		}
		if !canOperateToken(reqUser, token) {
			api.WriteError(w, "access token not found", 404)
			return
		}
	} else {
		t, err := getAccessTokenByIDFromKV(tokenID)
		if err != nil {
			api.WriteError(w, err.Error(), 404)
			return
		}
		if !canOperateToken(reqUser, t) {
			api.WriteError(w, "access token not found", 404)
			return
		}
		token = t
	}

	if reqBody.Name != "" {
		token.Name = reqBody.Name
	}
	if reqBody.Description != "" {
		token.Description = reqBody.Description
	}

	if len(reqBody.Permissions) > 0 {
		if isNative() {
			// The NEW permissions must be a subset of the caller's own permissions.
			requested := security.ConvertPermissionKeysToHashSet(reqBody.Permissions)
			if !util.IsSuperset(GetPermissionHashSet(reqUser), requested) {
				panic("invalid permissions")
			}
		}
		token.Permissions = reqBody.Permissions
	}

	if isNative() {
		ctx := orm.NewContextWithParent(req.Context())
		ctx.Refresh = orm.WaitForRefresh
		if err = orm.Save(ctx, token); err != nil {
			panic(err)
		}
	}

	if err = kv.AddValue(KVAccessTokenBucket, []byte(token.AccessToken), util.MustToJSONBytes(token)); err != nil {
		panic(err)
	}

	// Force the next request that uses this (or any) token to re-derive its
	// permissions, so the freshly-saved permission set takes effect immediately
	// instead of waiting for the 30-minute permissionCache TTL.
	security.IncreasePermissionVersion()

	api.WriteUpdatedOKJSON(w, tokenID)
}

// GenerateApiTokenName generates a unique API token name
func GenerateApiTokenName(prefix string) string {
	if prefix == "" {
		prefix = "token"
	}
	timestamp := time.Now().UnixMilli()
	randomStr := util.GenerateRandomString(8)
	return fmt.Sprintf("%s_%d_%s", prefix, timestamp, randomStr)
}

// --- KV-side index helpers (non-native mode only) -------------------------

func loadTokenIDs() ([]string, error) {
	bytes, err := kv.GetValue(kvAccessTokenIndexBucket, []byte(kvIndexListKey))
	if err != nil {
		return nil, err
	}
	if len(bytes) == 0 {
		return nil, nil
	}
	ids := []string{}
	if err := util.FromJSONBytes(bytes, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func saveTokenIDs(ids []string) error {
	return kv.AddValue(kvAccessTokenIndexBucket, []byte(kvIndexListKey), util.MustToJSONBytes(ids))
}

func addTokenToIndex(tokenID, tokenString string) error {
	indexLock.Lock()
	defer indexLock.Unlock()

	if err := kv.AddValue(kvAccessTokenIndexBucket, []byte(tokenID), []byte(tokenString)); err != nil {
		return err
	}

	ids, err := loadTokenIDs()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == tokenID {
			return nil
		}
	}
	ids = append(ids, tokenID)
	return saveTokenIDs(ids)
}

func removeTokenFromIndex(tokenID string) (string, error) {
	indexLock.Lock()
	defer indexLock.Unlock()

	tokenStringBytes, err := kv.GetValue(kvAccessTokenIndexBucket, []byte(tokenID))
	if err != nil {
		return "", err
	}
	if len(tokenStringBytes) == 0 {
		return "", errors.Errorf("access token not found: %s", tokenID)
	}

	if err := kv.DeleteKey(kvAccessTokenIndexBucket, []byte(tokenID)); err != nil {
		return "", err
	}

	ids, err := loadTokenIDs()
	if err != nil {
		return string(tokenStringBytes), err
	}
	out := ids[:0]
	for _, id := range ids {
		if id != tokenID {
			out = append(out, id)
		}
	}
	if err := saveTokenIDs(out); err != nil {
		return string(tokenStringBytes), err
	}

	return string(tokenStringBytes), nil
}

func getAccessTokenByIDFromKV(tokenID string) (*security.AccessToken, error) {
	tokenStringBytes, err := kv.GetValue(kvAccessTokenIndexBucket, []byte(tokenID))
	if err != nil {
		return nil, err
	}
	if len(tokenStringBytes) == 0 {
		return nil, errors.Errorf("access token not found: %s", tokenID)
	}
	return GetToken(string(tokenStringBytes))
}

func listAccessTokensFromKV(ownerID string) ([]util.MapStr, error) {
	ids, err := loadTokenIDs()
	if err != nil {
		return nil, err
	}
	out := make([]util.MapStr, 0, len(ids))
	for _, id := range ids {
		t, err := getAccessTokenByIDFromKV(id)
		if err != nil {
			log.Warnf("load access token [%s] failed: %v", id, err)
			continue
		}
		if ownerID != "" && t.GetOwnerID() != ownerID {
			continue
		}
		out = append(out, util.MapStr{
			"_id":     t.ID,
			"_source": t,
		})
	}
	return out, nil
}

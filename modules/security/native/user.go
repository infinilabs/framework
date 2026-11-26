/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gookit/validate"
	"net/http"
	"strings"

	log "github.com/cihub/seelog"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

func GetUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &obj)
	if err != nil {
		panic(err)
	}
	if !exists {
		api.NotFoundResponse(id)
		return
	}

	api.WriteGetOKJSON(w, id, obj)
}

func UpdateUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	ctx := orm.NewContextWithParent(req.Context())

	obj := security.UserAccount{}
	err := api.DecodeJSON(req, &obj)
	if err != nil {
		panic(err)
	}

	api.MustValidateInput(w, obj)

	oldObj := security.UserAccount{}
	oldObj.ID = id
	exists, err := orm.GetV2(ctx, &oldObj)
	if err != nil {
		panic(err)
	}
	if !exists {
		api.NotFoundResponse(id)
		return
	}

	//protect
	obj.ID = id
	obj.Email = oldObj.Email

	sessionUser := security.MustGetUserFromContext(ctx)
	userID := sessionUser.MustGetUserID()
	if userID == id {
		//user can't update self's role
		if !util.CompareStringArray(obj.Roles, oldObj.Roles) {
			panic("sorry, you can not update your roles")
		}
	}

	if obj.Password == "" {
		obj.Password = oldObj.Password
	} else {
		if !util.ValidateSecure(obj.Password) {
			panic("should be secured password")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(obj.Password), bcrypt.DefaultCost)
		if err != nil {
			panic(err)
		}
		obj.Password = string(hash)
	}
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Update(ctx, &obj)
	if err != nil {
		panic(err)
	}

	security.IncreasePermissionVersion()

	api.WriteUpdatedOKJSON(w, obj.ID)
}
func DeleteUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	sessionUser := security.MustGetUserFromContext(ctx)
	userID := sessionUser.MustGetUserID()
	if userID == id {
		panic("you can not delete yourself")
	}

	ctx.Refresh = orm.WaitForRefresh
	err := orm.Delete(ctx, &obj)
	if err != nil {
		panic(err)
	}

	api.WriteDeletedOKJSON(w, obj.ID)
}

func SearchUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	builder, err := orm.NewQueryBuilderFromRequest(req, "id", "name", "email")
	if err != nil {
		panic(err)
	}
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()

	ctx.PermissionScope(security.PermissionScopePlatform)

	orm.WithModel(ctx, &security.UserAccount{})
	res, err := orm.SearchV2(ctx, builder)
	if err != nil {
		panic(err)
	}

	_, err = api.Write(w, res.Payload.([]byte))
	if err != nil {
		panic(err)
	}
}

// GetUserByLogin finds a native account by email.
func GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	if !orm.IsHandlerRegistered() {
		return false, nil, errors.New("native account ORM is not ready")
	}
	ctx := orm.NewContext().DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	orm.WithModel(ctx, &security.UserAccount{})
	result, err := orm.SearchV2(ctx, orm.NewQuery().Filter(orm.TermQuery("email", email)).Size(1))
	if err != nil {
		return false, nil, err
	}
	if result == nil || result.Status < 200 || result.Status >= 300 {
		return false, nil, fmt.Errorf("native account search failed: %v", result)
	}
	if result.Error != nil && *result.Error != nil {
		return false, nil, *result.Error
	}
	var raw []byte
	switch payload := result.Payload.(type) {
	case []byte:
		raw = payload
	case string:
		raw = []byte(payload)
	default:
		return false, nil, errors.New("native account search returned no payload")
	}
	var completion struct {
		Shards struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits *json.RawMessage `json:"hits"`
	}
	if err := json.Unmarshal(raw, &completion); err != nil {
		return false, nil, err
	}
	if completion.Shards.Failed > 0 || completion.Hits == nil {
		return false, nil, errors.New("native account search returned incomplete results")
	}
	response, err := elastic.DecodeSearchResult(result)
	if err != nil {
		return false, nil, err
	}
	if response.Error != nil || response.TimedOut {
		return false, nil, errors.New("native account search did not complete")
	}
	accounts, _, err := elastic.DecodeHits[security.UserAccount](result)
	if err != nil {
		return false, nil, err
	}
	if len(accounts) == 0 {
		return false, nil, nil
	}
	return true, &accounts[0], nil
}

func (provider *SecurityBackendProvider) GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	return GetUserByLogin(email)
}

// GetUserByID treats only a missing document as normal absence.
func (provider *SecurityBackendProvider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	if !orm.IsHandlerRegistered() {
		return false, nil, errors.New("native account ORM is not ready")
	}
	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContext().DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)
	exists, err := orm.GetV2(ctx, &obj)
	if errors.Is(err, orm.ErrNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	if !exists {
		return false, nil, nil
	}
	return true, &obj, nil
}

func (provider *SecurityBackendProvider) CreateUser(name, email, password string, force bool) (*security.UserAccount, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}

	if !util.ValidateSecure(password) {
		return nil, errors.New("password does not meet the security policy")
	}

	exists, account, err := GetUserByLogin(email)
	if err != nil {
		return nil, err
	}

	userID := getUIDByEmail(email)
	if exists {
		userID = account.ID
	}
	if err := validateStaticIdentity(userID, email); err != nil {
		return nil, err
	}

	if exists && !force {
		return nil, errors.New("email already existed")
	}

	var obj = &security.UserAccount{}
	obj.ID = userID

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	obj.Name = name
	obj.Email = email
	obj.Roles = []string{security.RoleAdmin}
	obj.Password = string(hash)

	ctx := orm.NewContext()
	ctx.DirectAccess()
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func getUIDByEmail(email string) string {
	return util.MD5digest(global.Env().SystemConfig.NodeConfig.ID + email)
}

func CreateUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &security.UserAccount{}
	err := api.DecodeJSON(req, obj)
	if err != nil {
		api.WriteError(w, "invalid user input", http.StatusBadRequest)
		return
	}

	obj.Email, err = normalizeEmail(obj.Email)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	validation := validate.Struct(obj)
	if !validation.Validate() {
		api.WriteError(w, "invalid user input", http.StatusBadRequest)
		return
	}

	if err := validateStaticIdentity(getUIDByEmail(obj.Email), obj.Email); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, security.ErrIdentityConflict) {
			status = http.StatusConflict
		}
		api.WriteError(w, "account identity check failed", status)
		return
	}
	exists, account, err := GetUserByLogin(obj.Email)
	if err != nil {
		panic(err)
	}
	if exists && account != nil {
		log.Warn("email already exists")
		//obj.ID = account.ID
		panic("email already existed")
	} else {
		obj.ID = getUIDByEmail(obj.Email)
	}

	randStr := util.GenerateSecureString(8)
	hash, err := bcrypt.GenerateFromPassword([]byte(randStr), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	obj.Password = string(hash)

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, obj)
	if err != nil {
		panic(err)
	}

	obj.Password = randStr
	api.WriteJSON(w, obj, 200)
}

// Helper function to apply the same email rules to both native creation entry points.
func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if !validate.IsEmail(email) {
		return "", errors.New("a valid email is required")
	}
	return email, nil
}

// Helper function to reject static identity conflicts at both native creation entry points.
func validateStaticIdentity(userID, login string) error {
	provider, enabled := security.GetAuthenticationProvider(security.StaticAuthBackend)
	if !enabled {
		return nil
	}
	found, _, err := provider.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("check static account ID: %w", err)
	}
	if found {
		return fmt.Errorf("%w: static account ID %q", security.ErrIdentityConflict, userID)
	}
	found, _, err = provider.GetUserByLogin(login)
	if err != nil {
		return fmt.Errorf("check static account login: %w", err)
	}
	if found {
		return fmt.Errorf("%w: static account login %q", security.ErrIdentityConflict, login)
	}
	return nil
}

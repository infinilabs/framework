/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"fmt"
	"net/http"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	cerr "infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

const (
	errCannotUpdateOwnRoles = "sorry, you can not update your roles"
	errCannotDeleteSelf     = "you can not delete yourself"
	errInsecurePassword     = "password does not meet security requirements"
	errEmailAlreadyExists   = "email already existed"
)

func GetUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		api.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
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
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	api.MustValidateInput(w, obj)

	oldObj := security.UserAccount{}
	oldObj.ID = id
	exists, err := orm.GetV2(ctx, &oldObj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		api.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
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
			api.WriteError(w, errCannotUpdateOwnRoles, http.StatusForbidden)
			return
		}
	}

	if obj.Password == "" {
		// Preserve the verifier material on metadata-only updates so editing roles,
		// names, or other fields does not silently disable challenge login.
		obj.Password = oldObj.Password
		obj.PasswordSalt = oldObj.PasswordSalt
		obj.PasswordVerifier = oldObj.PasswordVerifier
	} else {
		if err := validateSecurePassword(obj.Password); err != nil {
			api.WriteError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := security.SetPassword(&obj, obj.Password); err != nil {
			api.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Update(ctx, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
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
		api.WriteError(w, errCannotDeleteSelf, http.StatusForbidden)
		return
	}

	ctx.Refresh = orm.WaitForRefresh
	err := orm.Delete(ctx, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.WriteDeletedOKJSON(w, obj.ID)
}

func SearchUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	builder, err := orm.NewQueryBuilderFromRequest(req, "id", "name", "email")
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()

	ctx.PermissionScope(security.PermissionScopePlatform)

	orm.WithModel(ctx, &security.UserAccount{})
	res, err := orm.SearchV2(ctx, builder)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = api.Write(w, res.Payload.([]byte))
	if err != nil {
		api.Error(w, err)
	}
}

func GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	ctx := orm.NewContext()
	ctx.DirectReadAccess()

	ctx.PermissionScope(security.PermissionScopePlatform)

	ctx = orm.WithModel(ctx, &security.UserAccount{})

	qb := orm.NewQuery()
	qb.Should(orm.TermQuery("email", email), orm.TermQuery("id", getUIDByEmail(email))).MinimumShouldMatch(1)
	items := []security.UserAccount{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &items, qb, nil)
	if err != nil {
		return false, nil, err
	}

	return resolveUserByLogin(email, items)
}

func (provider *SecurityBackendProvider) GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	return GetUserByLogin(email)
}

func (provider *SecurityBackendProvider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContext()
	ctx.DirectReadAccess()

	ctx.PermissionScope(security.PermissionScopePlatform)

	exists, err := orm.GetV2(ctx, &obj)
	if err != nil {
		return false, nil, err
	}
	if !exists {
		return false, nil, nil
	}
	return true, &obj, nil
}

func (provider *SecurityBackendProvider) CreateUser(name, email, password string, force bool) (*security.UserAccount, error) {

	if err := validateSecurePassword(password); err != nil {
		return nil, err
	}

	exists, account, err := GetUserByLogin(email)
	if err != nil {
		return nil, err
	}

	if exists && !force {
		return nil, cerr.NewWithHTTPCode(http.StatusConflict, errEmailAlreadyExists)
	}

	var obj = &security.UserAccount{}
	if exists && account != nil {
		log.Warn("email already exists, will be replaced")
		obj.ID = account.ID
	} else {
		obj.ID = getUIDByEmail(email)
	}

	obj.Name = name
	obj.Email = email
	obj.Roles = []string{security.RoleAdmin}
	if err := security.SetPassword(obj, password); err != nil {
		return nil, err
	}

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
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	api.MustValidateInput(w, obj)

	exists, account, err := GetUserByLogin(obj.Email)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists && account != nil {
		log.Warn("email already exists")
		api.WriteError(w, errEmailAlreadyExists, http.StatusConflict)
		return
	} else {
		obj.ID = getUIDByEmail(obj.Email)
	}

	randStr := util.GenerateSecureString(8)
	if err := security.SetPassword(obj, randStr); err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	obj.Password = randStr
	// The one-time bootstrap password should be returned to the caller, but the
	// persisted verifier material must stay server-side only.
	obj.PasswordSalt = ""
	obj.PasswordVerifier = ""
	api.WriteJSON(w, obj, 200)
}

func validateSecurePassword(password string) error {
	if util.ValidateSecure(password) {
		return nil
	}
	return cerr.NewWithHTTPCode(http.StatusBadRequest, errInsecurePassword)
}

func resolveUserByLogin(login string, items []security.UserAccount) (bool, *security.UserAccount, error) {
	switch len(items) {
	case 0:
		return false, nil, nil
	case 1:
		return true, &items[0], nil
	default:
		log.Warnf("invalid users, more than one account was associated with the same email: %v", login)
		return false, nil, fmt.Errorf("multiple accounts found for login %q", login)
	}
}

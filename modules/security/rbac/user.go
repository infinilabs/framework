/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	log "github.com/cihub/seelog"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"net/http"
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

func GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
	ctx = orm.WithModel(ctx, &security.UserAccount{})

	qb := orm.NewQuery()
	qb.Should(orm.TermQuery("email", email), orm.TermQuery("id", getUIDByEmail(email))).MinimumShouldMatch(1)
	items := []security.UserAccount{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &items, qb, nil)
	if err != nil {
		return false, nil, err
	}
	if len(items) > 0 {
		if len(items) == 1 {
			return true, &items[0], nil
		} else {
			log.Warnf("invalid users, more than one account was associated with the same email: %v", email)
		}
	}
	return false, nil, nil
}

func (provider *SecurityBackendProvider) GetUserByLogin(email string) (bool, *security.UserAccount, error) {
	return GetUserByLogin(email)
}

func (provider *SecurityBackendProvider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	obj := security.UserAccount{}
	obj.ID = id
	ctx := orm.NewContext()
	ctx.DirectReadAccess()
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

	if !util.ValidateSecure(password) {
		panic("should be secured password")
	}

	exists, account, err := GetUserByLogin(email)
	if err != nil {
		panic(err)
	}

	if exists && !force {
		panic("email already existed")
	}

	var obj = &security.UserAccount{}
	if exists && account != nil {
		log.Warn("email already exists, will be replaced")
		obj.ID = account.ID
	} else {
		obj.ID = getUIDByEmail(obj.Email)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
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
		panic(err)
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
		panic(err)
	}

	api.MustValidateInput(w, obj)

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

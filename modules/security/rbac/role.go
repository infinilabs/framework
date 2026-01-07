/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"context"
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

func GetRole(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := security.UserRole{}
	obj.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()

	ctx.Set(orm.ReadPermissionCheckingScope, security.PermissionScopePlatform)

	exists, err := orm.GetV2(ctx, &obj)
	if !exists || err != nil {
		api.NotFoundResponse(id)
		return
	}

	api.WriteGetOKJSON(w, id, obj)
}

func UpdateRole(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectAccess()

	obj := security.UserRole{}
	err := api.DecodeJSON(req, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.MustValidateInput(w, obj)

	//bypass managed mode
	if !global.Env().SystemConfig.WebAppConfig.Security.Managed {
		sessionUser := security.MustGetUserFromContext(ctx)
		userID := sessionUser.MustGetUserID()
		_, account, err := security.GetUserByID(userID)

		if account == nil || err != nil {
			panic("invalid user")
		}

		_, role := GetRoleByID(id)
		if role == nil {
			panic("invalid role")
		}
		if util.ContainsAnyInArray(role.Name, account.Roles) {
			panic("you can not update the roles for you")
		}
	}

	//protect
	obj.ID = id
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	security.ReplacePermissionsForRole(obj.Name, obj.Grants.AllowedPermissions)

	api.WriteUpdatedOKJSON(w, obj.ID)
}
func DeleteRole(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := security.UserRole{}
	obj.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectAccess()
	ctx.Refresh = orm.WaitForRefresh
	err := orm.Delete(ctx, &obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.WriteDeletedOKJSON(w, obj.ID)
}

func SearchRole(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	builder, err := orm.NewQueryBuilderFromRequest(req, "id", "name", "description")
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()

	ctx.Set(orm.ReadPermissionCheckingScope, security.PermissionScopePlatform)

	orm.WithModel(ctx, &security.UserRole{})
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

func GetRoleByID(id string) (bool, *security.UserRole) {
	obj := security.UserRole{}
	obj.ID = id
	ctx := orm.NewContext()
	ctx.DirectReadAccess()

	ctx.Set(orm.ReadPermissionCheckingScope, security.PermissionScopePlatform)

	exists, err := orm.GetV2(ctx, &obj)
	if !exists || err != nil {
		return false, nil
	}
	return true, &obj
}

func GetRoleByName(name string) (bool, *security.UserRole) {
	builder := orm.NewQuery()
	builder.Must(orm.TermQuery("name", name))
	ctx := orm.NewContext()
	ctx.DirectReadAccess()

	ctx.Set(orm.ReadPermissionCheckingScope, security.PermissionScopePlatform)

	orm.WithModel(ctx, &security.UserRole{})
	out := []security.UserRole{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &out, builder, nil)
	if err != nil {
		return false, nil
	}
	if len(out) == 1 {
		if out[0].Name == name {
			return true, &out[0]
		}
	}
	return false, nil
}

func CreateRole(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &security.UserRole{}
	err := api.DecodeJSON(req, obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if obj.Name == "admin" {
		panic("can not use the reserved role name")
	}

	api.MustValidateInput(w, obj)

	exists, _ := GetRoleByName(obj.Name)
	if exists {
		panic("same role name already exists")
	}

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh
	err = orm.Create(ctx, obj)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	security.ReplacePermissionsForRole(obj.Name, obj.Grants.AllowedPermissions)

	api.WriteCreatedOKJSON(w, obj.ID)
}

type SecurityBackendProvider struct {
}

func (provider *SecurityBackendProvider) GetPermissionKeysByUserID(ctx1 context.Context, userID string) []security.PermissionKey {
	var allowedPermissions = []security.PermissionKey{}

	//bypass managed mode
	if global.Env().SystemConfig.WebAppConfig.Security.Managed {
		return allowedPermissions
	}

	_, account, _ := security.GetUserByID(userID)
	if account == nil {
		return allowedPermissions
	}

	//for admin only
	if util.ContainsAnyInArray(security.RoleAdmin, account.Roles) {
		permissions := security.GetAllPermissionKeys()
		return permissions
	}

	if len(account.Roles) > 0 {
		perms := provider.GetPermissionKeysByRoles(ctx1, account.Roles)
		allowedPermissions = append(allowedPermissions, perms...)
	}

	return allowedPermissions
}

func (provider *SecurityBackendProvider) GetPermissionKeysByRoles(ctx1 context.Context, roles []string) []security.PermissionKey {
	if len(roles) == 0 {
		return []security.PermissionKey{}
	}

	ctx := orm.NewContextWithParent(ctx1)
	ctx.DirectReadAccess()

	ctx.Set(orm.ReadPermissionCheckingScope, security.PermissionScopePlatform)

	orm.WithModel(ctx, &security.UserRole{})
	qb := orm.NewQuery()
	qb.Must(orm.TermsQuery("name", roles))
	result := []security.UserRole{}
	err, _ := elastic.SearchV2WithResultItemMapper(ctx, &result, qb, nil)
	if err != nil {
		panic(err)
	}

	allowed := make(map[security.PermissionKey]struct{}, 128)
	denied := make(map[security.PermissionKey]struct{}, 64)
	// collect permissions
	for _, v := range result {
		for _, p := range v.Grants.AllowedPermissions {
			allowed[p] = struct{}{}
		}
		for _, p := range v.Grants.DeniedPermissions {
			denied[p] = struct{}{}
		}
	}

	// remove denied keys from allowed
	for p := range denied {
		delete(allowed, p)
	}

	// convert map to slice
	keys := make([]security.PermissionKey, 0, len(allowed))
	for p := range allowed {
		keys = append(keys, p)
	}
	return keys
}

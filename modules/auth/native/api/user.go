/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"errors"
	log "github.com/cihub/seelog"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic"
	"net/http"
	"time"
)

func (h APIHandler) CreateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var user rbac.User
	err := h.DecodeJSON(r, &user)
	if err != nil {
		h.Error400(w, err.Error())
		return
	}
	if user.Name == ""  {
		h.Error400(w, "username is require")
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	if h.userNameExists(w, user.Name) {
		return
	}
	randStr := util.GenerateRandomString(8)
	hash, err := bcrypt.GenerateFromPassword([]byte(randStr), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	user.Password = string(hash)

	user.Created = time.Now()
	user.Updated = time.Now()

	id, err := h.User.Create(&user)
	user.ID = id
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	_ = h.WriteOKJSON(w, util.MapStr{
		"_id":      id,
		"password": randStr,
		"result":   "created",
	})
	return

}

func (h APIHandler) userNameExists(w http.ResponseWriter, name string) bool {
	u, err := h.User.GetBy("name", name)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return true
	}
	if  name == "admin" || u != nil {
		h.ErrorInternalServer(w, "user name already exists")
		return true
	}
	return false
}

func (h APIHandler) GetUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	user, err := h.User.Get(id)
	if errors.Is(err, elastic.ErrNotFound) {
		h.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
		return
	}

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.FoundResponse(id, user))
	return
}

func (h APIHandler) UpdateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var user rbac.User
	err := h.DecodeJSON(r, &user)
	if err != nil {
		_ = log.Error(err.Error())
		h.Error400(w, err.Error())
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	oldUser, err := h.User.Get(id)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	if user.Name != oldUser.Name && h.userNameExists(w, user.Name) {
		return
	}
	user.Updated = time.Now()
	user.Created = oldUser.Created
	user.ID = id
	err = h.User.Update(&user)

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	_ = h.WriteOKJSON(w, api.UpdateResponse(id))
	return
}


func (h APIHandler) DeleteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	err := h.User.Delete(id)
	if errors.Is(err, elastic.ErrNotFound) {
		h.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
		return
	}
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	_ = h.WriteOKJSON(w, api.DeleteResponse(id))
	return
}

func (h APIHandler) SearchUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var (
		keyword = h.GetParameterOrDefault(r, "keyword", "")
		from    = h.GetIntOrDefault(r, "from", 0)
		size    = h.GetIntOrDefault(r, "size", 20)
	)

	res, err := h.User.Search(keyword, from, size)
	if err != nil {
		log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}

	h.Write(w, res.Raw)
	return

}
func (h APIHandler) UpdateUserPassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var req = struct {
		Password string `json:"password"`
	}{}
	err := h.DecodeJSON(r, &req)
	if err != nil {
		_ = log.Error(err.Error())
		h.Error400(w, err.Error())
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	user, err := h.User.Get(id)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	user.Password = string(hash)
	user.Updated = time.Now()
	err = h.User.Update(&user)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}

	_ = h.WriteOKJSON(w, api.UpdateResponse(id))
	return

}

func (h APIHandler) SetBuiltinUserAdminDisabled(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	u, _ := global.Env().GetConfig("bootstrap.username", "admin")
	if reqUser.UserId == u {
		h.ErrorInternalServer(w, "you are trying to disable yourself and it is not allowed!")
		return
	}
	disabled :=  h.GetParameter(r, "disabled")
	if disabled == "true" {
		err = api.DisableBuiltinUserAdmin()
	}else{
		err = api.EnableBuiltinUserAdmin()
	}
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}

	h.WriteJSON(w, util.MapStr{
		"result": "updated",
	}, http.StatusOK)
}

func (h APIHandler) GetSecuritySettings(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	h.WriteJSON(w, util.MapStr{
		"admin_disabled": api.IsBuiltinUserAdminDisabled(),
	}, http.StatusOK)
}
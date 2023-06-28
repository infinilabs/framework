/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/security/realm"
	"net/http"
)

const userInSession = "user_session:"

//const SSOProvider = "sso"
const NativeProvider = "native"
//const LDAPProvider = "ldap"

func (h APIHandler) CurrentUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	exists, user := api.GetSession(w, req, userInSession)
	if exists {
		data := util.MapStr{
			"name":      user,
			"avatar":    "",
			"userid":    "10001",
			"email":     "hello@infini.ltd",
			"signature": "极限科技 - 专业的开源搜索与实时数据分析整体解决方案提供商。",
			"title":     "首席设计师",
			"group":     "INFINI Labs",
			"tags": []util.MapStr{
				{
					"key":   "0",
					"label": "很有想法的",
				}},
			"notifyCount": 12,
			"country":     "China",
			"geographic": util.MapStr{
				"province": util.MapStr{
					"label": "湖南省",
					"key":   "330000",
				},
				"city": util.MapStr{
					"label": "长沙市",
					"key":   "330100",
				},
			},
			"address": "岳麓区湘江金融中心",
			"phone":   "4001399200",
		}

		h.WriteJSON(w, data, 200)
	} else {
		data := util.MapStr{
			"status":           "error",
			"type":             "account",
			"currentAuthority": "guest",
		}
		h.WriteJSON(w, data, 403)
	}
}

func (h APIHandler) Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	rbac.DeleteUserToken(reqUser.UserId)
	h.WriteOKJSON(w, util.MapStr{
		"status": "ok",
	})
}

func (h APIHandler) Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	if reqUser.Provider == NativeProvider {
		user, err := h.User.Get(reqUser.UserId)
		if err != nil {
			h.ErrorInternalServer(w, err.Error())
			return
		}
		if user.Nickname == "" {
			user.Nickname = user.Username
		}

		u := util.MapStr{
			"user_id":   user.ID,
			"name":      user.Username,
			"email":     user.Email,
			"nick_name": user.Nickname,
			"phone":     user.Phone,
		}
		h.WriteOKJSON(w, api.FoundResponse(reqUser.UserId, u))
	} else {
		u := util.MapStr{
			"user_id":   reqUser.UserId,
			"name":      reqUser.Username,
			"email":     "",               //TOOD, save user profile come from SSO
			"nick_name": reqUser.Username, //TODO
			"phone":     "",               //TODO
		}
		h.WriteOKJSON(w, api.FoundResponse(reqUser.UserId, u))
	}

}

func (h APIHandler) UpdatePassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	err = h.DecodeJSON(r, &req)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	user, err := h.User.Get(reqUser.UserId)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		h.ErrorInternalServer(w, "old password is not correct")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	user.Password = string(hash)
	err = h.User.Update(&user)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.UpdateResponse(reqUser.UserId))
	return
}

func (h APIHandler) UpdateProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
		Email string `json:"email"`
	}
	err = h.DecodeJSON(r, &req)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	user, err := h.User.Get(reqUser.UserId)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	user.Username = req.Name
	user.Email = req.Email
	user.Phone = req.Phone
	err = h.User.Update(&user)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.UpdateResponse(reqUser.UserId))
	return
}

func (h APIHandler) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	err := h.DecodeJSON(r, &req)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	var user *rbac.User

	//check user validation
	ok,user, err := realm.Authenticate(req.Username, req.Password)
	if err != nil {
		h.WriteError(w,err.Error(),500)
		return
	}

	if !ok{
		h.WriteError(w,"invalid username or password",403)
		return
	}

	if user == nil {
		h.ErrorInternalServer(w, fmt.Sprintf("failed to authenticate user: %v", req.Username))
		return
	}

	//check permissions
	ok,err=realm.Authorize(user)
	if err != nil||!ok {
		h.ErrorInternalServer(w, fmt.Sprintf("failed to authorize user: %v", req.Username))
		return
	}

	//fetch user profile
	//TODO
	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	//generate access token
	token, err := rbac.GenerateAccessToken(user)
	if err != nil{
		h.ErrorInternalServer(w, fmt.Sprintf("failed to authorize user: %v", req.Username))
		return
	}

	//api.SetSession(w, r, userInSession+req.Username, req.Username)
	h.WriteOKJSON(w, token)
}


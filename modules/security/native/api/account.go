/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
	log "github.com/cihub/seelog"
	"time"
)

const userInSession = "user_session:"

func (h APIHandler) authenticateUser(username string, password string) (user *rbac.User, err error) {

	user, err = h.User.GetBy("name", username)
	if err != nil {
		return user, err
	}
	if user == nil {
		return nil, fmt.Errorf("user account [%s] not found", username)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		err = errors.New("incorrect password")
		return
	}
	return
}

func authorize(user rbac.User,provider string) (m map[string]interface{}, err error) {
	var roles, privilege []string
	for _, v := range user.Roles {
		role,ok := rbac.RoleMap[v.Name]
		if ok{
			roles = append(roles, v.Name)
			privilege = append(privilege, role.Privilege.Platform...)
		}
	}

	if len(privilege)==0{
		log.Error("no privilege assigned to user:",user)
		return nil,errors.New("no privilege assigned to this user:"+user.Name)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, rbac.UserClaims{
		ShortUser: &rbac.ShortUser{
			Provider: provider,
			Username: user.Name,
			UserId:   user.ID,
			Roles:    roles,
		},
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	tokenString, err := token.SignedString([]byte(rbac.Secret))
	if err != nil {
		return
	}

	m = util.MapStr{
		"access_token": tokenString,
		"username":     user.Name,
		"id":           user.ID,
		"expire_in":    86400,
		"roles":        roles,
		"privilege":    privilege,
	}
	return
}

const SSOProvider = "sso"
const NativeProvider = "native"

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

	user, err = h.authenticateUser(req.Username, req.Password)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	data, err := authorize(*user,NativeProvider)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	token := rbac.Token{ExpireIn: time.Now().Unix() + 86400}
	rbac.SetUserToken(user.ID, token)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	data["status"] = "ok"

	//api.SetSession(w, r, userInSession+req.Username, req.Username)
	h.WriteOKJSON(w, data)
}

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

	if reqUser.Provider==NativeProvider{
		user, err := h.User.Get(reqUser.UserId)
		if err != nil {
			h.ErrorInternalServer(w, err.Error())
			return
		}
		u := util.MapStr{
			"user_id":  user.ID,
			"name": user.Name,
			"email":    user.Email,
			"nick_name":     user.NickName,
			"phone":    user.Phone,
		}
		h.WriteOKJSON(w, api.FoundResponse(reqUser.UserId, u))
	}else{
		u := util.MapStr{
			"user_id":  reqUser.UserId,
			"name": reqUser.Username,
			"email":   "" , //TOOD, save user profile come from SSO
			"nick_name":     reqUser.Username, //TODO
			"phone":    "", //TODO
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
	user.Name = req.Name
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


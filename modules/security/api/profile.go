/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"net/http"
	"time"
)

func (h APIHandler) Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	user, err := h.User.Get(reqUser.UserId)
	if err != nil {
		h.WriteJSON(w, api.NotFoundResponse(reqUser.UserId), 404)
		return
	}
	user.Payload = nil
	user.Password = ""
	h.WriteOKJSON(w, api.FoundResponse(reqUser.UserId, user))
}

func (h APIHandler) UpdateProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
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

	if req.Email != "" && user.Email != req.Email {
		user.Email = req.Email
		user.EmailVerified = false
	}

	user.Nickname = req.Nickname
	user.Phone = req.Phone

	err = h.User.Update(user)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.UpdateResponse(reqUser.UserId))
}

func (h APIHandler) VerifyProfileEmail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	var req struct {
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

	if !rate.GetRateLimiter("email_verify_token", reqUser.UserId, 1, 1, 1*time.Second).Allow() {
		h.WriteError(w, "request too frequently", 500)
		return
	}

	if req.Email != "" && user.Email == req.Email {

		err:=chekIfOtherUserAlreadyHoldThisEmail(user.Username,user.Email)
		if err!=nil{
			panic(err)
		}

		//tobe updated
		token := util.GetUUID() + util.GenerateRandomString(128)
		cacheObject := util.MapStr{
			"user_id": user.ID,
			"token":   token,
			"email":   req.Email,
			"created": util.ToString(util.GetLowPrecisionCurrentTime().Unix()), //expire after 120s
		}

		err = kv.AddValue("email_verify_token", []byte(user.ID), util.MustToJSONBytes(cacheObject))
		if err != nil {
			panic(err)
		}

		//send this to email queue
		link := fmt.Sprintf("%v/link/_verify_account_email?user=%v&token=%v",
			global.Env().SystemConfig.WebAppConfig.Domain,
			user.ID, token)
		email := util.MapStr{
			"template":  "verify_account_email_en_US",
			"server_id": "infinilabs",
			"email":     req.Email,
			"variables": util.MapStr{
				"name": user.Nickname,
				"link": link,
			},
		}
		err = queue.Push(queue.GetOrInitConfig("email_messages"), util.MustToJSONBytes(email))
		if err != nil {
			panic(err)
		}

		h.WriteAckOKJSON(w)
		return
	}

	h.WriteAckJSON(w, false, 500, nil)

}

func (h APIHandler) ClickVerifyEmailLink(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	user := h.Get(r, "user", "") //user ID
	token := h.Get(r, "token", "")
	if user != "" && token != "" {
		b, err := kv.GetValue("email_verify_token", []byte(user))
		if err != nil {
			panic(err) //TODO human friendly
		}
		o := util.MapStr{}
		err = util.FromJSONBytes(b, &o)
		if err != nil {
			panic(err)
		}
		v, ok := o["user_id"]
		if ok {
			if util.TrimSpaces(v.(string)) == util.TrimSpaces(user) {
				v, ok := o["token"]
				if ok {
					if util.TrimSpaces(v.(string)) == util.TrimSpaces(token) {
						created, ok := o["created"]
						if ok {
							i, err := util.ToInt64(created.(string))
							if err == nil {
								t := util.FromUnixTimestamp(int64(i))
								if time.Since(t) < 120*time.Second {
									v, ok := o["email"]
									if ok {
										email, ok := v.(string)
										if ok {
											old, err := h.User.Get(user)
											if err != nil {
												panic(err)
											}

											if old.Email == email {

												err:=chekIfOtherUserAlreadyHoldThisEmail(old.Username,old.Email)
												if err!=nil{
													panic(err)
												}

												old.ID = user
												old.EmailVerified = true
												err = h.User.Update(old)
												if err != nil {
													panic(err)
												}
												err = kv.DeleteKey("email_verify_token", []byte(user))
												if err != nil {
													panic(err)
												}
												log.Infof("user:%v verified email:%v", user, email)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !rate.GetRateLimiter("email_verify_token", user, 10, 1, 1*time.Second).Allow() {
		h.WriteError(w, "request too frequently", 500)
		return
	}

	api.DefaultAPI.Redirect(w, r, "/")
}

func chekIfOtherUserAlreadyHoldThisEmail(username, email string) error{
	//if no others with this email already verified
	u := rbac.User{}
	cond:=orm.And(orm.Eq("email_verified", true),
		orm.Eq("email", email), orm.NotEq("username", username))
	q:=orm.Query{}
	q.Conds=cond
	err,v:=orm.Search(u, &q)
	if err!=nil{
		return err
	}
	if v.Total>0{
		return errors.Error("this email address has been used by someone")
	}
	return nil
}

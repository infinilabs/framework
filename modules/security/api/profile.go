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
	"infini.sh/framework/core/kv"
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
		//tobe updated
		token := util.GetUUID() + util.GenerateRandomString(128)
		req := util.MapStr{
			"user_id": user.ID,
			"token":   token,
			"email":   req.Email,
			"created": util.ToString(util.GetLowPrecisionCurrentTime().Unix()), //expire after 120s
		}

		err := kv.AddValue("email_verify_token", []byte(user.ID), util.MustToJSONBytes(req))
		if err != nil {
			panic(err)
		}

		//send this to email queue
		link := fmt.Sprintf("/link/_verify_account_email?user=%v&token=%v", user.ID, token)
		log.Info(link) //TODO

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
												old.ID = user
												old.EmailVerified = true
												err := h.User.Update(old)
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

	//TODO redirect to success page
}

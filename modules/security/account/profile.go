/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package account

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"net/http"
)

func init() {
	api.HandleUIMethod(api.GET, "/account/profile", Profile, api.RequireLogin(), api.AllowOPTIONSS(), api.Feature(api.FeatureCORS))
}

func Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !api.IsAuthEnable() {
		api.WriteError(w, "auth is not enabled", http.StatusInternalServerError)
		return
	}

	reqUser, err := security.GetUserFromContext(r.Context())
	if err != nil || reqUser == nil {
		api.WriteError(w, "invalid user", http.StatusUnauthorized)
		return
	}

	p := &security.UserProfile{
		Name:  reqUser.Login,
		Roles: reqUser.Roles,
	}
	p.ID = reqUser.UserID

	if reqUser.Provider == security.DefaultNativeAuthBackend {
		if _, account, err := security.GetUserByID(reqUser.UserID); err == nil && account != nil {
			if account.Name != "" {
				p.Name = account.Name
			}
			p.Email = account.Email
			p.Roles = account.Roles
		}
	}

	//get all permissions for user
	p.Permissions = reqUser.GetPermissionKeys()

	api.WriteJSON(w, p, http.StatusOK)
}

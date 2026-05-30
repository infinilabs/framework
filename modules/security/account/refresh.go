/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package account

import (
	"fmt"
	"net/http"
	"strings"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
)

func init() {
	api.HandleUIMethod(
		api.POST,
		"/account/refresh",
		api.RequireSecureTransport(Refresh, api.SecureTransportOptions{TrustForwardHeaders: true}),
		api.RequireLogin(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS),
	)
}

// Refresh reissues an access token for the current session user while reloading the
// native account record so updated roles/profile data are reflected in new tokens.
func Refresh(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := security.GetUserFromContext(r.Context())
	if err != nil || reqUser == nil {
		api.WriteError(w, "invalid user", http.StatusUnauthorized)
		return
	}

	sessionUser, err := buildRefreshedSession(reqUser)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err, token := security.AddUserToSession(w, r, sessionUser); err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
	} else {
		security.DecorateSessionTokenResponse(token, sessionUser)
		api.WriteOKJSON(w, token)
	}
}

func buildRefreshedSession(reqUser *security.UserSessionInfo) (*security.UserSessionInfo, error) {
	if reqUser == nil {
		return nil, fmt.Errorf("user not found")
	}

	sessionUser := cloneSessionUser(reqUser)
	if reqUser.Provider != security.DefaultNativeAuthBackend {
		return sessionUser, nil
	}

	provider, account, err := security.GetUserByID(reqUser.UserID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("user not found")
	}

	login := strings.TrimSpace(account.Email)
	if login == "" {
		login = strings.TrimSpace(reqUser.Login)
	}
	if provider == "" {
		provider = security.DefaultNativeAuthBackend
	}

	sessionUser = &security.UserSessionInfo{
		Provider:    provider,
		Login:       login,
		Roles:       append([]string(nil), account.Roles...),
		Permissions: append([]security.PermissionKey(nil), reqUser.Permissions...),
		LastLogin:   reqUser.LastLogin,
	}
	sessionUser.SetUserID(account.ID)
	return sessionUser, nil
}

func cloneSessionUser(reqUser *security.UserSessionInfo) *security.UserSessionInfo {
	if reqUser == nil {
		return nil
	}

	sessionUser := &security.UserSessionInfo{
		Provider:    reqUser.Provider,
		Login:       reqUser.Login,
		Roles:       append([]string(nil), reqUser.Roles...),
		Permissions: append([]security.PermissionKey(nil), reqUser.Permissions...),
		LastLogin:   reqUser.LastLogin,
	}
	sessionUser.SetUserID(reqUser.UserID)
	return sessionUser
}

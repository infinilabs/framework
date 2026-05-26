// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package rbac

import (
	"errors"
	"net/http"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	replaysecurity "infini.sh/framework/core/security/replay"
	"infini.sh/framework/core/util"
)

var (
	errInvalidLoginCredentials = errors.New("invalid login or password")
	errIncompleteChallenge     = errors.New("challenge response is incomplete")
	errMissingPassword         = errors.New("password is required")
)

type accountLoginRequest struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	UserName    string `json:"userName"`
	Password    string `json:"password"`
	ChallengeID string `json:"challenge_id"`
	Proof       string `json:"proof"`
}

func registerAccountRoutes() {
	api.HandleUIMethod(api.POST, "/account/replay_nonce",
		api.RequireSecureTransport(IssueReplayNonce),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))

	api.HandleUIMethod(api.POST, "/account/login/challenge",
		api.RequireSecureTransport(LoginChallenge),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))

	api.HandleUIMethod(api.POST, "/account/login",
		api.RequireSecureTransport(Login),
		api.AllowPublicAccess(),
		api.AllowOPTIONSS(),
		api.Feature(api.FeatureCORS))
}

func IssueReplayNonce(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var req struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}

	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	nonce, ttl, err := replaysecurity.IssueReplayNonce(r, req.Method, req.Path)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	api.WriteOKJSON(w, util.MapStr{
		"status":            "ok",
		"nonce":             nonce,
		"expire_in_seconds": int(ttl / time.Second),
	})
}

func LoginChallenge(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var req accountLoginRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	login := req.NormalizedLogin()
	if login == "" {
		api.WriteError(w, "login is required", http.StatusBadRequest)
		return
	}

	exists, user, err := lookupAccountByLogin(login)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.WriteOKJSON(w, buildLoginChallengeResponse(login, exists, user))
}

func Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var req accountLoginRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	login := req.NormalizedLogin()
	if login == "" {
		api.WriteError(w, "login is required", http.StatusBadRequest)
		return
	}

	exists, user, err := lookupAccountByLogin(login)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists || user == nil {
		api.WriteError(w, errInvalidLoginCredentials.Error(), http.StatusForbidden)
		return
	}

	usedChallenge := req.ChallengeID != "" || req.Proof != ""
	if err := validateReplayNonce(r, usedChallenge); err != nil {
		api.WriteError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	usedChallenge, err = authenticateLogin(user, login, req.Password, req.ChallengeID, req.Proof)
	if err != nil {
		statusCode := http.StatusForbidden
		if errors.Is(err, errIncompleteChallenge) || errors.Is(err, errMissingPassword) {
			statusCode = http.StatusBadRequest
		}
		api.WriteError(w, err.Error(), statusCode)
		return
	}

	if !usedChallenge {
		upgradePasswordChallenge(user, req.Password)
	}

	sessionUser := newNativeSession(user, login)
	if err, token := security.AddUserToSession(w, r, sessionUser); err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
	} else {
		api.WriteOKJSON(w, token)
	}
}

func (req accountLoginRequest) NormalizedLogin() string {
	for _, candidate := range []string{req.Login, req.Email, req.Username, req.UserName} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return ""
}

func buildLoginChallengeResponse(login string, exists bool, user *security.UserAccount) util.MapStr {
	if exists && security.CanUsePasswordChallenge(user) {
		challenge := security.NewLoginChallenge(login)
		return util.MapStr{
			"status":       "ok",
			"method":       security.PasswordChallengeMethod,
			"algorithm":    security.PasswordChallengeAlgorithm,
			"iterations":   security.PasswordChallengeIterations,
			"challenge_id": challenge.ID,
			"nonce":        challenge.Nonce,
			"salt":         user.PasswordSalt,
		}
	}

	return util.MapStr{
		"status": "ok",
		"method": "plain",
	}
}

func authenticateLogin(user *security.UserAccount, login, password, challengeID, proof string) (bool, error) {
	if user == nil {
		return false, errInvalidLoginCredentials
	}

	if challengeID != "" || proof != "" {
		if challengeID == "" || proof == "" {
			return true, errIncompleteChallenge
		}

		challenge, err := security.ConsumeLoginChallenge(challengeID, login)
		if err != nil || !security.CanUsePasswordChallenge(user) {
			return true, errInvalidLoginCredentials
		}
		if !security.VerifyPasswordProof(user.PasswordVerifier, login, challenge.ID, challenge.Nonce, proof) {
			return true, errInvalidLoginCredentials
		}
		return true, nil
	}

	if password == "" {
		return false, errMissingPassword
	}
	if err := security.VerifyPassword(user, password); err != nil {
		return false, errInvalidLoginCredentials
	}
	return false, nil
}

func lookupAccountByLogin(login string) (bool, *security.UserAccount, error) {
	exists, user, err := GetUserByLogin(login)
	if err != nil && err.Error() == "not found" {
		return false, nil, nil
	}
	return exists, user, err
}

func validateReplayNonce(r *http.Request, required bool) error {
	nonce := strings.TrimSpace(r.Header.Get(replaysecurity.HeaderName))
	if nonce == "" && !required {
		return nil
	}
	return replaysecurity.ValidateAndConsumeReplayNonce(r)
}

func upgradePasswordChallenge(user *security.UserAccount, password string) {
	if user == nil || password == "" || security.CanUsePasswordChallenge(user) {
		return
	}

	if err := security.EnsurePasswordChallenge(user, password); err != nil {
		log.Warnf("failed to derive password challenge for user [%s]: %v", user.Email, err)
		return
	}

	ctx := orm.NewContext()
	ctx.DirectAccess()
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.Update(ctx, user); err != nil {
		log.Warnf("failed to persist password challenge for user [%s]: %v", user.Email, err)
	}
}

func newNativeSession(user *security.UserAccount, login string) *security.UserSessionInfo {
	userLogin := strings.TrimSpace(user.Email)
	if userLogin == "" {
		userLogin = login
	}

	session := &security.UserSessionInfo{
		Provider: security.DefaultNativeAuthBackend,
		Login:    userLogin,
		Roles:    append([]string(nil), user.Roles...),
	}
	session.SetUserID(user.ID)
	return session
}

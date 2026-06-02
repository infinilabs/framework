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

var persistPasswordChallengeUpgrade = func(ctx *orm.Context, user *security.UserAccount) error {
	return orm.Save(ctx, user)
}

var (
	// Keep the password and challenge paths aligned on one user-facing failure message.
	errInvalidLoginCredentials = errors.New("invalid login or password")
	// A challenge login must send both the one-time challenge id and the derived proof.
	errIncompleteChallenge = errors.New("challenge response is incomplete")
	// Password login keeps requiring the legacy password field when no challenge proof is supplied.
	errMissingPassword = errors.New("password is required")
)

func shouldCollapseLoginError(err error) bool {
	if err == nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(err.Error())) {
	case "user not found":
		return true
	default:
		return false
	}
}

// accountLoginRequest accepts both the framework-native "login" field and the aliases
// already used by existing clients while challenge login is rolled out incrementally.
type accountLoginRequest struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	UserName    string `json:"userName"`
	Password    string `json:"password"`
	ChallengeID string `json:"challenge_id"`
	Proof       string `json:"proof"`
}

// IssueReplayNonce mints a short-lived nonce bound to the caller and target request scope.
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

// LoginChallenge tells the client whether this account can use challenge login and, if so,
// returns the one-time challenge payload required to derive the proof locally.
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

	exists, user, err := security.GetUserByLogin(login)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.WriteOKJSON(w, buildLoginChallengeResponse(login, exists, user))
}

// Login accepts either the legacy password payload or the new challenge proof and then
// reuses the existing session/token issuance path once the credentials are verified.
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

	usedChallenge := req.ChallengeID != "" || req.Proof != ""
	exists, user, err := security.GetUserByLogin(login)
	if err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if usedChallenge && (!exists || user == nil) {
		api.WriteError(w, errInvalidLoginCredentials.Error(), http.StatusForbidden)
		return
	}

	if err := validateReplayNonce(r, usedChallenge); err != nil {
		api.WriteError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	usedChallenge, sessionUser, nativeUser, err := authenticateLogin(user, login, req.Password, req.ChallengeID, req.Proof)
	if err != nil {
		statusCode := http.StatusForbidden
		if errors.Is(err, errIncompleteChallenge) || errors.Is(err, errMissingPassword) {
			statusCode = http.StatusBadRequest
		}
		api.WriteError(w, err.Error(), statusCode)
		return
	}

	if !usedChallenge && nativeUser != nil {
		upgradePasswordChallenge(nativeUser, login, req.Password)
	}

	if err, token := security.AddUserToSession(w, r, sessionUser); err != nil {
		api.WriteError(w, err.Error(), http.StatusInternalServerError)
	} else {
		security.DecorateSessionTokenResponse(token, sessionUser)
		api.WriteOKJSON(w, token)
	}
}

// NormalizedLogin resolves the various historical request field names into one lookup key.
func (req accountLoginRequest) NormalizedLogin() string {
	for _, candidate := range []string{req.Login, req.Email, req.Username, req.UserName} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return ""
}

// buildLoginChallengeResponse returns plain login for existing legacy accounts that have
// not been upgraded with challenge material yet. Accounts that do not exist still receive
// a fake challenge payload to avoid user enumeration.
func buildLoginChallengeResponse(login string, exists bool, user *security.UserAccount) util.MapStr {
	if exists && !security.CanUsePasswordChallenge(user) {
		return util.MapStr{
			"status": "ok",
			"method": "plain",
		}
	}

	salt := util.GenerateSecureString(32)
	if exists && security.CanUsePasswordChallenge(user) {
		salt = user.PasswordSalt
	}

	// The challenge payload gives clients everything needed to derive a proof
	// locally without sending the raw password back to the server.
	challenge := security.NewLoginChallenge(login)
	return util.MapStr{
		"status":       "ok",
		"method":       security.PasswordChallengeMethod,
		"algorithm":    security.PasswordChallengeAlgorithm,
		"iterations":   security.PasswordChallengeIterations,
		"challenge_id": challenge.ID,
		"nonce":        challenge.Nonce,
		"salt":         salt,
	}
}

// authenticateLogin selects the correct credential validation path based on the request body.
func authenticateLogin(user *security.UserAccount, login, password, challengeID, proof string) (bool, *security.UserSessionInfo, *security.UserAccount, error) {
	if challengeID != "" || proof != "" {
		if challengeID == "" || proof == "" {
			return true, nil, nil, errIncompleteChallenge
		}

		if user == nil {
			return true, nil, nil, errInvalidLoginCredentials
		}
		challenge, err := security.ConsumeLoginChallenge(challengeID, login)
		if err != nil || !security.CanUsePasswordChallenge(user) {
			return true, nil, nil, errInvalidLoginCredentials
		}
		if !security.VerifyPasswordProof(user.PasswordVerifier, login, challenge.ID, challenge.Nonce, proof) {
			return true, nil, nil, errInvalidLoginCredentials
		}
		return true, newNativeSession(user, login), user, nil
	}

	if password == "" {
		return false, nil, nil, errMissingPassword
	}

	if user != nil {
		if err := security.VerifyPassword(user, password); err == nil {
			return false, newNativeSession(user, login), user, nil
		}
	}

	sessionUser, err := security.AuthenticateAccountPasswordLogin(login, password)
	if err != nil {
		if shouldCollapseLoginError(err) {
			return false, nil, nil, errInvalidLoginCredentials
		}
		return false, nil, nil, err
	}
	if sessionUser != nil {
		return false, sessionUser, nil, nil
	}

	return false, nil, nil, errInvalidLoginCredentials
}

// validateReplayNonce keeps challenge login replay-safe while leaving older password-only
// clients working until they adopt the explicit nonce negotiation endpoint.
func validateReplayNonce(r *http.Request, required bool) error {
	nonce := strings.TrimSpace(r.Header.Get(replaysecurity.HeaderName))
	if nonce == "" && !required {
		// Keep the original password login path backward compatible: upgraded clients
		// send replay nonces, while older clients can still post passwords directly.
		return nil
	}
	return replaysecurity.ValidateAndConsumeReplayNonce(r)
}

// upgradePasswordChallenge backfills verifier material after a successful legacy login so
// existing native accounts can move onto the challenge flow without an offline migration.
func upgradePasswordChallenge(user *security.UserAccount, login, password string) {
	if user == nil || password == "" || security.CanUsePasswordChallenge(user) {
		return
	}

	if err := security.EnsurePasswordChallenge(user, password); err != nil {
		log.Warnf("failed to derive password challenge for user [%s]: %v", user.Email, err)
		return
	}

	// Persist the verifier after a successful legacy password login so subsequent
	// logins can move onto the challenge flow without an explicit migration step.
	// This upgrade is best-effort; the current login already succeeded, so it should
	// not wait for an index refresh before returning to the caller.
	if user.ID == "" {
		userLogin := strings.TrimSpace(user.Email)
		if userLogin == "" {
			userLogin = strings.TrimSpace(login)
		}
		if userLogin != "" {
			user.ID = getUIDByEmail(userLogin)
		}
	}
	ctx := orm.NewContext()
	ctx.DirectAccess()
	if err := persistPasswordChallengeUpgrade(ctx, user); err != nil {
		log.Warnf("failed to persist password challenge for user [%s]: %v", user.Email, err)
	}
}

// newNativeSession converts a native account record into the existing framework session claims.
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

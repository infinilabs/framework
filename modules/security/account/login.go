/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package account

import (
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Unified password login — POST /account/login
//
// This is the framework-level login endpoint shared by all applications. It
// works with any registered AuthenticationBackend (static, native, ...) via
// security.GetUserByLogin, performs bcrypt password verification, and creates
// a JWT session via security.AddUserToSession.
//
// Request (JSON):  {"login": "<username or email>", "password": "<password>"}
// Response (200):  {"access_token": "<jwt>", "expire_in": <unix>, "status": "ok"}
// Errors:          401 (invalid credentials), 429 (rate limited), 400 (bad request)
//
// Rate limiting: 10 attempts per minute per client IP.
// ──────────────────────────────────────────────────────────────────────────

func init() {
	// Register only when auth is enabled; AllowPublicAccess because the login
	// endpoint itself must be reachable without an existing session.
	api.HandleUIMethod(api.POST, "/account/login", Login, api.AllowPublicAccess(), api.AllowOPTIONSS(), api.Feature(api.FeatureCORS))
}

// loginRequest is the JSON body for POST /account/login.
type loginRequest struct {
	Login    string `json:"login"`    // username or email
	Password string `json:"password"` // plaintext password
}

// Login handles password-based authentication.
func Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !api.IsAuthEnable() {
		api.WriteError(w, "auth is not enabled", http.StatusInternalServerError)
		return
	}

	// Rate limit: 10 login attempts per minute per client IP.
	clientIP := util.ClientIP(r)
	if !rate.GetRateLimiter("login", clientIP, 10, 10, 1*time.Minute).Allow() {
		api.WriteError(w, "too many login attempts, please try again later", http.StatusTooManyRequests)
		return
	}

	var req loginRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	login := strings.TrimSpace(req.Login)
	password := req.Password
	if login == "" || password == "" {
		api.WriteError(w, "login and password are required", http.StatusBadRequest)
		return
	}

	// Look up the user across all registered authentication backends (static,
	// native, etc.). GetUserByLogin iterates the providers and returns the
	// first match.
	exists, account, err := security.GetUserByLogin(login)
	if err != nil || !exists || account == nil {
		api.WriteError(w, "invalid login or password", http.StatusUnauthorized)
		return
	}

	// Verify the password against the stored bcrypt hash.
	if account.Password == "" {
		// No password set on this account — cannot log in via password.
		api.WriteError(w, "invalid login or password", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		api.WriteError(w, "invalid login or password", http.StatusUnauthorized)
		return
	}

	// Build the session info from the verified account.
	sessionInfo := &security.UserSessionInfo{
		Provider: account.Email, // the login identifier used
		Login:    login,
		Roles:    account.Roles,
	}
	sessionInfo.SetUserID(account.ID)

	// Create the session: generates a JWT (24h) and stores it in the session
	// cookie. Subsequent requests are authenticated by the session_token
	// auth filter provider (priority 10).
	tokenErr, token := security.AddUserToSession(w, r, sessionInfo)
	if tokenErr != nil {
		api.WriteError(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	api.WriteJSON(w, token, http.StatusOK)
}

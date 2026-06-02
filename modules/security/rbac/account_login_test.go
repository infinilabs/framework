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
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	replaysecurity "infini.sh/framework/core/security/replay"
)

type testAccountPasswordLoginProvider struct{}
type testMissingUserAccountPasswordLoginProvider struct{}

type testChallengeAuthenticationBackend struct{}

func (testChallengeAuthenticationBackend) GetUserByID(id string) (bool, *security.UserAccount, error) {
	return false, nil, nil
}

func (testChallengeAuthenticationBackend) GetUserByLogin(login string) (bool, *security.UserAccount, error) {
	if login != "bridge-admin" {
		return false, nil, nil
	}
	user := &security.UserAccount{Email: "bridge-admin"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		return false, nil, err
	}
	user.ID = "bridge-admin-id"
	return true, user, nil
}

func (testChallengeAuthenticationBackend) CreateUser(name, login, password string, force bool) (*security.UserAccount, error) {
	return nil, nil
}

func (testAccountPasswordLoginProvider) AuthenticateByPassword(login, password string) (*security.UserSessionInfo, error) {
	if login != "ldap-user" || password != "StrongPassw0rd!" {
		return nil, nil
	}

	sessionUser := &security.UserSessionInfo{
		Provider: "ldap",
		Login:    login,
		Roles:    []string{"viewer"},
	}
	sessionUser.SetUserID("ldap-user-id")
	return sessionUser, nil
}

func (testMissingUserAccountPasswordLoginProvider) AuthenticateByPassword(login, password string) (*security.UserSessionInfo, error) {
	if login != "missing-user" {
		return nil, nil
	}

	return nil, errors.New("user not found")
}

// The request payload accepts multiple historical login field names during rollout.
func TestAccountLoginRequestNormalizedLogin(t *testing.T) {
	req := accountLoginRequest{
		Email:    "admin@example.org",
		Username: "ignored@example.org",
	}

	if got := req.NormalizedLogin(); got != "admin@example.org" {
		t.Fatalf("expected email to be preferred, got %q", got)
	}
}

// Password login remains the backward-compatible path for accounts and clients not yet upgraded.
func TestAuthenticateLoginWithPassword(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	usedChallenge, sessionUser, nativeUser, err := authenticateLogin(user, user.Email, "StrongPassw0rd!", "", "")
	if err != nil {
		t.Fatalf("authenticate login: %v", err)
	}
	if usedChallenge {
		t.Fatal("expected password login path")
	}
	if sessionUser == nil || nativeUser == nil {
		t.Fatalf("expected native password login state, got session=%#v native=%#v", sessionUser, nativeUser)
	}
}

// Challenge login should succeed once the account already has verifier material.
func TestAuthenticateLoginWithChallenge(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	challenge := security.NewLoginChallenge(user.Email)
	proof, err := security.BuildPasswordProof(user.PasswordVerifier, user.Email, challenge.ID, challenge.Nonce)
	if err != nil {
		t.Fatalf("build password proof: %v", err)
	}

	usedChallenge, sessionUser, nativeUser, err := authenticateLogin(user, user.Email, "", challenge.ID, proof)
	if err != nil {
		t.Fatalf("authenticate login: %v", err)
	}
	if !usedChallenge {
		t.Fatal("expected challenge login path")
	}
	if sessionUser == nil || nativeUser == nil {
		t.Fatalf("expected native challenge login state, got session=%#v native=%#v", sessionUser, nativeUser)
	}
}

// Partially supplied challenge payloads should fail distinctly from bad credentials.
func TestAuthenticateLoginRejectsIncompleteChallenge(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	_, _, _, err := authenticateLogin(user, user.Email, "", "challenge-id", "")
	if !errors.Is(err, errIncompleteChallenge) {
		t.Fatalf("expected incomplete challenge error, got %v", err)
	}
}

// Incorrect proofs should collapse to the same user-facing error as bad passwords.
func TestAuthenticateLoginRejectsWrongProof(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	challenge := security.NewLoginChallenge(user.Email)
	_, _, _, err := authenticateLogin(user, user.Email, "", challenge.ID, "bad-proof")
	if !errors.Is(err, errInvalidLoginCredentials) {
		t.Fatalf("expected invalid credential error, got %v", err)
	}
}

// Applications can attach non-native password realms to the shared framework login flow.
func TestAuthenticateLoginFallsBackToRegisteredPasswordProvider(t *testing.T) {
	security.RegisterAccountPasswordLoginProvider("test-account-login", testAccountPasswordLoginProvider{})

	usedChallenge, sessionUser, nativeUser, err := authenticateLogin(nil, "ldap-user", "StrongPassw0rd!", "", "")
	if err != nil {
		t.Fatalf("authenticate login: %v", err)
	}
	if usedChallenge {
		t.Fatal("expected password fallback path")
	}
	if nativeUser != nil {
		t.Fatalf("expected no native user for fallback path, got %#v", nativeUser)
	}
	if sessionUser == nil || sessionUser.Provider != "ldap" {
		t.Fatalf("expected ldap session user, got %#v", sessionUser)
	}
}

func TestAuthenticateLoginCollapsesMissingUserProviderError(t *testing.T) {
	security.RegisterAccountPasswordLoginProvider("test-account-login-missing-user", testMissingUserAccountPasswordLoginProvider{})

	_, _, _, err := authenticateLogin(nil, "missing-user", "StrongPassw0rd!", "", "")
	if !errors.Is(err, errInvalidLoginCredentials) {
		t.Fatalf("expected invalid credential error for missing user, got %v", err)
	}
}

// Older accounts intentionally advertise plain login until their verifier is available.
func TestBuildLoginChallengeResponseFallsBackToPlain(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	resp := buildLoginChallengeResponse(user.Email, true, user)

	if got := resp["method"]; got != "plain" {
		t.Fatalf("expected plain fallback, got %v", got)
	}
	if _, ok := resp["challenge_id"]; ok {
		t.Fatal("did not expect challenge payload for plain fallback")
	}
}

func TestBuildLoginChallengeResponseFakesChallengeForMissingUser(t *testing.T) {
	resp := buildLoginChallengeResponse("missing@example.org", false, nil)

	if got := resp["method"]; got != security.PasswordChallengeMethod {
		t.Fatalf("expected fake challenge method for missing user, got %v", got)
	}
	if resp["challenge_id"] == "" {
		t.Fatal("expected fake challenge id for missing user")
	}
	if resp["nonce"] == "" {
		t.Fatal("expected fake nonce for missing user")
	}
	if resp["salt"] == "" {
		t.Fatal("expected fake salt for missing user")
	}
}

// Upgraded accounts should return the exact challenge inputs the client needs next.
func TestBuildLoginChallengeResponseReturnsChallenge(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	resp := buildLoginChallengeResponse(user.Email, true, user)
	if got := resp["method"]; got != security.PasswordChallengeMethod {
		t.Fatalf("expected challenge method, got %v", got)
	}
	if resp["challenge_id"] == "" {
		t.Fatal("expected challenge id to be returned")
	}
	if resp["nonce"] == "" {
		t.Fatal("expected nonce to be returned")
	}
	if resp["salt"] != user.PasswordSalt {
		t.Fatal("expected challenge response to expose password salt")
	}
}

func TestLoginChallengeUsesRegisteredAuthenticationBackend(t *testing.T) {
	security.RegisterAuthenticationProvider("test-login-challenge-provider", testChallengeAuthenticationBackend{})

	body := bytes.NewBufferString(`{"login":"bridge-admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/account/login/challenge", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	LoginChallenge(recorder, req, nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["method"]; got != security.PasswordChallengeMethod {
		t.Fatalf("expected challenge method from registered provider, got %v", got)
	}
	if resp["challenge_id"] == "" {
		t.Fatal("expected challenge id from registered provider")
	}
}

// Legacy password clients keep working even before they learn the replay-nonce preflight.
func TestValidateReplayNonceAllowsLegacyPasswordLoginWithoutNonce(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	if err := validateReplayNonce(req, false); err != nil {
		t.Fatalf("expected missing nonce to be allowed for legacy password login, got %v", err)
	}
}

// Challenge logins must enforce nonce usage immediately because the frontend already negotiated it.
func TestValidateReplayNonceRequiresNonceForChallengeLogin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	if err := validateReplayNonce(req, true); err == nil {
		t.Fatal("expected missing nonce to be rejected for challenge login")
	}
}

// Once a nonce is explicitly issued for /account/login it should validate on that exact route.
func TestValidateReplayNonceConsumesIssuedNonce(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	nonce, _, err := replaysecurity.IssueReplayNonce(req, http.MethodPost, "/account/login")
	if err != nil {
		t.Fatalf("issue replay nonce: %v", err)
	}
	req.Header.Set(replaysecurity.HeaderName, nonce)

	if err := validateReplayNonce(req, true); err != nil {
		t.Fatalf("expected issued nonce to validate, got %v", err)
	}
}

// Native sessions should still be constructible even when the stored account email is blank.
func TestNewNativeSessionFallsBackToRequestedLogin(t *testing.T) {
	user := &security.UserAccount{Email: "", Roles: []string{security.RoleAdmin}}
	user.ID = "user-1"

	session := newNativeSession(user, "admin@example.org")
	if session.Login != "admin@example.org" {
		t.Fatalf("expected requested login fallback, got %q", session.Login)
	}
	if session.Provider != security.DefaultNativeAuthBackend {
		t.Fatalf("expected native provider, got %q", session.Provider)
	}
}

func TestUpgradePasswordChallengePersistsLegacyAdminByLogin(t *testing.T) {
	originalPersist := persistPasswordChallengeUpgrade
	defer func() {
		persistPasswordChallengeUpgrade = originalPersist
	}()

	var persisted *security.UserAccount
	persistPasswordChallengeUpgrade = func(ctx *orm.Context, user *security.UserAccount) error {
		copied := *user
		persisted = &copied
		return nil
	}

	user := &security.UserAccount{Name: "admin"}
	hash, err := bcrypt.GenerateFromPassword([]byte("StrongPassw0rd!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	user.Password = string(hash)

	upgradePasswordChallenge(user, "admin", "StrongPassw0rd!")

	if persisted == nil {
		t.Fatal("expected legacy admin upgrade to be persisted")
	}
	if persisted.ID != getUIDByEmail("admin") {
		t.Fatalf("expected fallback id %q, got %q", getUIDByEmail("admin"), persisted.ID)
	}
	if persisted.PasswordSalt == "" || persisted.PasswordVerifier == "" {
		t.Fatal("expected challenge credentials to be populated before persisting")
	}
}

func TestUpgradePasswordChallengeSkipsExistingChallengeUser(t *testing.T) {
	originalPersist := persistPasswordChallengeUpgrade
	defer func() {
		persistPasswordChallengeUpgrade = originalPersist
	}()

	called := false
	persistPasswordChallengeUpgrade = func(ctx *orm.Context, user *security.UserAccount) error {
		called = true
		return nil
	}

	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	upgradePasswordChallenge(user, user.Email, "StrongPassw0rd!")
	if !security.CanUsePasswordChallenge(user) {
		t.Fatal("expected challenge material to be available")
	}
	called = false

	upgradePasswordChallenge(user, user.Email, "StrongPassw0rd!")
	if called {
		t.Fatal("did not expect already-upgraded account to be persisted again")
	}
}

// The framework login response keeps the console frontend contract while the handler
// implementation moves from console into framework-owned routes.
func TestDecorateLoginResponseAddsConsoleCompatibilityFields(t *testing.T) {
	session := &security.UserSessionInfo{
		Provider:    security.DefaultNativeAuthBackend,
		Login:       "admin@example.org",
		Roles:       []string{security.RoleAdmin},
		Permissions: []security.PermissionKey{security.GetSimplePermission("generic", "unit", security.Read)},
	}
	session.SetUserID("user-1")

	token := map[string]interface{}{
		"status":    "ok",
		"expire_in": time.Now().Unix() + 3600,
	}
	security.DecorateSessionTokenResponse(token, session)

	if token["username"] != session.Login {
		t.Fatalf("expected username %q, got %v", session.Login, token["username"])
	}
	if token["id"] != session.UserID {
		t.Fatalf("expected id %q, got %v", session.UserID, token["id"])
	}
	if token["expires_at"] == nil {
		t.Fatal("expected expires_at to be populated")
	}
	if expireIn, ok := token["expire_in"].(int64); !ok || expireIn <= 0 || expireIn > 3600 {
		t.Fatalf("expected expire_in to become remaining lifetime seconds, got %#v", token["expire_in"])
	}
	privilege, ok := token["privilege"].([]security.PermissionKey)
	if !ok || len(privilege) == 0 {
		t.Fatalf("expected privilege list to be populated, got %#v", token["privilege"])
	}
}

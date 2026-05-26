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
	"net/http/httptest"
	"testing"

	"infini.sh/framework/core/security"
	replaysecurity "infini.sh/framework/core/security/replay"
)

func TestAccountLoginRequestNormalizedLogin(t *testing.T) {
	req := accountLoginRequest{
		Email:    "admin@example.org",
		Username: "ignored@example.org",
	}

	if got := req.NormalizedLogin(); got != "admin@example.org" {
		t.Fatalf("expected email to be preferred, got %q", got)
	}
}

func TestAuthenticateLoginWithPassword(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	usedChallenge, err := authenticateLogin(user, user.Email, "StrongPassw0rd!", "", "")
	if err != nil {
		t.Fatalf("authenticate login: %v", err)
	}
	if usedChallenge {
		t.Fatal("expected password login path")
	}
}

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

	usedChallenge, err := authenticateLogin(user, user.Email, "", challenge.ID, proof)
	if err != nil {
		t.Fatalf("authenticate login: %v", err)
	}
	if !usedChallenge {
		t.Fatal("expected challenge login path")
	}
}

func TestAuthenticateLoginRejectsIncompleteChallenge(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	_, err := authenticateLogin(user, user.Email, "", "challenge-id", "")
	if !errors.Is(err, errIncompleteChallenge) {
		t.Fatalf("expected incomplete challenge error, got %v", err)
	}
}

func TestAuthenticateLoginRejectsWrongProof(t *testing.T) {
	user := &security.UserAccount{Email: "admin@example.org"}
	if err := security.SetPassword(user, "StrongPassw0rd!"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	challenge := security.NewLoginChallenge(user.Email)
	_, err := authenticateLogin(user, user.Email, "", challenge.ID, "bad-proof")
	if !errors.Is(err, errInvalidLoginCredentials) {
		t.Fatalf("expected invalid credential error, got %v", err)
	}
}

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

func TestValidateReplayNonceAllowsLegacyPasswordLoginWithoutNonce(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	if err := validateReplayNonce(req, false); err != nil {
		t.Fatalf("expected missing nonce to be allowed for legacy password login, got %v", err)
	}
}

func TestValidateReplayNonceRequiresNonceForChallengeLogin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	if err := validateReplayNonce(req, true); err == nil {
		t.Fatal("expected missing nonce to be rejected for challenge login")
	}
}

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

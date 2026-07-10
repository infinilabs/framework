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

package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A nonce is one-time use by design, so any second validation attempt must fail.
func TestReplayNonceCanOnlyBeUsedOnce(t *testing.T) {
	store := NewStore(StoreOptions{})
	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)

	nonce, _, err := store.IssueReplayNonce(req, http.MethodPost, "/account/login")
	if err != nil {
		t.Fatalf("issue replay nonce failed: %v", err)
	}

	req.Header.Set(HeaderName, nonce)
	if err := store.ValidateAndConsumeReplayNonce(req); err != nil {
		t.Fatalf("expected first nonce use to succeed: %v", err)
	}
	if err := store.ValidateAndConsumeReplayNonce(req); err == nil {
		t.Fatal("expected second nonce use to be rejected")
	}
}

// Replay tokens should stay bound to the caller identity, not just the raw path/method tuple.
func TestReplayNonceBindsToAuthorizationHeader(t *testing.T) {
	store := NewStore(StoreOptions{})
	issueReq := httptest.NewRequest(http.MethodPut, "https://console.local/credential/test", nil)
	issueReq.Header.Set("Authorization", "Bearer token-a")

	nonce, _, err := store.IssueReplayNonce(issueReq, http.MethodPut, "/credential/test")
	if err != nil {
		t.Fatalf("issue replay nonce failed: %v", err)
	}

	useReq := httptest.NewRequest(http.MethodPut, "https://console.local/credential/test", nil)
	useReq.Header.Set(HeaderName, nonce)
	useReq.Header.Set("Authorization", "Bearer token-b")
	if err := store.ValidateAndConsumeReplayNonce(useReq); err == nil {
		t.Fatal("expected nonce bound to a different authorization header to fail")
	}
}

// The scope includes HTTP method so a nonce issued for one mutation cannot authorize another.
func TestReplayNonceBindsToPathAndMethod(t *testing.T) {
	store := NewStore(StoreOptions{})
	issueReq := httptest.NewRequest(http.MethodPost, "https://console.local/setup/_initialize", nil)

	nonce, _, err := store.IssueReplayNonce(issueReq, http.MethodPost, "/setup/_initialize")
	if err != nil {
		t.Fatalf("issue replay nonce failed: %v", err)
	}

	useReq := httptest.NewRequest(http.MethodPut, "https://console.local/setup/_initialize", nil)
	useReq.Header.Set(HeaderName, nonce)
	if err := store.ValidateAndConsumeReplayNonce(useReq); err == nil {
		t.Fatal("expected nonce with mismatched method to fail")
	}
}

// Anonymous callers still need a stable default subject so unauthenticated setup flows work.
func TestDefaultSubjectExtractorFallsBackToAnonymous(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)
	if got := DefaultSubjectExtractor(req); got != "anonymous" {
		t.Fatalf("expected anonymous subject, got %q", got)
	}
}

// Path normalization lets clients request a nonce with equivalent path forms safely.
func TestReplayNonceNormalizesPath(t *testing.T) {
	store := NewStore(StoreOptions{})
	issueReq := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)

	nonce, _, err := store.IssueReplayNonce(issueReq, http.MethodPost, "account/../account/login")
	if err != nil {
		t.Fatalf("issue replay nonce failed: %v", err)
	}

	useReq := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)
	useReq.Header.Set(HeaderName, nonce)
	if err := store.ValidateAndConsumeReplayNonce(useReq); err != nil {
		t.Fatalf("expected normalized path to validate: %v", err)
	}
}

// Expired nonces should be rejected even if the caller, method, and path still match.
func TestReplayNonceExpires(t *testing.T) {
	store := NewStore(StoreOptions{TTL: time.Millisecond})
	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)

	nonce, _, err := store.IssueReplayNonce(req, http.MethodPost, "/account/login")
	if err != nil {
		t.Fatalf("issue replay nonce failed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	req.Header.Set(HeaderName, nonce)
	if err := store.ValidateAndConsumeReplayNonce(req); err == nil {
		t.Fatal("expected expired nonce to be rejected")
	}
}

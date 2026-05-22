package replay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

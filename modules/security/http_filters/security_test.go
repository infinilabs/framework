/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package http_filters

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

// Secure-transport enforcement should stop the request before the wrapped UI handler runs.
func TestSecurityFilterSecureTransportFeature(t *testing.T) {
	filter := &SecurityFilter{}
	options := &api.HandlerOptions{}
	api.SecureTransportOption()(options)

	called := false
	protected := filter.ApplyFilter(http.MethodPost, "/account/login", options, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "http://console.local/account/login", nil)
	resp := httptest.NewRecorder()
	protected(resp, req, nil)

	if called {
		t.Fatal("expected insecure request to be blocked")
	}
	if resp.Code != http.StatusUpgradeRequired {
		t.Fatalf("expected status %d, got %d", http.StatusUpgradeRequired, resp.Code)
	}
}

// When a nonce matches the request scope, the filter should behave like a no-op wrapper.
func TestSecurityFilterReplayProtectionFeature(t *testing.T) {
	filter := &SecurityFilter{}
	options := &api.HandlerOptions{}
	api.ReplayProtectionOption()(options)

	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)
	nonce, _, err := replaysecurity.IssueReplayNonce(req, http.MethodPost, "/account/login")
	if err != nil {
		t.Fatalf("issue replay nonce: %v", err)
	}
	req.Header.Set(replaysecurity.HeaderName, nonce)

	called := false
	protected := filter.ApplyFilter(http.MethodPost, "/account/login", options, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	resp := httptest.NewRecorder()
	protected(resp, req, nil)

	if !called {
		t.Fatal("expected replay-protected handler to run")
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

// Missing nonce headers must block replay-protected routes before business logic executes.
func TestSecurityFilterReplayProtectionRejectsMissingNonce(t *testing.T) {
	filter := &SecurityFilter{}
	options := &api.HandlerOptions{}
	api.ReplayProtectionOption()(options)

	called := false
	protected := filter.ApplyFilter(http.MethodPost, "/account/login", options, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)
	resp := httptest.NewRecorder()
	protected(resp, req, nil)

	if called {
		t.Fatal("expected missing nonce to block handler execution")
	}
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}
}

// Trusted forward headers let deployments behind HTTPS reverse proxies pass transport checks.
func TestSecurityFilterWithTrustedForwardHeaders(t *testing.T) {
	filter := &SecurityFilter{}
	options := &api.HandlerOptions{}
	api.SecureTransportOption(api.SecureTransportOptions{TrustForwardHeaders: true})(options)

	called := false
	protected := filter.ApplyFilter(http.MethodPost, "/account/login", options, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "http://console.local/account/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := httptest.NewRecorder()
	protected(resp, req, nil)

	if !called {
		t.Fatal("expected trusted forwarded proto request to be allowed")
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

// Routes that do not opt into trusted proxy headers should stay conservative by default.
func TestTrustForwardHeadersFromOptionsDefaultsFalse(t *testing.T) {
	if trustForwardHeadersFromOptions(nil) {
		t.Fatal("expected nil options to disable trusted forward headers")
	}
	if trustForwardHeadersFromOptions(&api.HandlerOptions{}) {
		t.Fatal("expected missing label to disable trusted forward headers")
	}
}

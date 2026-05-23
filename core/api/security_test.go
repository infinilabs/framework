package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

func TestRequestUsesSecureTransport(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(req *http.Request)
		options []SecureTransportOptions
		secure  bool
	}{
		{
			name: "tls request",
			setup: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{}
			},
			secure: true,
		},
		{
			name: "forwarded proto requires opt in",
			setup: func(req *http.Request) {
				req.Header.Set("X-Forwarded-Proto", "https")
			},
			secure: false,
		},
		{
			name: "forwarded proto trusted when enabled",
			setup: func(req *http.Request) {
				req.Header.Set("X-Forwarded-Proto", "https")
			},
			options: []SecureTransportOptions{{TrustForwardHeaders: true}},
			secure:  true,
		},
		{
			name:   "plain http",
			setup:  func(req *http.Request) {},
			secure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://console.local/account/login", nil)
			tt.setup(req)

			if RequestUsesSecureTransport(req, tt.options...) != tt.secure {
				t.Fatalf("expected secure=%v", tt.secure)
			}
		})
	}
}

func TestRequireSecureTransport(t *testing.T) {
	handler := Handler{}
	called := false
	protected := handler.RequireSecureTransport(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func TestRequireReplayProtection(t *testing.T) {
	handler := Handler{}
	req := httptest.NewRequest(http.MethodPost, "https://console.local/account/login", nil)
	nonce, _, err := replaysecurity.IssueReplayNonce(req, http.MethodPost, "/account/login")
	if err != nil {
		t.Fatalf("issue replay nonce: %v", err)
	}
	req.Header.Set(replaysecurity.HeaderName, nonce)

	called := false
	protected := handler.RequireReplayProtection(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

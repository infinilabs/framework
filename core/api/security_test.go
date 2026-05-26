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

package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

// The transport tests cover both direct TLS and trusted proxy headers because the
// security helpers are shared by embedded UI routes that may sit behind a proxy.
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

// The wrapper should fail fast before running the protected handler on plain HTTP.
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

// Replay-protected handlers should pass straight through once a matching nonce exists.
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

// Route options are later consumed by SecurityFilter, so the feature flag and labels
// must both be set when secure transport enforcement is requested declaratively.
func TestSecureTransportOption(t *testing.T) {
	options := &HandlerOptions{}
	SecureTransportOption(SecureTransportOptions{TrustForwardHeaders: true})(options)

	if !options.Feature(FeatureRequireSecureTransport) {
		t.Fatal("expected secure transport feature to be enabled")
	}
	if options.Labels == nil {
		t.Fatal("expected labels to be initialized")
	}
	if v, ok := options.Labels[LabelTrustForwardHeaders].(bool); !ok || !v {
		t.Fatalf("expected trust forward headers label to be true, got %#v", options.Labels[LabelTrustForwardHeaders])
	}
}

// Replay protection uses a single feature flag because the filter reads no extra labels.
func TestReplayProtectionOption(t *testing.T) {
	options := &HandlerOptions{}
	ReplayProtectionOption()(options)

	if !options.Feature(FeatureRequireReplayProtection) {
		t.Fatal("expected replay protection feature to be enabled")
	}
}

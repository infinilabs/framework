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
	"net/http"
	"strings"

	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

type SecureTransportOptions struct {
	// TrustForwardHeaders allows HTTPS detection to honor reverse-proxy forwarding headers.
	TrustForwardHeaders bool
}

const (
	// FeatureRequireSecureTransport marks a UI handler as HTTPS-only when it is enforced by filters.
	FeatureRequireSecureTransport = "feature_require_secure_transport"
	// FeatureRequireReplayProtection marks a UI handler as requiring a valid replay nonce.
	FeatureRequireReplayProtection = "feature_require_replay_protection"
	// LabelTrustForwardHeaders stores whether HTTPS checks may trust reverse-proxy forwarding headers.
	LabelTrustForwardHeaders = "label_trust_forward_headers"
)

// RequestUsesSecureTransport reports whether the request arrived over HTTPS directly or, when
// allowed, through a trusted reverse proxy that forwarded HTTPS metadata.
func RequestUsesSecureTransport(req *http.Request, options ...SecureTransportOptions) bool {
	if req == nil {
		return false
	}
	if req.TLS != nil {
		return true
	}

	resolved := resolveSecureTransportOptions(options)
	if !resolved.TrustForwardHeaders {
		return false
	}

	for _, header := range []string{"X-Forwarded-Proto", "X-Forwarded-Protocol", "X-Url-Scheme"} {
		if headerIndicatesHTTPS(req.Header.Get(header)) {
			return true
		}
	}

	if strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Ssl")), "on") {
		return true
	}

	return forwardedHeaderIndicatesHTTPS(req.Header.Get("Forwarded"))
}

// RequireSecureTransport wraps a handler so it rejects requests that do not resolve to HTTPS.
func (handler Handler) RequireSecureTransport(h httprouter.Handle, options ...SecureTransportOptions) httprouter.Handle {
	resolved := resolveSecureTransportOptions(options)
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if !RequestUsesSecureTransport(r, resolved) {
			handler.WriteError(w, "this endpoint requires HTTPS. use https:// directly or route through a trusted HTTPS reverse proxy", http.StatusUpgradeRequired)
			return
		}
		h(w, r, ps)
	}
}

// RequireSecureTransport wraps a handler with the default security handler implementation.
func RequireSecureTransport(h httprouter.Handle, options ...SecureTransportOptions) httprouter.Handle {
	return Handler{}.RequireSecureTransport(h, options...)
}

// RequireReplayProtection wraps a handler so each request must present a valid replay nonce.
func (handler Handler) RequireReplayProtection(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := replaysecurity.ValidateAndConsumeReplayNonce(r); err != nil {
			handler.WriteError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h(w, r, ps)
	}
}

// RequireReplayProtection wraps a handler with the default replay-protection implementation.
func RequireReplayProtection(h httprouter.Handle) httprouter.Handle {
	return Handler{}.RequireReplayProtection(h)
}

// SecureTransportOption annotates a UI route so SecurityFilter can enforce HTTPS consistently.
func SecureTransportOption(options ...SecureTransportOptions) Option {
	resolved := resolveSecureTransportOptions(options)
	return func(o *HandlerOptions) {
		Feature(FeatureRequireSecureTransport)(o)
		Label(LabelTrustForwardHeaders, resolved.TrustForwardHeaders)(o)
	}
}

// ReplayProtectionOption annotates a UI route so SecurityFilter enforces replay-nonce validation.
func ReplayProtectionOption() Option {
	return Feature(FeatureRequireReplayProtection)
}

func resolveSecureTransportOptions(options []SecureTransportOptions) SecureTransportOptions {
	if len(options) == 0 {
		return SecureTransportOptions{}
	}
	return options[0]
}

func headerIndicatesHTTPS(value string) bool {
	if value == "" {
		return false
	}
	first := strings.TrimSpace(strings.Split(value, ",")[0])
	return strings.EqualFold(first, "https")
}

func forwardedHeaderIndicatesHTTPS(value string) bool {
	if value == "" {
		return false
	}

	for _, forwardedValue := range strings.Split(value, ",") {
		for _, token := range strings.Split(forwardedValue, ";") {
			parts := strings.SplitN(strings.TrimSpace(token), "=", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "proto") {
				continue
			}
			proto := strings.Trim(parts[1], "\"")
			return strings.EqualFold(proto, "https")
		}
	}

	return false
}

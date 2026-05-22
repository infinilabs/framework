package api

import (
	"net/http"
	"strings"

	httprouter "infini.sh/framework/core/api/router"
	replaysecurity "infini.sh/framework/core/security/replay"
)

type SecureTransportOptions struct {
	TrustForwardHeaders bool
}

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

func (handler Handler) RequireSecureTransport(h httprouter.Handle, options ...SecureTransportOptions) httprouter.Handle {
	resolved := resolveSecureTransportOptions(options)
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if !RequestUsesSecureTransport(r, resolved) {
			handler.WriteError(w, "sensitive endpoints require HTTPS or a trusted HTTPS reverse proxy", http.StatusUpgradeRequired)
			return
		}
		h(w, r, ps)
	}
}

func RequireSecureTransport(h httprouter.Handle, options ...SecureTransportOptions) httprouter.Handle {
	return Handler{}.RequireSecureTransport(h, options...)
}

func (handler Handler) RequireReplayProtection(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := replaysecurity.ValidateAndConsumeReplayNonce(r); err != nil {
			handler.WriteError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h(w, r, ps)
	}
}

func RequireReplayProtection(h httprouter.Handle) httprouter.Handle {
	return Handler{}.RequireReplayProtection(h)
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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package config

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"infini.sh/framework/core/util"
)

// IPv6 runtime addresses must remain valid in both advertised endpoints.
func TestGetEffectiveAddrPreservesIPv6Brackets(t *testing.T) {
	for _, binding := range []string{"[::1]:39123", "[2001:db8::1]:39123"} {
		t.Run(binding, func(t *testing.T) {
			cfg := NetworkConfig{Binding: "127.0.0.1:29000"}
			cfg.SetEffectiveBinding(binding)
			if got := cfg.GetEffectiveAddr(); got != binding {
				t.Errorf("GetEffectiveAddr = %q, want %q", got, binding)
			}
			web := WebAppConfig{NetworkConfig: cfg}
			api := APIConfig{NetworkConfig: cfg}
			for _, endpoint := range []string{web.GetEndpoint(), api.GetEndpoint()} {
				u, err := url.Parse(endpoint)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := net.SplitHostPort(u.Host); err != nil {
					t.Errorf("invalid endpoint %q: %v", endpoint, err)
				}
			}
		})
	}
}

// An explicit publish endpoint is operator intent (NAT/public mappings) and
// must survive the listener opening — the runtime binding only carries the
// local view.
func TestGetEffectiveAddrPreservesExplicitPublish(t *testing.T) {
	cfg := NetworkConfig{Binding: "192.168.1.5:29000", Publish: "gw.example.com:29000"}
	cfg.SetEffectiveBinding("10.0.0.9:39123")
	if got := cfg.GetEffectiveAddr(); got != "gw.example.com:29000" {
		t.Fatalf("GetEffectiveAddr = %q, want the explicit publish address", got)
	}
}

// A wildcard runtime binding advertises an intranet IP with the runtime port
// instead of leaking 0.0.0.0/[::] or the configured (possibly stale) port.
func TestGetEffectiveAddrNormalizesWildcardRuntimeBinding(t *testing.T) {
	cfg := NetworkConfig{Binding: "0.0.0.0:29000"}
	cfg.SetEffectiveBinding("0.0.0.0:39555")
	got := cfg.GetEffectiveAddr()
	host, port, err := net.SplitHostPort(got)
	if err != nil {
		t.Fatalf("GetEffectiveAddr = %q: %v", got, err)
	}
	if port != "39555" {
		t.Fatalf("port = %q, want the runtime port 39555", port)
	}
	if strings.Contains(got, "[") {
		t.Fatalf("GetEffectiveAddr = %q: bracketed wildcard leaked into the endpoint", got)
	}
	if ip, ipErr := util.GetIntranetIP(); ipErr == nil && ip != "" && host != ip {
		t.Fatalf("host = %q, want the intranet IP %q", host, ip)
	}
}

// A runtime binding pinned to a specific interface keeps that host — only the
// port follows the runtime value.
func TestGetEffectiveAddrKeepsPinnedHost(t *testing.T) {
	cfg := NetworkConfig{Binding: "192.168.1.5:29000"}
	cfg.SetEffectiveBinding("192.168.1.5:39556")
	if got := cfg.GetEffectiveAddr(); got != "192.168.1.5:39556" {
		t.Fatalf("GetEffectiveAddr = %q, want 192.168.1.5:39556", got)
	}
}

// While nothing is listening, the static publish/binding semantics apply.
func TestGetEffectiveAddrFallsBackToPublishWhenNotListening(t *testing.T) {
	cfg := NetworkConfig{Host: "127.0.0.1", Port: 29000}
	if got := cfg.GetEffectiveAddr(); got != "127.0.0.1:29000" {
		t.Fatalf("GetEffectiveAddr = %q, want 127.0.0.1:29000", got)
	}
}

// net.Listener.Addr().String() reports bracketed IPv6 forms; the port parser
// must read through the brackets.
func TestGetEffectivePortParsesBracketedIPv6(t *testing.T) {
	cfg := NetworkConfig{}
	cfg.SetEffectiveBinding("[::]:39557")
	if got := cfg.GetEffectivePort(); got != 39557 {
		t.Fatalf("GetEffectivePort = %d, want 39557", got)
	}
}

// GetEffectiveBinding is the raw address for local dialers: the runtime value
// as bound, or the configured binding while nothing is listening.
func TestGetEffectiveBindingRawAndFallback(t *testing.T) {
	cfg := NetworkConfig{Host: "127.0.0.1", Port: 29000}
	if got := cfg.GetEffectiveBinding(); got != "127.0.0.1:29000" {
		t.Fatalf("GetEffectiveBinding fallback = %q, want 127.0.0.1:29000", got)
	}
	cfg.SetEffectiveBinding("[::]:39558")
	if got := cfg.GetEffectiveBinding(); got != "[::]:39558" {
		t.Fatalf("GetEffectiveBinding = %q, want the raw [::]:39558", got)
	}
}

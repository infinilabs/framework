/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"infini.sh/framework/core/config"
)

// proxyEnabledWithoutAddress is the opt-in shape for the dynamic takeover:
// the master switch is on but no static proxy content is configured.
func proxyEnabledWithoutAddress() *config.HTTPClientConfig {
	cfg := &config.HTTPClientConfig{}
	cfg.Proxy.Enabled = true
	return cfg
}

// The Enabled master switch is enforced at the dial sites, standalone and
// before resolveProxy: a disabled client must connect directly even while a
// resolver is registered that would reroute everything into a dead proxy.
func TestDisabledClientDialsDirectDespiteResolver(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, addr string) (bool, *config.ProxyConfig) {
		return true, &config.ProxyConfig{HTTPProxy: "http://127.0.0.1:1"} //nothing listens there
	})

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("direct-ok"))
	}))
	defer origin.Close()

	cfg := &config.HTTPClientConfig{Timeout: "5s", DialTimeout: "2s"} //Enabled stays false
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatalf("disabled client must dial direct, got: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from origin, got %v", resp.StatusCode)
	}
}

func TestResolveProxyStaticConfigWins(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	cfg := &config.HTTPClientConfig{}
	cfg.Proxy.Enabled = true
	cfg.Proxy.DefaultProxyConfig.HTTPProxy = "http://127.0.0.1:1"

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, addr string) (bool, *config.ProxyConfig) {
		return true, &config.ProxyConfig{HTTPProxy: "http://10.0.0.1:1"}
	})

	ok, pc := resolveProxy(cfg, "example.com:443")
	if !ok || pc == nil || pc.HTTPProxy != "http://127.0.0.1:1" {
		t.Fatalf("static config must win, got ok=%v cfg=%v", ok, pc)
	}
}

// Denied lists are static proxy content too: with them present the static
// rules alone decide and the resolver is not consulted.
func TestResolveProxyDeniedListBeatsResolver(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, addr string) (bool, *config.ProxyConfig) {
		return true, &config.ProxyConfig{HTTPProxy: "http://127.0.0.1:7890"}
	})

	cfg := &config.HTTPClientConfig{}
	cfg.Proxy.Enabled = true
	cfg.Proxy.Denied = []string{"internal.example"}

	ok, pc := resolveProxy(cfg, "internal.example:443")
	if ok || pc != nil {
		t.Fatalf("denied addr must stay direct, got ok=%v cfg=%v", ok, pc)
	}
}

func TestResolveProxyDynamicResolver(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, addr string) (bool, *config.ProxyConfig) {
		if addr == "192.168.1.10:9200" {
			return false, nil
		}
		return true, &config.ProxyConfig{HTTPProxy: "http://127.0.0.1:7890"}
	})

	cfg := proxyEnabledWithoutAddress()

	ok, pc := resolveProxy(cfg, "example.com:443")
	if !ok || pc == nil || pc.HTTPProxy != "http://127.0.0.1:7890" {
		t.Fatalf("expected takeover proxy, got ok=%v cfg=%v", ok, pc)
	}

	ok, pc = resolveProxy(cfg, "192.168.1.10:9200")
	if ok || pc != nil {
		t.Fatalf("expected bypass for internal addr, got ok=%v cfg=%v", ok, pc)
	}

	//unregister restores direct connections
	UnregisterProxyResolver()
	ok, pc = resolveProxy(cfg, "example.com:443")
	if ok || pc != nil {
		t.Fatalf("expected direct after unregister, got ok=%v cfg=%v", ok, pc)
	}
}

func TestRegisterProxyResolverIgnoresNil(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	RegisterProxyResolver(nil)
	if dynamicProxyResolver.Load() != nil {
		t.Fatal("nil resolver must be ignored")
	}
}

func TestStripProxyScheme(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:7890": "127.0.0.1:7890",
		"127.0.0.1:7890":        "127.0.0.1:7890",
		"https://p.example":     "p.example",
		"[::1]:7890":            "[::1]:7890",
	}
	for in, want := range cases {
		if got := stripProxyScheme(in); got != want {
			t.Fatalf("stripProxyScheme(%q)=%q, want %q", in, got, want)
		}
	}
}

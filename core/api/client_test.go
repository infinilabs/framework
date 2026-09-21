/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"testing"

	"infini.sh/framework/core/config"
)

func TestResolveProxyDirectWithoutResolver(t *testing.T) {
	UnregisterProxyResolver()
	cfg := &config.HTTPClientConfig{}

	ok, pc := resolveProxy(cfg, "example.com:443")
	if ok || pc != nil {
		t.Fatalf("expected direct connection, got ok=%v cfg=%v", ok, pc)
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

func TestResolveProxyDynamicResolver(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, addr string) (bool, *config.ProxyConfig) {
		if addr == "192.168.1.10:9200" {
			return false, nil
		}
		return true, &config.ProxyConfig{HTTPProxy: "http://127.0.0.1:7890"}
	})

	cfg := &config.HTTPClientConfig{}

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

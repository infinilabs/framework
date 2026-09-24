/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestProxyDialAddr(t *testing.T) {
	cases := []struct {
		in, addr, auth string
		wantErr        bool
	}{
		{in: "http://127.0.0.1:7890", addr: "127.0.0.1:7890"},
		{in: "127.0.0.1:7890", addr: "127.0.0.1:7890"},
		// userinfo must be decoded, not passed through in percent-encoded form
		{in: "http://us%40er:p%40ss@10.0.0.1:8080", addr: "10.0.0.1:8080", auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("us@er:p@ss"))},
		{in: "http://user@10.0.0.1:8080", addr: "10.0.0.1:8080", auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:"))},
		{in: "http://10.0.0.1", wantErr: true}, //missing port
	}
	for _, c := range cases {
		addr, auth, err := proxyDialAddr(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("proxyDialAddr(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("proxyDialAddr(%q): %v", c.in, err)
		}
		if addr != c.addr || auth != c.auth {
			t.Fatalf("proxyDialAddr(%q)=(%q,%q), want (%q,%q)", c.in, addr, auth, c.addr, c.auth)
		}
	}
}

// fakeConnectProxy is a minimal HTTP forward proxy: it answers one CONNECT
// (rejecting a wrong Proxy-Authorization with 407), then serves exactly one
// request traveling through the tunnel before closing.
func fakeConnectProxy(t *testing.T, wantAuth string) (addr string, gotAuth <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	authCh := make(chan string, 1)
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		br := bufio.NewReader(conn)
		req, err := http.ReadRequest(br)
		if err != nil || req.Method != http.MethodConnect {
			return
		}
		authCh <- req.Header.Get("Proxy-Authorization")
		if wantAuth != "" && req.Header.Get("Proxy-Authorization") != wantAuth {
			_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		if _, err := http.ReadRequest(br); err != nil { //the tunneled request
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 9\r\nConnection: close\r\n\r\ntunnel-ok")
	}()
	return ln.Addr().String(), authCh
}

func TestDialHTTPProxyTunnelAuthAndDeadline(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("us@er:p@ss"))
	addr, gotAuth := fakeConnectProxy(t, want)

	conn, err := dialHTTPProxyTunnel(context.Background(), &net.Dialer{Timeout: 2 * time.Second}, addr, want, "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if auth := <-gotAuth; auth != want {
		t.Fatalf("proxy received Proxy-Authorization %q, want %q", auth, want)
	}

	// The returned connection is an end-to-end tunnel: a request written to
	// it must reach the origin (the fake proxy's tunneled phase).
	if _, err := conn.Write([]byte("GET /hello HTTP/1.0\r\nHost: example.com\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "tunnel-ok" {
		t.Fatalf("tunneled request got status=%v body=%q", resp.StatusCode, body)
	}

	// A proxy that accepts TCP but never answers CONNECT must fail within
	// the dial timeout, not hang until the OS-level TCP timeout.
	stalled, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Close()
	go func() {
		if c, err := stalled.Accept(); err == nil {
			defer c.Close()
			time.Sleep(10 * time.Second) //never answers
		}
	}()

	start := time.Now()
	_, err = dialHTTPProxyTunnel(context.Background(), &net.Dialer{Timeout: 300 * time.Millisecond}, stalled.Addr().String(), "", "example.com:443")
	if err == nil {
		t.Fatal("expected an error from an unresponsive proxy")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("CONNECT round trip not bounded by the deadline, took %v", elapsed)
	}
}

// A full net/http client with the takeover-shaped config (enabled, no static
// proxy content) must tunnel through the resolver's HTTP proxy, with
// Proxy-Authorization built from the proxy URL userinfo.
func TestClientTunnelsThroughHTTPProxyWithAuth(t *testing.T) {
	UnregisterProxyResolver()
	defer UnregisterProxyResolver()

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("us@er:p@ss"))
	addr, gotAuth := fakeConnectProxy(t, want)

	RegisterProxyResolver(func(clientCfg *config.HTTPClientConfig, a string) (bool, *config.ProxyConfig) {
		return true, &config.ProxyConfig{HTTPProxy: "http://us%40er:p%40ss@" + addr}
	})

	cfg := proxyEnabledWithoutAddress()
	cfg.Timeout = "5s"
	cfg.DialTimeout = "2s"
	client, err := NewHTTPClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Get("http://" + addr + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "tunnel-ok" {
		t.Fatalf("client via tunnel got status=%v body=%q", resp.StatusCode, body)
	}
	if auth := <-gotAuth; auth != want {
		t.Fatalf("proxy received Proxy-Authorization %q, want %q (userinfo must be decoded)", auth, want)
	}
}

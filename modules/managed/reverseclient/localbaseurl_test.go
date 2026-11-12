/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package reverse

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

// localBaseURL must rewrite every wildcard form — including the bracketed
// [::] that net.Listener.Addr().String() reports — to 127.0.0.1, or the
// loopback URL can be sent through an HTTP proxy together with its API token.
func TestLocalBaseURLRewritesWildcardEffectiveBindings(t *testing.T) {
	cases := []struct {
		name string
		nc   config.NetworkConfig
		want string
	}{
		{"bracketed ipv6 wildcard", config.NetworkConfig{Port: 39901, EffectiveBinding: "[::]:39901"}, "http://127.0.0.1:39901"},
		{"ipv4 wildcard", config.NetworkConfig{Port: 39902, EffectiveBinding: "0.0.0.0:39902"}, "http://127.0.0.1:39902"},
		{"pinned interface ip", config.NetworkConfig{Port: 39903, EffectiveBinding: "192.168.1.5:39903"}, "http://192.168.1.5:39903"},
		{"fallback to configured binding", config.NetworkConfig{Host: "0.0.0.0", Port: 39904}, "http://127.0.0.1:39904"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prev := swapWebNetworkConfig(t, c.nc)
			defer prev()
			if got := localBaseURL(); got != c.want {
				t.Fatalf("localBaseURL = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLocalBaseURLWildcardUsesHTTPSWhenTLSEnabled(t *testing.T) {
	nc := config.NetworkConfig{Port: 39905, EffectiveBinding: "[::]:39905"}
	prev := swapWebNetworkConfig(t, nc)
	defer prev()
	e := global.Env()
	prevTLS := e.SystemConfig.WebAppConfig.TLSConfig.TLSEnabled
	e.SystemConfig.WebAppConfig.TLSConfig.TLSEnabled = true
	defer func() { e.SystemConfig.WebAppConfig.TLSConfig.TLSEnabled = prevTLS }()
	if got := localBaseURL(); got != "https://127.0.0.1:39905" {
		t.Fatalf("localBaseURL = %q, want https://127.0.0.1:39905", got)
	}
}

// swapWebNetworkConfig installs a web network config on the global env and
// returns a restore func.
func swapWebNetworkConfig(t *testing.T, nc config.NetworkConfig) func() {
	t.Helper()
	e := global.Env()
	if e == nil {
		e = env.EmptyEnv()
		global.RegisterEnv(e)
	}
	prev := e.SystemConfig.WebAppConfig.NetworkConfig
	e.SystemConfig.WebAppConfig.NetworkConfig = nc
	return func() { e.SystemConfig.WebAppConfig.NetworkConfig = prev }
}

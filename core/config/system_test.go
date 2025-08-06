package config

import (
	"sync"
	"testing"
)

// --- Test Setup ---

// This struct will hold all necessary configs for our tests.
type TestSetup struct {
	ClientConfig *HTTPClientConfig
	// We store the specific proxy config here to easily reference it later.
	GithubProxyConfig ProxyConfig
}

// setupTestConfig now returns the setup struct.
func setupTestConfig() TestSetup {
	githubProxy := ProxyConfig{
		HTTPProxy:    "http://127.0.0.1:7890",
		Socket5Proxy: "socks5://127.0.0.1:7890",
	}

	cfg := &HTTPClientConfig{}
	cfg.Proxy.Enabled = true
	cfg.Proxy.DefaultProxyConfig = ProxyConfig{
		HTTPProxy:    "http://127.0.0.1:7890",
		Socket5Proxy: "socks5://127.0.0.1:7890",
	}
	cfg.Proxy.Permitted = []string{"google.com"}
	cfg.Proxy.Denied = []string{
		"localhost", "localhost:9201", "127.0.0.1", "www.yuque.com", "yuque.com",
		"infinilabs.com", "api.coco.rs", "dashscope.aliyuncs.com",
	}
	cfg.Proxy.Domains = map[string]ProxyConfig{
		"github.com": githubProxy,
	}

	return TestSetup{
		ClientConfig:      cfg,
		GithubProxyConfig: githubProxy,
	}
}

func resetOnce() {
	localIPsOnce = sync.Once{}
}

// --- Main Test Function ---

func TestHTTPClientConfig_ValidateProxy(t *testing.T) {
	t.Run("Whitelist Mode", func(t *testing.T) {
		resetOnce()
		setup := setupTestConfig()
		cfg := setup.ClientConfig

		// We get the pointer from the pre-defined struct field, not the map index.
		githubProxyPtr := &setup.GithubProxyConfig

		tests := []struct {
			name       string
			addr       string
			wantProxy  bool
			wantConfig *ProxyConfig
		}{
			{
				name:       "Should use specific proxy for github.com",
				addr:       "github.com",
				wantProxy:  true,
				wantConfig: githubProxyPtr, // Use the valid pointer here
			},
			{
				name:       "Should NOT proxy localhost (denied list)",
				addr:       "localhost",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should NOT proxy localhost with port",
				addr:       "localhost:9200",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should NOT proxy 127.0.0.1 with port",
				addr:       "127.0.0.1:8080",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should NOT proxy machine's own IP (mocked)",
				addr:       "192.168.1.10:9200",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should NOT proxy infinilabs.com (denied list)",
				addr:       "infinilabs.com",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should NOT proxy subdomain of denied domain",
				addr:       "docs.infinilabs.com",
				wantProxy:  false,
				wantConfig: nil,
			},
			{
				name:       "Should proxy google.com (permitted list)",
				addr:       "google.com",
				wantProxy:  true,
				wantConfig: &cfg.Proxy.DefaultProxyConfig,
			},
			{
				name:       "Should proxy subdomain of permitted domain",
				addr:       "mail.google.com",
				wantProxy:  true,
				wantConfig: &cfg.Proxy.DefaultProxyConfig,
			},
			{
				name:       "Should NOT proxy domain NOT in permitted list",
				addr:       "example.com",
				wantProxy:  false,
				wantConfig: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gotProxy, gotConfig := cfg.ValidateProxy(tt.addr)
				if gotProxy != tt.wantProxy {
					t.Errorf("ValidateProxy() gotProxy = %v, want %v", gotProxy, tt.wantProxy)
				}
				if (gotConfig == nil) != (tt.wantConfig == nil) {
					t.Errorf("ValidateProxy() config nil mismatch: got %v, want %v", gotConfig, tt.wantConfig)
				}
				if gotConfig != nil && tt.wantConfig != nil && *gotConfig != *tt.wantConfig {
					t.Errorf("ValidateProxy() gotConfig = %+v, want %+v", *gotConfig, *tt.wantConfig)
				}
			})
		}
	})

	t.Run("Blacklist Mode", func(t *testing.T) {
		resetOnce()
		setup := setupTestConfig()
		cfg := setup.ClientConfig
		cfg.Proxy.Permitted = []string{}

		tests := []struct {
			name      string
			addr      string
			wantProxy bool
		}{
			{
				name:      "Should NOT proxy localhost (denied)",
				addr:      "localhost:9200",
				wantProxy: false,
			},
			{
				name:      "Should NOT proxy yuque.com (denied)",
				addr:      "www.yuque.com",
				wantProxy: false,
			},
			{
				name:      "Should proxy google.com (not denied)",
				addr:      "google.com",
				wantProxy: true,
			},
			{
				name:      "Should proxy any other domain (not denied)",
				addr:      "another-random-site.com",
				wantProxy: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gotProxy, _ := cfg.ValidateProxy(tt.addr)
				if gotProxy != tt.wantProxy {
					t.Errorf("ValidateProxy() gotProxy = %v, want %v", gotProxy, tt.wantProxy)
				}
			})
		}
	})

	t.Run("Proxy Disabled", func(t *testing.T) {
		resetOnce()
		setup := setupTestConfig()
		cfg := setup.ClientConfig
		cfg.Proxy.Enabled = false

		gotProxy, _ := cfg.ValidateProxy("google.com")
		if gotProxy != false {
			t.Errorf("ValidateProxy() with proxy disabled; gotProxy = %v, want false", gotProxy)
		}
	})
}

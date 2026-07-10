package env

import (
	"testing"

	"infini.sh/framework/core/config"
)

func TestGetHTTPClientConfigFallsBackToConfigsTLS(t *testing.T) {
	env := &Env{
		SystemConfig: &config.SystemConfig{
			Configs: config.ConfigsConfig{
				TLSConfig: config.TLSConfig{
					TLSEnabled:             true,
					TLSCertFile:            "config/client.crt",
					TLSKeyFile:             "config/client.key",
					TLSCACertFile:          "config/ca.crt",
					TLSInsecureSkipVerify:  false,
					SkipDomainVerify:       true,
					ClientSessionCacheSize: 64,
				},
			},
		},
	}

	cfg := env.GetHTTPClientConfig("configs", "")
	if cfg.TLSConfig.TLSCertFile != "config/client.crt" {
		t.Fatalf("expected configs tls cert_file fallback, got %q", cfg.TLSConfig.TLSCertFile)
	}
	if cfg.TLSConfig.TLSKeyFile != "config/client.key" {
		t.Fatalf("expected configs tls key_file fallback, got %q", cfg.TLSConfig.TLSKeyFile)
	}
	if cfg.TLSConfig.TLSCACertFile != "config/ca.crt" {
		t.Fatalf("expected configs tls ca_file fallback, got %q", cfg.TLSConfig.TLSCACertFile)
	}
	if !cfg.TLSConfig.SkipDomainVerify {
		t.Fatal("expected configs tls skip_domain_verify fallback")
	}
}

func TestGetHTTPClientConfigKeepsExplicitConfigsClientTLS(t *testing.T) {
	env := &Env{
		SystemConfig: &config.SystemConfig{
			Configs: config.ConfigsConfig{
				TLSConfig: config.TLSConfig{
					TLSCertFile: "config/client.crt",
				},
			},
			HTTPClientConfig: map[string]config.HTTPClientConfig{
				"configs": {
					TLSConfig: config.TLSConfig{
						TLSCertFile: "override/client.crt",
					},
				},
			},
		},
	}

	cfg := env.GetHTTPClientConfig("configs", "")
	if cfg.TLSConfig.TLSCertFile != "override/client.crt" {
		t.Fatalf("expected explicit configs client tls to win, got %q", cfg.TLSConfig.TLSCertFile)
	}
}

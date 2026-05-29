package api

import (
	"strings"
	"testing"

	"infini.sh/framework/core/config"
)

func TestValidateServerExposureConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.SystemConfig
		wantErr string
	}{
		{
			name: "safe api config",
			cfg: config.SystemConfig{
				APIConfig: config.APIConfig{
					Enabled: true,
					Security: config.APISecurityConfig{
						Enabled:  true,
						Username: "api-user",
					},
				},
			},
		},
		{
			name: "reject insecure api",
			cfg: config.SystemConfig{
				APIConfig: config.APIConfig{
					Enabled: true,
				},
			},
			wantErr: "api.enabled requires api.security.enabled",
		},
		{
			name: "reject embedded api on web",
			cfg: config.SystemConfig{
				WebAppConfig: config.WebAppConfig{
					Enabled:      true,
					EmbeddingAPI: true,
				},
			},
			wantErr: "web.embedding_api is forbidden",
		},
		{
			name: "reject missing api username",
			cfg: config.SystemConfig{
				APIConfig: config.APIConfig{
					Enabled: true,
					Security: config.APISecurityConfig{
						Enabled: true,
					},
				},
			},
			wantErr: "api.security.username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerExposureConfig(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

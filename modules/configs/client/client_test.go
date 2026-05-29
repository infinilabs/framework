package client

import (
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	ucfg "infini.sh/framework/lib/go-ucfg"
)

func TestApplyManagerRequestAuthUsesAccessTokenHeader(t *testing.T) {
	oldConfigs := global.Env().SystemConfig.Configs
	t.Cleanup(func() {
		global.Env().SystemConfig.Configs = oldConfigs
	})

	global.Env().SystemConfig.Configs = config.ConfigsConfig{
		ManagerConfig: struct {
			LocalConfigsRepoPath string            `config:"local_configs_repo_path"`
			BasicAuth            config.BasicAuth  `config:"basic_auth"`
			AccessToken          ucfg.SecretString `config:"access_token"`
		}{
			AccessToken: ucfg.SecretString("manager-api-token"),
			BasicAuth: config.BasicAuth{
				Username: "manager",
				Password: ucfg.SecretString("secret"),
			},
		},
	}

	req := &util.Request{}
	if err := applyManagerRequestAuth(req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	headers := req.AllHeaders()
	if headers[model.API_TOKEN] != "manager-api-token" {
		t.Fatalf("expected %s header to be set, got %#v", model.API_TOKEN, headers)
	}
	if auth := headers["Authorization"]; auth != "" {
		t.Fatalf("expected no Authorization header, got %q", auth)
	}
}

func TestBuildManagedRegisterAccessToken(t *testing.T) {
	t.Setenv("KEYSTORE_PATH", t.TempDir())

	instance := model.Instance{}
	instance.ID = "gateway-1"
	instance.Application = env.Application{Name: "gateway"}

	registerToken, err := buildManagedRegisterAccessToken(instance)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if registerToken == nil || registerToken.Value == "" {
		t.Fatalf("expected managed register token, got %#v", registerToken)
	}
	if !strings.Contains(registerToken.Description, "gateway") {
		t.Fatalf("unexpected description: %q", registerToken.Description)
	}

	other := model.Instance{}
	other.ID = "other-1"
	other.Application = env.Application{Name: "console"}
	registerToken, err = buildManagedRegisterAccessToken(other)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if registerToken != nil {
		t.Fatalf("expected no managed register token, got %#v", registerToken)
	}
}

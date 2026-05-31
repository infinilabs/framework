package client

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	instance.Application = env.Application{
		Name:    "gateway",
		Version: env.Version{VersionNumber: "1.30.5"},
	}

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

	legacy := model.Instance{}
	legacy.ID = "legacy-agent-1"
	legacy.Application = env.Application{
		Name:    "agent",
		Version: env.Version{VersionNumber: "1.30.4"},
	}
	registerToken, err = buildManagedRegisterAccessToken(legacy)
	if err != nil {
		t.Fatalf("expected nil error for legacy agent, got %v", err)
	}
	if registerToken != nil {
		t.Fatalf("expected legacy agent to skip managed register token, got %#v", registerToken)
	}
}

func TestListenConfigChangesStillSyncsAfterHTTPClientInit(t *testing.T) {
	var syncRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs/_sync" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected request method: %s", r.Method)
		}
		syncRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changed":false}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	mainConfigFile := filepath.Join(tempDir, "agent.yml")
	if err := os.WriteFile(mainConfigFile, []byte("node:\n  id: test-agent\n"), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	oldEnv := global.Env()
	oldHTTPClientInitLock := managerHTTPClientInitLock
	oldConfigSyncInitLock := configSyncInitLock
	oldClient := mTLSClient
	t.Cleanup(func() {
		global.RegisterEnv(oldEnv)
		managerHTTPClientInitLock = oldHTTPClientInitLock
		configSyncInitLock = oldConfigSyncInitLock
		mTLSClient = oldClient
	})

	testEnv := env.EmptyEnv()
	testEnv.SystemConfig.Configs.Managed = true
	testEnv.SystemConfig.Configs.Servers = []string{server.URL}
	testEnv.SystemConfig.Configs.Interval = "30s"
	testEnv.SystemConfig.PathConfig.Config = configDir
	testEnv.SystemConfig.NodeConfig.ID = "test-agent"
	testEnv.SetConfigFile(mainConfigFile)
	global.RegisterEnv(testEnv)

	managerHTTPClientInitLock = sync.Once{}
	configSyncInitLock = sync.Once{}
	mTLSClient = nil

	if getManagerHTTPClient() == nil {
		t.Fatal("expected manager HTTP client to initialize")
	}
	if err := ListenConfigChanges(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if syncRequests.Load() != 1 {
		t.Fatalf("expected one immediate sync request, got %d", syncRequests.Load())
	}
}

func TestHandleUnauthorizedConfigSyncResponseClearsStateAndReconnects(t *testing.T) {
	oldClear := clearManagedRegistrationStateFunc
	oldReconnect := reconnectToManagerFunc
	oldRetryAt := lastUnauthorizedRegisterRetryAt
	t.Cleanup(func() {
		clearManagedRegistrationStateFunc = oldClear
		reconnectToManagerFunc = oldReconnect
		lastUnauthorizedRegisterRetryAt = oldRetryAt
	})

	var cleared atomic.Int32
	var reconnected atomic.Int32
	clearManagedRegistrationStateFunc = func() error {
		cleared.Add(1)
		return nil
	}
	reconnectToManagerFunc = func() error {
		reconnected.Add(1)
		return nil
	}
	lastUnauthorizedRegisterRetryAt = time.Time{}

	handled := handleUnauthorizedConfigSyncResponse(&util.Result{StatusCode: http.StatusUnauthorized})
	if !handled {
		t.Fatal("expected unauthorized config sync response to be handled")
	}
	if cleared.Load() != 1 {
		t.Fatalf("expected local registration state to be cleared once, got %d", cleared.Load())
	}
	if reconnected.Load() != 1 {
		t.Fatalf("expected reconnect to run once, got %d", reconnected.Load())
	}

	handled = handleUnauthorizedConfigSyncResponse(&util.Result{StatusCode: http.StatusUnauthorized})
	if !handled {
		t.Fatal("expected throttled unauthorized config sync response to still be handled")
	}
	if cleared.Load() != 1 {
		t.Fatalf("expected throttled retry not to clear state again, got %d", cleared.Load())
	}
	if reconnected.Load() != 1 {
		t.Fatalf("expected throttled retry not to reconnect again, got %d", reconnected.Load())
	}
}

func TestManagedConfigSyncGuardPreventsOverlap(t *testing.T) {
	configSyncInProgress.Store(false)
	t.Cleanup(func() {
		configSyncInProgress.Store(false)
	})

	if !tryStartManagedConfigSync() {
		t.Fatal("expected first config sync to start")
	}
	if tryStartManagedConfigSync() {
		t.Fatal("expected overlapping config sync to be rejected")
	}

	finishManagedConfigSync()

	if !tryStartManagedConfigSync() {
		t.Fatal("expected config sync to start again after previous one finished")
	}
	finishManagedConfigSync()
}

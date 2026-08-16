/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	ucfg "infini.sh/framework/lib/go-ucfg"
)

// ──────────────────────────────────────────────────────────────────────────
// Deployed-agent wire compatibility (pr/agent-managed-token-flow-20260524 +
// for_console_agent builds, console_framework-era client, deployed 2026-05/06).
//
// These tests pin the EXACT wire behavior of already-deployed agents so the
// servers built on this branch keep accepting them WITHOUT an agent upgrade:
//
//   auth headers (applyManagerRequestAuth precedence):
//     X-API-Token: <configs.manager.access_token>        (exchanged token)
//     Authorization: Bearer <keystore manager token>     (fallback)
//     Basic <configs.manager.basic_auth>                 (legacy)
//
//   POST /instance/_register        plain model.Instance JSON
//   POST /instance/_exchange_token  {"instance_id","agent_api_token"}
//   POST /configs/_sync             ConfigSyncRequest
//
// If a change here is intentional (protocol deprecation), update the deployed
// fleet first — do not break these assertions silently.
// ──────────────────────────────────────────────────────────────────────────

type capturedRequest struct {
	Path        string
	Headers     http.Header
	Body        string
	ContentType string
	Request     *http.Request
}

func newCapturingServer(t *testing.T) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	captured := []capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body := make([]byte, req.ContentLength)
		if req.ContentLength > 0 {
			_, _ = req.Body.Read(body)
		}
		mu.Lock()
		captured = append(captured, capturedRequest{
			Path:        req.URL.Path,
			Headers:     req.Header.Clone(),
			Body:        string(body),
			ContentType: req.Header.Get("Content-Type"),
			Request:     req,
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// setTestConfigs swaps the global configs for the test scope.
func setTestConfigs(t *testing.T, mutate func(cfg *config.ConfigsConfig)) {
	t.Helper()
	old := global.Env().SystemConfig.Configs
	t.Cleanup(func() { global.Env().SystemConfig.Configs = old })
	cfg := config.ConfigsConfig{}
	mutate(&cfg)
	global.Env().SystemConfig.Configs = cfg
}

func TestDeployedAgentAuth_XAPITokenPreferred(t *testing.T) {
	srv, captured := newCapturingServer(t)
	setTestConfigs(t, func(cfg *config.ConfigsConfig) {
		cfg.Servers = []string{srv.URL}
		cfg.ManagerConfig.AccessToken = ucfg.SecretString("exchanged-manager-token")
		cfg.ManagerConfig.BasicAuth.Username = "legacy"
		cfg.ManagerConfig.BasicAuth.Password = ucfg.SecretString("legacy-pass")
	})

	req := &util.Request{Method: util.Verb_POST, Path: "/configs/_sync", ContentType: "application/json", Body: []byte(`{}`)}
	if _, _, err := DoManagerRequest(req); err != nil {
		t.Fatalf("DoManagerRequest: %v", err)
	}

	got := (*captured)[0]
	if v := got.Headers.Get(model.API_TOKEN); v != "exchanged-manager-token" {
		t.Fatalf("deployed agents send the exchanged token via %s header; got %q", model.API_TOKEN, v)
	}
	// X-API-Token precedence: must NOT also fall back to BasicAuth
	if got.Headers.Get("Authorization") != "" {
		t.Fatalf("X-API-Token must suppress other auth; got Authorization=%q", got.Headers.Get("Authorization"))
	}
}

func TestDeployedAgentAuth_LegacyBasicAuth(t *testing.T) {
	srv, captured := newCapturingServer(t)
	setTestConfigs(t, func(cfg *config.ConfigsConfig) {
		cfg.Servers = []string{srv.URL}
		cfg.ManagerConfig.BasicAuth.Username = "legacy"
		cfg.ManagerConfig.BasicAuth.Password = ucfg.SecretString("legacy-pass")
	})

	req := &util.Request{Method: util.Verb_POST, Path: "/configs/_sync", ContentType: "application/json", Body: []byte(`{}`)}
	if _, _, err := DoManagerRequest(req); err != nil {
		t.Fatalf("DoManagerRequest: %v", err)
	}

	got := (*captured)[0]
	auth := got.Headers.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		t.Fatalf("legacy agents fall back to BasicAuth; got Authorization=%q", auth)
	}
	user, pass, _ := got.Request.BasicAuth()

	if user != "legacy" || pass != "legacy-pass" {
		t.Fatalf("basic credentials = %v/%v", user, pass)
	}
}

func TestDeployedAgentExchangeTokenWireFormat(t *testing.T) {
	srv, captured := newCapturingServer(t)
	setTestConfigs(t, func(cfg *config.ConfigsConfig) {
		cfg.Servers = []string{srv.URL}
		cfg.ManagerConfig.AccessToken = ucfg.SecretString("current-token")
	})

	// The deployed agent's exchange request body (internal/managed/exchange.go):
	// {"instance_id":"...","agent_api_token":"..."}
	body := util.MustToJSONBytes(map[string]string{
		"instance_id":    "agent-1",
		"agent_api_token": "agent-own-api-token",
	})
	req := &util.Request{Method: util.Verb_POST, Path: "/instance/_exchange_token", ContentType: "application/json", Body: body}
	if _, _, err := DoManagerRequest(req); err != nil {
		t.Fatalf("DoManagerRequest: %v", err)
	}

	got := (*captured)[0]
	if got.Path != "/instance/_exchange_token" {
		t.Fatalf("path = %s", got.Path)
	}
	if !strings.Contains(got.Body, `"instance_id":"agent-1"`) || !strings.Contains(got.Body, `"agent_api_token"`) {
		t.Fatalf("exchange body shape changed: %s", got.Body)
	}
	if v := got.Headers.Get(model.API_TOKEN); v != "current-token" {
		t.Fatalf("exchange must carry current manager token; got %q", v)
	}
}

func TestDeployedAgentRegisterPayloadShape(t *testing.T) {
	srv, captured := newCapturingServer(t)
	setTestConfigs(t, func(cfg *config.ConfigsConfig) {
		cfg.Servers = []string{srv.URL}
		cfg.ManagerConfig.AccessToken = ucfg.SecretString("bootstrap")
	})

	// Deployed agents POST a plain model.Instance (legacy shape, NOT wrapped
	// in {"client": ...}) with the self-generated API token in AccessToken.
	inst := model.GetInstanceInfo()
	inst.ID = "deployed-agent-1" // deployed agents always carry their node id
	inst.AccessToken = &model.Token{Value: "agent-self-generated"}

	req := &util.Request{Method: util.Verb_POST, Path: "/instance/_register", ContentType: "application/json", Body: util.MustToJSONBytes(inst)}
	if _, _, err := DoManagerRequest(req); err != nil {
		t.Fatalf("DoManagerRequest: %v", err)
	}

	got := (*captured)[0]
	if !strings.Contains(got.Body, `"id":`) || !strings.Contains(got.Body, `"access_token"`) {
		t.Fatalf("register payload must stay plain-Instance with access_token; got: %.200s", got.Body)
	}
	if strings.Contains(got.Body, `"client":`) {
		t.Fatalf("deployed agents send the LEGACY unwrapped register payload; servers must keep accepting it")
	}
}

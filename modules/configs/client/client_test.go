package client

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/modules/configs/common"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

var clientTestKeystoreDir string
var clientTestKeystoreOnce sync.Once

func useClientTestKeystore(t *testing.T) {
	t.Helper()

	clientTestKeystoreOnce.Do(func() {
		dir, err := os.MkdirTemp("", "framework-client-keystore-*")
		if err != nil {
			t.Fatalf("create client test keystore dir: %v", err)
		}
		clientTestKeystoreDir = dir
	})

	t.Setenv(keystore.PathEnvKey, clientTestKeystoreDir)
}

func TestBuildRegisterRequestAddsAgentAccessToken(t *testing.T) {
	useClientTestKeystore(t)

	info := model.Instance{}
	info.ID = "agent-1"
	info.Application = env.Application{Name: "agent"}

	req, err := buildRegisterRequest(info)
	if err != nil {
		t.Fatalf("build register request: %v", err)
	}
	if req.AccessToken == nil {
		t.Fatal("expected agent access token in register request")
	}
	if req.AccessToken.Name != "agent-1 reverse access token" {
		t.Fatalf("unexpected access token name: %q", req.AccessToken.Name)
	}
	if req.AccessToken.Value == "" {
		t.Fatal("expected generated access token value")
	}

	req2, err := buildRegisterRequest(info)
	if err != nil {
		t.Fatalf("build register request second call: %v", err)
	}
	if req2.AccessToken == nil || req2.AccessToken.Value != req.AccessToken.Value {
		t.Fatal("expected agent access token to be reused")
	}
}

func TestBuildRegisterRequestSkipsAccessTokenForNonAgent(t *testing.T) {
	info := model.Instance{}
	info.ID = "gateway-1"
	info.Application = env.Application{Name: "gateway"}

	req, err := buildRegisterRequest(info)
	if err != nil {
		t.Fatalf("build register request: %v", err)
	}
	if req.AccessToken != nil {
		t.Fatalf("expected no access token for non-agent instance, got %#v", req.AccessToken)
	}
}

func TestSubmitRequestToManagerPrefersBearerToken(t *testing.T) {
	useClientTestKeystore(t)

	envRef := global.Env()
	originalConfig := envRef.SystemConfig
	originalClient := mTLSClient
	t.Cleanup(func() {
		envRef.SystemConfig = originalConfig
		mTLSClient = originalClient
	})

	systemConfig := &config.SystemConfig{}
	systemConfig.Configs.Servers = []string{"http://manager.example:8080"}
	systemConfig.Configs.ManagerConfig.BasicAuth.Username = "admin"
	systemConfig.Configs.ManagerConfig.BasicAuth.Password = ucfg.SecretString("secret")
	envRef.SystemConfig = systemConfig

	if err := common.SaveTokenToKeystore(common.ManagerTokenKeystoreKey, "bearer-token"); err != nil {
		t.Fatalf("save manager token: %v", err)
	}

	var authHeader string
	var requestBody []byte
	mTLSClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			authHeader = req.Header.Get("Authorization")
			requestBody, _ = io.ReadAll(req.Body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	req := &util.Request{
		Method: util.Verb_POST,
		Path:   common.REGISTER_API,
		Body:   []byte(`{"hello":"world"}`),
	}
	server, res, err := submitRequestToManager(req)
	if err != nil {
		t.Fatalf("submit request to manager: %v", err)
	}
	if server != "http://manager.example:8080" {
		t.Fatalf("unexpected server: %s", server)
	}
	if res == nil || res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: %#v", res)
	}
	if authHeader != "Bearer bearer-token" {
		t.Fatalf("unexpected authorization header: %q", authHeader)
	}
	if string(requestBody) != `{"hello":"world"}` {
		t.Fatalf("unexpected request body: %s", string(requestBody))
	}
}

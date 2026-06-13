package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

func TestAppSettingsAPIHandlerIncludesSetupRequired(t *testing.T) {
	oldEnv := global.Env()
	testEnv := env.EmptyEnv()
	testEnv.SystemConfig.PathConfig.Data = t.TempDir()
	testEnv.EnableSetup(true)
	global.RegisterEnv(testEnv)
	defer global.RegisterEnv(oldEnv)

	req := httptest.NewRequest(http.MethodGet, "/setting/application", nil)
	resp := httptest.NewRecorder()

	appSettingsAPIHandler(resp, req, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if v, ok := body["setup_required"].(bool); !ok || !v {
		t.Fatalf("expected setup_required=true, got %#v", body["setup_required"])
	}
}

package api

import (
	"net/http"
	"testing"

	"infini.sh/framework/core/config"
)

func TestWebsocketRegistrationPath(t *testing.T) {
	cfg := config.WebAppConfig{}
	cfg.WebsocketConfig.Enabled = true
	cfg.WebsocketConfig.BasePath = "/custom-ws"

	if got := getWebsocketRegistrationPath(cfg); got != "/custom-ws" {
		t.Fatalf("unexpected websocket path: %s", got)
	}

	cfg.WebsocketConfig.BasePath = ""
	if got := getWebsocketRegistrationPath(cfg); got != "/ws" {
		t.Fatalf("unexpected default websocket path: %s", got)
	}
}

func TestShouldRegisterWebsocketOnWeb(t *testing.T) {
	originalHandlers := registeredAPIFuncHandler
	t.Cleanup(func() {
		registeredAPIFuncHandler = originalHandlers
	})

	cfg := config.WebAppConfig{}
	cfg.WebsocketConfig.Enabled = true
	cfg.WebsocketConfig.BasePath = "/ws"
	cfg.EmbeddingAPI = true

	registeredAPIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){
		"/ws": func(http.ResponseWriter, *http.Request) {},
	}

	if shouldRegisterWebsocketOnWeb(cfg) {
		t.Fatal("expected embedded API websocket registration to suppress duplicate web registration")
	}

	delete(registeredAPIFuncHandler, "/ws")
	if !shouldRegisterWebsocketOnWeb(cfg) {
		t.Fatal("expected websocket registration when no embedded API websocket handler exists")
	}

	cfg.EmbeddingAPI = false
	if !shouldRegisterWebsocketOnWeb(cfg) {
		t.Fatal("expected websocket registration when embedding_api is disabled")
	}
}

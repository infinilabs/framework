package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
)

func newTestBinding(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on random port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

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

func TestShouldSkipEmbeddedAPIRoute(t *testing.T) {
	originalUIHandlers := registeredUIHandler
	originalUIMethodHandlers := registeredUIMethodHandler
	t.Cleanup(func() {
		registeredUIHandler = originalUIHandlers
		registeredUIMethodHandler = originalUIMethodHandlers
	})

	registeredUIHandler = map[string]http.Handler{
		"/": http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	}
	registeredUIMethodHandler = map[Method]map[string]RegisteredAPIHandler{
		GET: {
			"/stats": {
				Handler: func(http.ResponseWriter, *http.Request, httprouter.Params) {},
			},
		},
	}

	if !shouldSkipEmbeddedAPIRoute("", "/") {
		t.Fatal("expected API root route to be skipped when UI root is registered")
	}
	if !shouldSkipEmbeddedAPIRoute(string(GET), "/stats") {
		t.Fatal("expected method-based UI route to suppress embedded API registration")
	}
	if !shouldSkipEmbeddedAPIRoute("", "/stats") {
		t.Fatal("expected UI method route to suppress embedded API func registration on same path")
	}
	if shouldSkipEmbeddedAPIRoute(string(GET), "/_info") {
		t.Fatal("expected unrelated API route not to be skipped")
	}
}

func TestRegisterMissingAPIMethodUIRoutesSkipsExistingUIRoutes(t *testing.T) {
	originalAPIHandlers := registeredAPIMethodHandler
	originalUIHandlers := registeredUIMethodHandler
	originalServer := srv
	originalRouter := uiRouter
	originalServeMux := uiServeMux
	t.Cleanup(func() {
		registeredAPIMethodHandler = originalAPIHandlers
		registeredUIMethodHandler = originalUIHandlers
		srv = originalServer
		uiRouter = originalRouter
		uiServeMux = originalServeMux
	})

	registeredAPIMethodHandler = map[string]map[string]func(http.ResponseWriter, *http.Request, httprouter.Params){
		http.MethodGet: {
			"/api-only": func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
				w.WriteHeader(http.StatusAccepted)
			},
			"/stats": func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
				w.WriteHeader(http.StatusNoContent)
			},
		},
	}
	registeredUIMethodHandler = map[Method]map[string]RegisteredAPIHandler{
		GET: {
			"/stats": {
				Handler: func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
					w.WriteHeader(http.StatusCreated)
				},
				Options: &HandlerOptions{},
			},
		},
	}

	RegisterMissingAPIMethodUIRoutes(func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
	})

	webCfg := config.WebAppConfig{}
	webCfg.NetworkConfig.Binding = newTestBinding(t)
	StartWeb(webCfg)
	defer StopWeb(webCfg)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api-only", nil)
	if err := ServeRegisteredUIRequest(resp, req); err != nil {
		t.Fatalf("serve api-only ui route: %v", err)
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("expected missing API route to be mirrored onto web, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/stats", nil)
	if err := ServeRegisteredUIRequest(resp, req); err != nil {
		t.Fatalf("serve stats ui route: %v", err)
	}
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected existing UI route to win over mirrored API route, got %d", resp.Code)
	}
}

func TestHandleUIMethodRegistersRouteAfterStartWeb(t *testing.T) {
	originalUIHandlers := registeredUIMethodHandler
	originalServer := srv
	originalRouter := uiRouter
	originalServeMux := uiServeMux
	t.Cleanup(func() {
		registeredUIMethodHandler = originalUIHandlers
		srv = originalServer
		uiRouter = originalRouter
		uiServeMux = originalServeMux
	})

	registeredUIMethodHandler = map[Method]map[string]RegisteredAPIHandler{}

	webCfg := config.WebAppConfig{}
	webCfg.NetworkConfig.Binding = newTestBinding(t)
	StartWeb(webCfg)
	defer StopWeb(webCfg)

	HandleUIMethod(GET, "/late-ui-route", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusAccepted)
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/late-ui-route", nil)
	if err := ServeRegisteredUIRequest(resp, req); err != nil {
		t.Fatalf("serve late ui route: %v", err)
	}
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected late ui route to be available after web start, got %d", resp.Code)
	}
}

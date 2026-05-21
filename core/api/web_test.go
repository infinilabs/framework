package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
)

func TestServeRegisteredUIRequestRequiresRouter(t *testing.T) {
	previous := uiRouter
	uiRouter = nil
	t.Cleanup(func() {
		uiRouter = previous
	})

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	resp := httptest.NewRecorder()

	err := ServeRegisteredUIRequest(resp, req)
	if err == nil {
		t.Fatal("expected error when ui router is not initialized")
	}
	if !strings.Contains(err.Error(), "web router is not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeRegisteredUIRequestDispatchesToRouter(t *testing.T) {
	previous := uiRouter
	router := httprouter.New(http.NewServeMux())
	router.Handle(http.MethodGet, "/ui", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		_, _ = w.Write([]byte("ok"))
	})
	uiRouter = router
	t.Cleanup(func() {
		uiRouter = previous
	})

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	resp := httptest.NewRecorder()

	err := ServeRegisteredUIRequest(resp, req)
	if err != nil {
		t.Fatalf("unexpected error serving ui request: %v", err)
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	if body := resp.Body.String(); body != "ok" {
		t.Fatalf("unexpected response body: %q", body)
	}
}

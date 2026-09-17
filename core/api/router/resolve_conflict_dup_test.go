package httprouter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func h1(w http.ResponseWriter, req *http.Request, p Params) {}
func h2(w http.ResponseWriter, req *http.Request, p Params) {}

func TestDuplicateParamPathWithResolveConflict(t *testing.T) {
	r := New(http.NewServeMux())
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panic on duplicate param path: %v", rec)
		}
	}()
	r.Handle("GET", "/queue/:id/_scroll", h1)
	r.Handle("GET", "/queue/:id/_scroll", h2)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/queue/abc/_scroll", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDuplicateStaticPathWithResolveConflict(t *testing.T) {
	r := New(http.NewServeMux())
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panic on duplicate static path: %v", rec)
		}
	}()
	r.Handle("GET", "/stats", h1)
	r.Handle("GET", "/stats", h1)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/stats", nil))
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

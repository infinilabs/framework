/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */
package http_filters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
)

// withCacheEnabled forces the response-cache feature flag on for a test and
// restores the previous value on cleanup, and starts from an empty cache so
// tests don't depend on global env or on each other.
func withCacheEnabled(t *testing.T) {
	t.Helper()
	prev := isEnabled
	isEnabled = func() bool { return true }
	t.Cleanup(func() { isEnabled = prev })
	responseCache.Clear()
}

func TestResponseCacheFilter_HitSkipsHandler(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: 30 * time.Second}

	calls := 0
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/x", opts, next)

	// 1st request: cache miss → handler runs, body returned, no HIT header.
	rec1 := httptest.NewRecorder()
	wrapped(rec1, httptest.NewRequest(http.MethodGet, "/x?r=1", nil), nil)
	if calls != 1 {
		t.Fatalf("miss: expected handler called once, got %d", calls)
	}
	if rec1.Body.String() != `{"ok":true}` {
		t.Fatalf("miss body = %q", rec1.Body.String())
	}
	if rec1.Header().Get("X-API-Cache") == "HIT" {
		t.Fatal("miss should not be marked HIT")
	}

	// 2nd request (same URL): cache hit → handler NOT called, body + type replayed.
	rec2 := httptest.NewRecorder()
	wrapped(rec2, httptest.NewRequest(http.MethodGet, "/x?r=1", nil), nil)
	if calls != 1 {
		t.Fatalf("hit: expected handler skipped, got calls=%d", calls)
	}
	if rec2.Body.String() != `{"ok":true}` {
		t.Fatalf("hit body = %q", rec2.Body.String())
	}
	if rec2.Header().Get("X-API-Cache") != "HIT" {
		t.Fatal("hit should be marked HIT")
	}
	if ct := rec2.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type not replayed: %q", ct)
	}

	// 3rd request (different query): different key → miss again.
	rec3 := httptest.NewRecorder()
	wrapped(rec3, httptest.NewRequest(http.MethodGet, "/x?r=2", nil), nil)
	if calls != 2 {
		t.Fatalf("different query: expected handler called, got calls=%d", calls)
	}
}

func TestResponseCacheFilter_BypassedWithoutOptionOrForNonGET(t *testing.T) {
	filter := &ResponseCacheFilter{}

	// No CacheTTL → next returned unchanged.
	if h := filter.ApplyFilter(string(api.GET), "/x", &api.HandlerOptions{}, nil); h != nil {
		t.Fatal("expected next unchanged when CacheTTL==0")
	}
	// Non-GET → next returned unchanged even with CacheTTL set.
	opts := &api.HandlerOptions{CacheTTL: time.Second}
	if h := filter.ApplyFilter(string(api.POST), "/x", opts, nil); h != nil {
		t.Fatal("expected next unchanged for non-GET")
	}
}

func TestResponseCacheFilter_NonOKNotCached(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: 30 * time.Second}

	calls := 0
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`err`))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/err", opts, next)

	for i := 0; i < 2; i++ {
		wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/err", nil), nil)
	}
	if calls != 2 {
		t.Fatalf("non-200 must not be cached; expected handler called twice, got %d", calls)
	}
}

// TestResponseCacheFilter_BodyAffectsKey ensures GET-with-body requests key the
// body into the cache, so two different bodies to the same URL are not confused.
// It also verifies the body is still readable by the downstream handler.
func TestResponseCacheFilter_BodyAffectsKey(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: 30 * time.Second}

	calls := 0
	lastBody := ""
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		_, _ = w.Write([]byte("ok"))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/x", opts, next)

	do := func(body string) {
		wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", strings.NewReader(body)), nil)
	}
	do("aaa") // miss
	do("aaa") // same body → hit
	do("bbb") // different body → miss
	if calls != 2 {
		t.Fatalf("body should affect key: expected handler called twice (miss,hit,miss), got %d", calls)
	}
	if lastBody != "bbb" {
		t.Fatalf("handler should see the restored body; last body = %q", lastBody)
	}
}

// TestResponseCacheFilter_CallerAffectsKey ensures the authenticated caller is
// part of the key, so one user's cached response is never served to another.
func TestResponseCacheFilter_CallerAffectsKey(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: 30 * time.Second}

	calls := 0
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		_, _ = w.Write([]byte("ok"))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/x", opts, next)

	doAs := func(userID string) {
		u := &security.UserSessionInfo{}
		u.SetUserID(userID)
		req := httptest.NewRequest(http.MethodGet, "/x", nil).
			WithContext(security.AddUserToContext(context.Background(), u))
		wrapped(httptest.NewRecorder(), req, nil)
	}
	doAs("alice") // miss
	doAs("alice") // same caller → hit
	doAs("bob")   // different caller → miss (no cross-user leak)
	if calls != 2 {
		t.Fatalf("caller should affect key: expected handler called twice, got %d", calls)
	}
}

// TestResponseCacheFilter_DisabledByFlag verifies the feature flag gates the
// whole cache: with it off, even an opted-in route runs the handler every time.
func TestResponseCacheFilter_DisabledByFlag(t *testing.T) {
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: 30 * time.Second}

	prev := isEnabled
	isEnabled = func() bool { return false }
	t.Cleanup(func() { isEnabled = prev })

	calls := 0
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		_, _ = w.Write([]byte("ok"))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/x", opts, next)
	for i := 0; i < 3; i++ {
		wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil), nil)
	}
	if calls != 3 {
		t.Fatalf("with flag disabled, every request must hit the handler; got %d", calls)
	}
}

// TestResponseCacheFilter_InvalidateAll verifies an active full purge.
func TestResponseCacheFilter_InvalidateAll(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: time.Minute}

	calls := 0
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls++
		_, _ = w.Write([]byte("ok"))
	}
	wrapped := filter.ApplyFilter(string(api.GET), "/x", opts, next)

	wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil), nil) // miss
	wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil), nil) // hit
	if calls != 1 {
		t.Fatalf("expected 1 call before purge, got %d", calls)
	}

	InvalidateAll()
	wrapped(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil), nil) // miss again
	if calls != 2 {
		t.Fatalf("expected 2 calls after InvalidateAll, got %d", calls)
	}
}

// TestResponseCacheFilter_InvalidatePath verifies a scoped purge by method+path
// leaves other cached paths untouched.
func TestResponseCacheFilter_InvalidatePath(t *testing.T) {
	withCacheEnabled(t)
	filter := &ResponseCacheFilter{}
	opts := &api.HandlerOptions{CacheTTL: time.Minute}

	calls := map[string]int{}
	next := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		calls[r.URL.Path]++
		_, _ = w.Write([]byte("ok"))
	}
	a := filter.ApplyFilter(string(api.GET), "/a", opts, next)
	b := filter.ApplyFilter(string(api.GET), "/b", opts, next)

	a(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/a", nil), nil) // miss a
	a(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/a", nil), nil) // hit a
	b(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/b", nil), nil) // miss b
	if calls["/a"] != 1 || calls["/b"] != 1 {
		t.Fatalf("setup: calls=%v", calls)
	}

	removed := InvalidatePath(string(api.GET), "/a")
	if removed != 1 {
		t.Fatalf("InvalidatePath(/a) removed %d, want 1", removed)
	}

	a(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/a", nil), nil) // miss a (purged)
	b(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/b", nil), nil) // hit b (untouched)
	if calls["/a"] != 2 || calls["/b"] != 1 {
		t.Fatalf("after purge: calls=%v (want /a=2, /b=1)", calls)
	}
}

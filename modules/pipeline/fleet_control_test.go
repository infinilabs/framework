/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

// The reverse channel is stubbed at the seam: tests capture what would be
// sent down the websocket and shape the response per instance.
type fakeReverse struct {
	mu       sync.Mutex
	calls    []fakeCall
	statusOf map[string]int // instance → response status (default 200)
}

type fakeCall struct {
	Instance string
	Method   string
	Path     string
}

func (f *fakeReverse) install(t *testing.T) {
	origProxy, origConn := reverseProxyRequest, reverseIsConnected
	reverseProxyRequest = func(peerID string, req *util.Request) (*util.Result, error) {
		f.mu.Lock()
		f.calls = append(f.calls, fakeCall{Instance: peerID, Method: req.Method, Path: req.Path})
		f.mu.Unlock()
		status := http.StatusOK
		if s, ok := f.statusOf[peerID]; ok {
			status = s
		}
		return &util.Result{StatusCode: status, Body: []byte(`{"ok":true}`)}, nil
	}
	reverseIsConnected = func(string) bool { return true }
	t.Cleanup(func() { reverseProxyRequest, reverseIsConnected = origProxy, origConn })
}

func (f *fakeReverse) recorded() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func callHandler(t *testing.T, h httprouter.Handle, method, target string, params httprouter.Params) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	w := httptest.NewRecorder()
	h(w, req, params)
	return w
}

func TestBroadcastDeploymentAction(t *testing.T) {
	env := newDeployTestEnv(t)
	dep := &PipelineDeployment{
		PipelineID: env.pid,
		Enabled:    true,
		Placement:  PlacementSpec{Strategy: "all", Instances: []string{env.instA, env.instB}},
	}
	dep.ID = "d1" // shared upsert id (deployment_targeting_test.go convention)
	dep.Name = "broadcast-test"
	env.saveDeployment(dep)

	fr := &fakeReverse{statusOf: map[string]int{env.instB: http.StatusConflict}} // B is on another generation
	fr.install(t)

	h := (&PipeModule{}).broadcastDeploymentActionHandler("pause")
	w := callHandler(t, h, http.MethodPost, "/pipeline/deployments/"+dep.ID+"/_pause?generation=7",
		httprouter.Params{{Key: "id", Value: dep.ID}})
	if w.Code != http.StatusOK {
		t.Fatalf("aggregate must be 200 even with per-instance failures: %d %s", w.Code, w.Body.String())
	}
	out := util.MapStr{}
	if err := util.FromJSONBytes(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["total"].(float64) != 2 || out["delivered"].(float64) != 1 {
		t.Fatalf("total=2 delivered=1 expected: %v", out)
	}

	calls := fr.recorded()
	if len(calls) != 2 {
		t.Fatalf("both targets must receive the command: %+v", calls)
	}
	for _, c := range calls {
		if c.Method != http.MethodPost {
			t.Fatalf("control commands are POSTs: %+v", c)
		}
		if c.Path != "/pipeline/task/"+env.pipeName+"/_pause?generation=7" {
			t.Fatalf("command must target the task by NAME with the generation guard: %+v", c)
		}
	}
}

func TestBroadcastUnreachableInstance(t *testing.T) {
	env := newDeployTestEnv(t)
	dep := &PipelineDeployment{
		PipelineID: env.pid,
		Enabled:    true,
		Placement:  PlacementSpec{Strategy: "all", Instances: []string{env.instA}},
	}
	dep.ID = "d1" // shared upsert id
	dep.Name = "broadcast-offline"
	env.saveDeployment(dep)

	origProxy, origConn := reverseProxyRequest, reverseIsConnected
	reverseIsConnected = func(string) bool { return false } // nobody dialed in
	t.Cleanup(func() { reverseProxyRequest, reverseIsConnected = origProxy, origConn })

	h := (&PipeModule{}).broadcastDeploymentActionHandler("stop")
	w := callHandler(t, h, http.MethodPost, "/pipeline/deployments/"+dep.ID+"/_stop",
		httprouter.Params{{Key: "id", Value: dep.ID}})
	out := util.MapStr{}
	_ = util.FromJSONBytes(w.Body.Bytes(), &out)
	if out["delivered"].(float64) != 0 {
		t.Fatalf("offline instance must not count as delivered: %v", out)
	}
}

func TestInstanceActionForwarding(t *testing.T) {
	env := newDeployTestEnv(t)
	fr := &fakeReverse{}
	fr.install(t)

	h := (&PipeModule{}).instanceActionHandler("drain")
	w := callHandler(t, h, http.MethodPost, "/pipeline/instances/"+env.instA+"/_drain?timeout_seconds=5",
		httprouter.Params{{Key: "id", Value: env.instA}})
	if w.Code != http.StatusOK {
		t.Fatalf("drain forward: %d %s", w.Code, w.Body.String())
	}
	calls := fr.recorded()
	if len(calls) != 1 || calls[0].Path != "/pipeline/_drain?timeout_seconds=5" {
		t.Fatalf("query string must ride along: %+v", calls)
	}

	// unknown instance → 404 before any reverse traffic
	before := len(fr.recorded())
	w = callHandler(t, h, http.MethodPost, "/pipeline/instances/ghost/_drain",
		httprouter.Params{{Key: "id", Value: "ghost"}})
	if w.Code != http.StatusNotFound || len(fr.recorded()) != before {
		t.Fatalf("ghost instance must 404 without proxying: %d", w.Code)
	}
}

func TestInstanceProxyAllowlist(t *testing.T) {
	env := newDeployTestEnv(t)
	fr := &fakeReverse{}
	fr.install(t)
	h := (&PipeModule{}).instanceProxyHandler

	// off-scope path refused
	w := callHandler(t, h, http.MethodGet, "/pipeline/instances/"+env.instA+"/proxy?path=/admin/",
		httprouter.Params{{Key: "id", Value: env.instA}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-pipeline path must be refused: %d", w.Code)
	}
	// traversal refused
	w = callHandler(t, h, http.MethodGet, "/pipeline/instances/"+env.instA+"/proxy?path=/pipeline/../../x",
		httprouter.Params{{Key: "id", Value: env.instA}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("traversal must be refused: %d", w.Code)
	}
	// in-scope read passes through, status code preserved
	fr.statusOf = map[string]int{env.instA: http.StatusConflict}
	w = callHandler(t, h, http.MethodGet, "/pipeline/instances/"+env.instA+"/proxy?path=/pipeline/task/x/flight",
		httprouter.Params{{Key: "id", Value: env.instA}})
	if w.Code != http.StatusConflict {
		t.Fatalf("instance status code must pass through verbatim: %d", w.Code)
	}
	calls := fr.recorded()
	if len(calls) != 1 || calls[0].Method != http.MethodGet || calls[0].Path != "/pipeline/task/x/flight" {
		t.Fatalf("proxy must forward the read as-is: %+v", calls)
	}
}

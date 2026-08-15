/* Copyright © INFINI LTD. All rights reserved. */

package crud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// mustCreate seeds one gizmo through the given create handler.
func mustCreate(t *testing.T, h Handlers, body string) string {
	t.Helper()
	w, out := call(t, h.Create, "POST", "/gizmos/", body)
	require.Equal(t, http.StatusOK, w.Code)
	id, _ := out["_id"].(string)
	require.NotEmpty(t, id)
	return id
}

func sourceOf(t *testing.T, h Handlers, id string) map[string]interface{} {
	t.Helper()
	w, out := call(t, h.Get, "GET", "/gizmos/"+id, "")
	require.Equal(t, http.StatusOK, w.Code)
	src, _ := out["_source"].(map[string]interface{})
	require.NotNil(t, src)
	return src
}

func TestUpdateModeFull_MergesOverLoadedObject(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		PrepareCreate: func(o *gizmo) error {
			if o.Status == "" {
				o.Status = "new"
			}
			return nil
		},
		UpdateMode: UpdateModeFull,
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)

	// full mode decodes the body over the loaded object: untouched fields
	// keep their stored values (merge semantics)
	w, out := call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"active"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "updated", out["result"])
	src := sourceOf(t, h, id)
	assert.Equal(t, "active", src["status"])
	assert.Equal(t, "alpha", src["name"])
}

func TestUpdateModeQueryFollowsReplaceParam(t *testing.T) {
	setupGizmos(t)
	var fullMode atomic.Bool
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		UpdateMode: UpdateModeQuery,
		// full mode hands PrepareUpdate the merged object (loaded fields
		// visible); partial mode only sets the id
		PrepareUpdate: func(o *gizmo, delta util.MapStr) error {
			fullMode.Store(o.Name != "")
			return nil
		},
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)

	// replace=false => partial delta (obj not loaded)
	_, out := call(t, h.Update, "PUT", "/gizmos/"+id+"?replace=false", `{"status":"active"}`)
	require.Equal(t, "updated", out["result"])
	assert.False(t, fullMode.Load(), "?replace=false should take the partial path")

	// default (no param) => full object (loaded, merge)
	fullMode.Store(false)
	_, out = call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"archived"}`)
	require.Equal(t, "updated", out["result"])
	assert.True(t, fullMode.Load(), "absent ?replace should take the full path")

	src := sourceOf(t, h, id)
	assert.Equal(t, "archived", src["status"])
	assert.Equal(t, "alpha", src["name"], "merge must keep untouched fields")
}

func TestUpdateFullModeProtectedFieldsRestored(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		UpdateMode:      UpdateModeFull,
		ProtectedFields: []string{"reserved", "created"},
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)
	// attempt to flip the protected field through the body
	_, out := call(t, h.Update, "PUT", "/gizmos/"+id, `{"name":"beta","reserved":true}`)
	require.Equal(t, "updated", out["result"])

	src := sourceOf(t, h, id)
	assert.Equal(t, "beta", src["name"])
	assert.Nil(t, src["reserved"], "protected field must be restored from the loaded object")
}

func TestProtectedFieldsStrippedFromDelta(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		ProtectedFields: []string{"reserved"},
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)
	_, out := call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"active","reserved":true}`)
	require.Equal(t, "updated", out["result"])

	src := sourceOf(t, h, id)
	assert.Nil(t, src["reserved"], "protected field must be stripped from the delta")
}

func TestPostHooksFire(t *testing.T) {
	setupGizmos(t)
	var created, updated, deleted atomic.Int32
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		PostCreate: func(o *gizmo) error { created.Add(1); return nil },
		PostUpdate: func(o *gizmo) error { updated.Add(1); return nil },
		PostDelete: func(o *gizmo) error { deleted.Add(1); return nil },
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)
	assert.EqualValues(t, 1, created.Load())

	_, out := call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"active"}`)
	require.Equal(t, "updated", out["result"])
	assert.EqualValues(t, 1, updated.Load())

	// failed delete (guard veto) must not fire the post hook
	w, _ := call(t, h.Delete, "DELETE", "/gizmos/nope", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.EqualValues(t, 0, deleted.Load())

	w, _ = call(t, h.Delete, "DELETE", "/gizmos/"+id, "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 1, deleted.Load())
}

func TestPostGetRefinesObject(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		PostGet: func(o *gizmo) error { o.Name = o.Name + "-refined"; return nil },
	})
	id := mustCreate(t, h, `{"name":"alpha"}`)
	src := sourceOf(t, h, id)
	assert.Equal(t, "alpha-refined", src["name"])
}

func TestPrepareSearchFilterAndPostSearchMutation(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		PrepareSearch: func(req *http.Request, builder *orm.QueryBuilder, ctx *orm.Context) error {
			// injected filter: only gizmos with the unique marker are visible
			builder.Filter(orm.TermsQuery("status", []string{"marker-xyz"}))
			return nil
		},
		PostSearch: func(res *elastic.SearchResponse) error {
			// per-hit decoration
			for i := range res.Hits.Hits {
				if res.Hits.Hits[i].Source == nil {
					res.Hits.Hits[i].Source = util.MapStr{}
				}
				res.Hits.Hits[i].Source["decorated"] = true
			}
			return nil
		},
	})

	// one gizmo carries the marker, one does not => only the first is visible
	id := mustCreate(t, h, `{"name":"alpha"}`)
	_, out := call(t, h.Update, "PUT", "/gizmos/"+id+"?replace=false", `{"status":"marker-xyz"}`)
	require.Equal(t, "updated", out["result"])
	mustCreate(t, h, `{"name":"beta"}`)

	w, out := call(t, h.Search, "GET", "/gizmos/_search?size=10", "")
	require.Equal(t, http.StatusOK, w.Code)
	hits, _ := out["hits"].(map[string]interface{})
	list, _ := hits["hits"].([]interface{})
	require.Len(t, list, 1)
	hit, _ := list[0].(map[string]interface{})
	src, _ := hit["_source"].(map[string]interface{})
	assert.Equal(t, "alpha", src["name"])
	assert.Equal(t, true, src["decorated"], "PostSearch decoration missing")
}

func TestCtxDecorateSeesActions(t *testing.T) {
	setupGizmos(t)
	var actions []string
	record := func(a string) { actions = append(actions, a) }
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		CtxDecorate: func(ctx *orm.Context, req *http.Request, action string) {
			record(action)
			ctx.Set("decorated_action", action)
		},
	})

	id := mustCreate(t, h, `{"name":"alpha"}`)
	call(t, h.Get, "GET", "/gizmos/"+id, "")
	call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"active"}`)
	call(t, h.Delete, "DELETE", "/gizmos/"+id, "")
	call(t, h.Search, "GET", "/gizmos/_search?size=10", "")

	assert.Equal(t, []string{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionSearch}, actions)
}

func TestIDParamOverride(t *testing.T) {
	setupGizmos(t)
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		IDParam: "gizmo_id",
	})

	// create via the standard helper (create has no id param)
	id := mustCreate(t, h, `{"name":"alpha"}`)

	// get with the custom param name
	req := httptest.NewRequest("GET", "/gizmos/"+id, nil)
	ps := httprouter.Params{{Key: "gizmo_id", Value: id}}
	w := httptest.NewRecorder()
	h.Get(w, req, ps)
	require.Equal(t, http.StatusOK, w.Code)
	out := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	assert.Equal(t, true, out["found"])
}

func TestBuildOptionsOrdering(t *testing.T) {
	cfg := Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo",
		Permission:   func(action string) api.PermissionKey { return api.PermissionKey("generic#gizmo/" + action) },
		MCP:          true,
		ExtraOptions: func(action string) []api.Option { return []api.Option{api.MCPTool("extra_probe", "x")} },
	}
	with := buildOptions(cfg, ActionRead, true)
	require.Len(t, with, 3, "permission + mcp + extra, in order")
	without := buildOptions(cfg, ActionSearch, false)
	require.Len(t, without, 2, "no-mcp variant drops only the mcp option")

	empty := Config[gizmo]{Prefix: "/gizmos", Resource: "gizmo"}
	assert.Nil(t, buildOptions(empty, ActionRead, true))
}

func TestRegisterCRUD_ExtraOptionsAndMCPOnGetOnly(t *testing.T) {
	setupGizmos(t)
	RegisterCRUD[gizmo](Config[gizmo]{
		Prefix:   "/gizmoroutes",
		Resource: "gizmo",
		IDParam:  "gizmo_id",
		MCP:      true,
		ExtraOptions: func(action string) []api.Option {
			// non-MCP extras (login/CORS/label style) are safe to apply to
			// every registration of the action, including POST _search
			return nil
		},
	})

	mcpRoutes := map[string]bool{}
	api.WalkMCPAutoUIMethodRoutes(func(route api.RegisteredUIMethodRoute) {
		if route.Route.Path == "/gizmoroutes/_search" ||
			route.Route.Path == "/gizmoroutes/:gizmo_id" ||
			route.Route.Path == "/gizmoroutes/" {
			mcpRoutes[route.Route.Path+" "+string(route.Route.Method)] = true
		}
	})
	assert.True(t, mcpRoutes["/gizmoroutes/_search GET"], "GET _search should be an MCP route: %v", mcpRoutes)
	assert.False(t, mcpRoutes["/gizmoroutes/_search POST"], "POST _search must not duplicate the MCP tool")
	assert.True(t, mcpRoutes["/gizmoroutes/:gizmo_id GET"], "custom id param should appear in routes")
}

func TestPostUpdateSeesPersistedObject(t *testing.T) {
	setupGizmos(t)
	var seenStatus atomic.Value
	h := NewHandlers[gizmo](Config[gizmo]{
		Prefix: "/gizmos", Resource: "gizmo", DefaultQueryFields: []string{"name"},
		PostUpdate: func(o *gizmo) error {
			seenStatus.Store(o.Status) // must reflect the persisted delta
			return nil
		},
	})
	id := mustCreate(t, h, `{"name":"alpha"}`)
	_, out := call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"reloaded"}`)
	require.Equal(t, "updated", out["result"])
	assert.Equal(t, "reloaded", seenStatus.Load())
}

func TestRegisterCRUD_SkipActions(t *testing.T) {
	setupGizmos(t)
	RegisterCRUD[gizmo](Config[gizmo]{
		Prefix:      "/gizmoskip",
		Resource:    "gizmo",
		MCP:         true,
		SkipActions: []string{ActionRead, ActionSearch},
	})

	seen := map[string]bool{}
	api.WalkMCPAutoUIMethodRoutes(func(route api.RegisteredUIMethodRoute) {
		if route.Route.Path == "/gizmoskip/:id" || route.Route.Path == "/gizmoskip/_search" {
			seen[route.Route.Path+" "+string(route.Route.Method)] = true
		}
	})
	assert.False(t, seen["/gizmoskip/:id GET"], "skipped read action must not register: %v", seen)
	assert.False(t, seen["/gizmoskip/_search GET"], "skipped search action must not register: %v", seen)
	// non-skipped actions on the same path still register
	assert.True(t, seen["/gizmoskip/:id PUT"], "update must still register: %v", seen)
	assert.True(t, seen["/gizmoskip/:id DELETE"], "delete must still register: %v", seen)
}

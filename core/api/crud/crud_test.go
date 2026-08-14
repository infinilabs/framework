/* Copyright © INFINI LTD. All rights reserved. */

package crud

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/sqlite"
)

// gizmo is the test model.
type gizmo struct {
	orm.ORMObjectBase
	Name     string `json:"name,omitempty" elastic_mapping:"name:{type:keyword}"`
	Status   string `json:"status,omitempty" elastic_mapping:"status:{type:keyword}"`
	Reserved bool   `json:"reserved,omitempty"`
}

func call(t *testing.T, h HandlerFunc, method, target, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	// crud uses :id params — parse from the path.
	ps := httprouter.Params{}
	if rest := trimPrefix(target, "/gizmos/"); rest != "" && !containsSlash(rest) {
		ps = httprouter.Params{{Key: "id", Value: rest}}
	}
	w := httptest.NewRecorder()
	h(w, req, ps)
	out := map[string]interface{}{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func trimPrefix(s, p string) string {
	if len(s) >= len(p) && s[:len(p)] == p {
		return s[len(p):]
	}
	return s
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}

// crudStoreOnce shares one registered sqlite handler across tests
// (orm.Register panics on duplicates within a binary).
var crudStoreOnce sync.Once

func setupGizmos(t *testing.T) Handlers {
	t.Helper()
	crudStoreOnce.Do(func() {
		handler := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{
			Enabled: true,
			DBPath:  filepath.Join(t.TempDir(), "crud.db"),
		}}
		if err := handler.Open(); err != nil {
			panic(err)
		}
		if err := handler.RegisterSchemaWithName(gizmo{}, "gizmos"); err != nil {
			panic(err)
		}
		orm.Register("sqlite", handler)
	})

	cfg := Config[gizmo]{
		Prefix:             "/gizmos",
		Resource:           "gizmo",
		DefaultQueryFields: []string{"name"},
		PrepareCreate: func(obj *gizmo) error {
			if obj.Name == "" {
				return errString("name is required")
			}
			if obj.Status == "" {
				obj.Status = "new"
			}
			return nil
		},
		GuardDelete: func(obj *gizmo) error {
			if obj.Reserved {
				return errString("reserved gizmo cannot be deleted")
			}
			return nil
		},
	}
	return NewHandlers[gizmo](cfg)
}

type errString string

func (e errString) Error() string { return string(e) }

func TestCRUD_FullFlow(t *testing.T) {
	h := setupGizmos(t)

	// Create: envelope {_id, result} + defaults applied.
	w, out := call(t, h.Create, "POST", "/gizmos/", `{"name":"alpha"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "created", out["result"])
	id, _ := out["_id"].(string)
	require.NotEmpty(t, id)

	// Validation error → 400.
	w, _ = call(t, h.Create, "POST", "/gizmos/", `{"name":""}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Get: {found, _id, _source} envelope with the default applied.
	w, out = call(t, h.Get, "GET", "/gizmos/"+id, "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, out["found"])
	src, _ := out["_source"].(map[string]interface{})
	require.NotNil(t, src)
	assert.Equal(t, "alpha", src["name"])
	assert.Equal(t, "new", src["status"])

	// Get missing → 404 envelope.
	w, out = call(t, h.Get, "GET", "/gizmos/nope", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, false, out["found"])

	// Update: partial delta preserves untouched fields (P1 merge contract).
	w, out = call(t, h.Update, "PUT", "/gizmos/"+id, `{"status":"active"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "updated", out["result"])
	w, out = call(t, h.Get, "GET", "/gizmos/"+id, "")
	src, _ = out["_source"].(map[string]interface{})
	assert.Equal(t, "active", src["status"])
	assert.Equal(t, "alpha", src["name"])

	// Update missing → 404.
	w, _ = call(t, h.Update, "PUT", "/gizmos/nope", `{"status":"x"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Search: ES-shaped response with default created DESC sort.
	for _, name := range []string{"beta", "gamma"} {
		_, out := call(t, h.Create, "POST", "/gizmos/", `{"name":"`+name+`"}`)
		require.NotEmpty(t, out["_id"])
	}
	w, out = call(t, h.Search, "GET", "/gizmos/_search?size=10", "")
	require.Equal(t, http.StatusOK, w.Code)
	hits, _ := out["hits"].(map[string]interface{})
	require.NotNil(t, hits, "ES-shaped response")
	list, _ := hits["hits"].([]interface{})
	assert.Len(t, list, 3)
	total, _ := hits["total"].(map[string]interface{})
	assert.EqualValues(t, 3, total["value"])

	// Search with filter.
	w, out = call(t, h.Search, "GET", "/gizmos/_search?filter=name:alpha&size=10", "")
	hits, _ = out["hits"].(map[string]interface{})
	list, _ = hits["hits"].([]interface{})
	assert.Len(t, list, 1)

	// Delete: reserved guard blocks with 403.
	reserved := gizmo{Name: "keep", Reserved: true}
	reserved.ID = "keep-1"
	require.NoError(t, orm.Save(orm.NewContext(), &reserved))
	w, _ = call(t, h.Delete, "DELETE", "/gizmos/keep-1", "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Delete: ack envelope, then gone.
	w, out = call(t, h.Delete, "DELETE", "/gizmos/"+id, "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "deleted", out["result"])
	w, out = call(t, h.Get, "GET", "/gizmos/"+id, "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Delete missing → 404.
	w, _ = call(t, h.Delete, "DELETE", "/gizmos/nope", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCRUD_SharingContext(t *testing.T) {
	setupGizmos(t) // registers the sqlite handler for this test too
	// SharingResource is exercised via the get path's context keys — assert
	// by configuring and confirming no behavioral difference (the keys are
	// consumed by security hooks that are absent here).
	cfg := Config[gizmo]{Prefix: "/gizmos", Resource: "gizmo", SharingResource: "gizmo"}
	hh := NewHandlers[gizmo](cfg)
	w, _ := call(t, hh.Search, "GET", "/gizmos/_search?size=1", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterCRUD_ValidatesConfig(t *testing.T) {
	called := false
	defer func() {
		if r := recover(); r != nil {
			called = true
		}
	}()
	RegisterCRUD[gizmo](Config[gizmo]{Prefix: "no-slash", Resource: "x"})
	assert.True(t, called, "bad prefix must panic")
}

var _ = util.MapStr{}

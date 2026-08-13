/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/orm"
)

// indexNestedSpec is a nested object struct whose scalar fields should get
// dotted-path indexes ($.cpu.model, $.cpu.physical_cpu).
type indexNestedSpec struct {
	Model       string `json:"model,omitempty" elastic_mapping:"model: { type: keyword }"`
	PhysicalCPU int    `json:"physical_cpu,omitempty" elastic_mapping:"physical_cpu: { type: integer }"`
	// text is not B-tree-indexable.
	Description string `json:"description,omitempty" elastic_mapping:"description: { type: text }"`
}

// indexSliceItem is the element of a nested array; its fields must NOT get an
// index because json_extract cannot address fields inside a JSON array.
type indexSliceItem struct {
	Key string `json:"key,omitempty" elastic_mapping:"key: { type: keyword }"`
}

// indexRootModel exercises every branch of createExpressionIndexes: promoted
// ORMObjectBase fields, a top-level keyword, a nested struct object, a nested
// pointer-to-struct object, a slice of scalars, a slice of structs (nested),
// and an enabled:false object backed by a map.
type indexRootModel struct {
	orm.ORMObjectBase
	Name   string                 `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	CPU    indexNestedSpec        `json:"cpu,omitempty" elastic_mapping:"cpu: { type: object }"`
	Disk   *indexNestedSpec       `json:"disk,omitempty" elastic_mapping:"disk: { type: object }"`
	Tags   []string               `json:"tags,omitempty" elastic_mapping:"tags: { type: keyword }"`
	Items  []indexSliceItem       `json:"items,omitempty" elastic_mapping:"items: { type: nested }"`
	Secret map[string]interface{} `json:"secret,omitempty" elastic_mapping:"secret: { type: object, enabled: false }"`
}

func openIndexTestDB(t *testing.T) (*SQLiteORM, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: filepath.Join(tmpDir, "test.db")}}
	require.NoError(t, handler.Open())
	require.NoError(t, handler.RegisterSchemaWithName(indexRootModel{}, "index_root"))
	return handler, func() { handler.Close() }
}

// sqliteIndexDDL returns map[indexName]ddlSQL for the given table.
func sqliteIndexDDL(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name=?", table)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, ddl sql.NullString
		require.NoError(t, rows.Scan(&name, &ddl))
		out[name.String] = ddl.String
	}
	require.NoError(t, rows.Err())
	return out
}

func TestCreateExpressionIndexes_NestedObjects(t *testing.T) {
	handler, cleanup := openIndexTestDB(t)
	defer cleanup()
	idx := sqliteIndexDDL(t, handler.DB, "index_root")

	// Each expected path must have an index whose DDL targets exactly that
	// json_extract path (the expression must match the query builder verbatim
	// for SQLite to use it).
	expectedPaths := []string{
		"id", "created", "updated", // promoted from orm.ORMObjectBase
		"name",
		"cpu.model", "cpu.physical_cpu",
		"disk.model", "disk.physical_cpu",
		"tags",
	}
	for _, p := range expectedPaths {
		name := indexNameFor("index_root", p)
		ddl, ok := idx[name]
		require.Truef(t, ok, "expected index %s for path $.%s to exist", name, p)
		wantExpr := fmt.Sprintf("json_extract(raw, '$.%s')", p)
		assert.Containsf(t, ddl, wantExpr, "index %s DDL should target %s", name, wantExpr)
	}

	// Explicitly NOT indexed: text leaf, nested-array element, enabled:false
	// subtree, the object/map roots themselves, and the _system map.
	for _, p := range []string{
		"cpu.description",
		"items.key",
		"secret",
		"cpu", "disk", "items",
		"_system",
	} {
		_, present := idx[indexNameFor("index_root", p)]
		assert.Falsef(t, present, "no index should exist for $.%s", p)
	}
}

// TestCreateExpressionIndexes_NestedQueryUsesIndex proves a query on a nested
// field actually picks up the dotted expression index (no full-table scan).
func TestCreateExpressionIndexes_NestedQueryUsesIndex(t *testing.T) {
	handler, cleanup := openIndexTestDB(t)
	defer cleanup()

	// Insert enough rows that the planner prefers the index over a scan.
	for i := 0; i < 200; i++ {
		raw := fmt.Sprintf(`{"id":"id_%d","name":"n%d","cpu":{"model":"m%d","physical_cpu":%d},"disk":{"model":"d%d"}}`,
			i, i, i%20, i, i%20)
		_, err := handler.DB.Exec("INSERT INTO index_root (id, raw) VALUES (?, ?)", fmt.Sprintf("id_%d", i), raw)
		require.NoError(t, err)
	}

	cases := []struct {
		name string
		path string // json_extract path used in WHERE
		want string // index name expected in the plan
	}{
		{"nested keyword", "cpu.model", indexNameFor("index_root", "cpu.model")},
		{"nested pointer keyword", "disk.model", indexNameFor("index_root", "disk.model")},
		{"top-level keyword regression", "name", indexNameFor("index_root", "name")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf("EXPLAIN QUERY PLAN SELECT id FROM index_root WHERE json_extract(raw, '$.%s') = 'm3'", tc.path)
			detail := queryPlanDetail(t, handler.DB, q)
			joined := strings.Join(detail, "\n")
			assert.Containsf(t, joined, tc.want, "plan should use index %s; got:\n%s", tc.want, joined)
		})
	}

	// A non-indexed field (text) must NOT claim an index — it scans.
	t.Run("text field scans", func(t *testing.T) {
		q := "EXPLAIN QUERY PLAN SELECT id FROM index_root WHERE json_extract(raw, '$.cpu.description') = 'x'"
		detail := queryPlanDetail(t, handler.DB, q)
		joined := strings.Join(detail, "\n")
		assert.NotContains(t, joined, "USING INDEX", "text field should not use an index; got:\n%s", joined)
	})
}

func queryPlanDetail(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()
	rows, err := db.Query(query)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		out = append(out, detail)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, out, "EXPLAIN QUERY PLAN returned no rows for: %s", query)
	return out
}

func TestParseMappingTag(t *testing.T) {
	cases := []struct {
		tag    string
		field  string
		esType string
		ok     bool
	}{
		{`stream_id:{type:keyword}`, "stream_id", "keyword", true},
		{`created: { type: date }`, "created", "date", true},
		{`cpu: { type: object }`, "cpu", "object", true},
		{`stats: { properties: { x: { type: keyword } } }`, "stats", "", false}, // no top-level type:
		{`s: { subtype: keyword }`, "s", "", false},                             // "type:" inside "subtype:" must not match
		{`enabled-only`, "", "", false},                                         // no colon
		{"", "", "", false},
	}
	for _, c := range cases {
		f, ty, ok := parseMappingTag(c.tag)
		assert.Equalf(t, c.field, f, "field mismatch for %q", c.tag)
		assert.Equalf(t, c.esType, ty, "type mismatch for %q", c.tag)
		assert.Equalf(t, c.ok, ok, "ok mismatch for %q", c.tag)
	}
}

func TestMappingDisabled(t *testing.T) {
	assert.True(t, mappingDisabled(`payload:{type:object,enabled:false}`))
	assert.True(t, mappingDisabled(`payload: { type: object, enabled: false }`))
	assert.False(t, mappingDisabled(`cpu: { type: object }`))
	assert.False(t, mappingDisabled(`name:{type:keyword}`))
}

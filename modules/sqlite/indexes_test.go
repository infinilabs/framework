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

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	sqliteOrm "infini.sh/framework/modules/sqlite/orm"
)

// indexNestedSpec is a nested object struct whose scalar fields should get
// dotted-path generated columns ($.cpu.model, $.cpu.physical_cpu).
type indexNestedSpec struct {
	Model       string `json:"model,omitempty" elastic_mapping:"model: { type: keyword }"`
	PhysicalCPU int    `json:"physical_cpu,omitempty" elastic_mapping:"physical_cpu: { type: integer }"`
	// text goes to the FTS table, not a scalar column.
	Description string `json:"description,omitempty" elastic_mapping:"description: { type: text }"`
}

// indexSliceItem is the element of a nested array; its fields must NOT be
// promoted because json_extract cannot address fields inside a JSON array.
type indexSliceItem struct {
	Key string `json:"key,omitempty" elastic_mapping:"key: { type: keyword }"`
}

// indexRootModel exercises every branch of the field walker: promoted
// ORMObjectBase fields, a top-level keyword, a nested struct object, a
// nested pointer-to-struct object, a slice of scalars, a slice of structs
// (nested), and an enabled:false object backed by a map.
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

// columnIndexName is the flattened-scheme index name for a JSON path.
func columnIndexName(table, path string) string {
	return "ixc_" + indexNameFor(table, path)[3:]
}

func TestFlattened_NestedObjects(t *testing.T) {
	handler, cleanup := openIndexTestDB(t)
	defer cleanup()

	cols, err := tableColumns(handler.DB, "index_root")
	require.NoError(t, err)
	idx := sqliteIndexDDL(t, handler.DB, "index_root")

	// Every promoted scalar leaf (including dotted nested paths) becomes a
	// generated column plus a plain column index.
	expectedPaths := []string{
		"created", "updated", // promoted from orm.ORMObjectBase
		"name",
		"cpu.model", "cpu.physical_cpu",
		"disk.model", "disk.physical_cpu",
		"tags",
	}
	for _, p := range expectedPaths {
		assert.Truef(t, cols[p], "expected generated column %q to exist", p)
		idxName := columnIndexName("index_root", p)
		ddl, ok := idx[idxName]
		require.Truef(t, ok, "expected column index %s for path $.%s", idxName, p)
		want := `"` + p + `"`
		assert.Containsf(t, ddl, want, "index %s should target column %q", idxName, p)
	}

	// Text leaves DO get generated columns (FTS trigger sources) but no
	// B-tree index; array elements, enabled:false subtrees, object roots and
	// the _system map get neither.
	for _, p := range []string{
		"cpu.description", "disk.description",
	} {
		assert.Truef(t, cols[p], "text leaf %q should have a generated column (FTS source)", p)
		_, has := idx[columnIndexName("index_root", p)]
		assert.Falsef(t, has, "text leaf %q must not get a B-tree index", p)
	}
	for _, p := range []string{
		"items.key",
		"secret",
		"cpu", "disk", "items",
		"_system",
	} {
		assert.Falsef(t, cols[p], "no generated column should exist for $.%s", p)
	}

	// Text leaves sync into the FTS table under sanitized column names.
	ftsCols, err := tableColumns(handler.DB, "fts_index_root")
	require.NoError(t, err)
	assert.True(t, ftsCols["cpu_description"], "text leaf should join the FTS table")
	assert.True(t, ftsCols["disk_description"], "pointer text leaf should join the FTS table")
}

// TestFlattened_NestedQueryUsesIndex proves a query translated through the
// resolver on a nested field hits the column index (no full-table scan).
func TestFlattened_NestedQueryUsesIndex(t *testing.T) {
	handler, cleanup := openIndexTestDB(t)
	defer cleanup()

	// Insert enough rows that the planner prefers the index over a scan.
	for i := 0; i < 200; i++ {
		raw := fmt.Sprintf(`{"id":"id_%d","name":"n%d","cpu":{"model":"m%d","physical_cpu":%d},"disk":{"model":"d%d"}}`,
			i, i, i%20, i, i%20)
		_, err := handler.DB.Exec("INSERT INTO index_root (id, raw) VALUES (?, ?)", fmt.Sprintf("id_%d", i), raw)
		require.NoError(t, err)
	}

	schema := lookupTableSchema("index_root")
	require.NotNil(t, schema)
	resolver := schema.resolver()

	cases := []struct {
		name  string
		path  string
		value string
		want  int
	}{
		{"nested keyword", "cpu.model", "m3", 10},
		{"nested pointer keyword", "disk.model", "d3", 10},
		{"top-level keyword regression", "name", "n3", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qb := orm.NewQuery().Filter(orm.TermQuery(tc.path, tc.value))
			qb.Build()
			where, args := sqliteOrm.BuildWhereClause(qb, resolver)
			require.NotEmpty(t, where)

			want := columnIndexName("index_root", tc.path)
			q := "EXPLAIN QUERY PLAN SELECT id FROM index_root WHERE " + where
			detail := queryPlanDetail(t, handler.DB, q, args...)
			joined := strings.Join(detail, "\n")
			assert.Containsf(t, joined, want, "plan should use index %s; got:\n%s", want, joined)

			// The clause must actually match rows via the generated column.
			var n int
			require.NoError(t, handler.DB.QueryRow("SELECT COUNT(*) FROM index_root WHERE "+where, args...).Scan(&n))
			assert.Equal(t, tc.want, n)
		})
	}

	// An unmapped path falls back to json_extract (correct, unindexed).
	t.Run("unmapped path falls back", func(t *testing.T) {
		expr, _, fts := resolver("items.key")
		assert.Nil(t, fts)
		assert.Equal(t, "json_extract(raw, '$.items.key')", expr)
	})

	// The id path maps to the primary key column, not a generated column.
	t.Run("id path maps to pk", func(t *testing.T) {
		expr, _, fts := resolver("id")
		assert.Nil(t, fts)
		assert.Equal(t, `"id"`, expr)
	})
}

// TestFlattened_MigratesLegacyTable proves an old-layout table (id+raw only,
// pre-generated-columns) is rebuilt in place with data preserved.
func TestFlattened_MigratesLegacyTable(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: filepath.Join(tmpDir, "test.db")}}
	require.NoError(t, handler.Open())
	defer handler.Close()

	// Create the legacy layout directly and seed rows.
	_, err := handler.DB.Exec(`CREATE TABLE legacy_items (id TEXT PRIMARY KEY, raw JSON NOT NULL)`)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		raw := fmt.Sprintf(`{"id":"L%d","status":"st%d","name":"legacy-%d"}`, i, i%2, i)
		_, err := handler.DB.Exec("INSERT INTO legacy_items (id, raw) VALUES (?, ?)", fmt.Sprintf("L%d", i), raw)
		require.NoError(t, err)
	}

	// Registering the model migrates the table.
	require.NoError(t, handler.RegisterSchemaWithName(TestItem{}, "legacy_items"))

	cols, err := tableColumns(handler.DB, "legacy_items")
	require.NoError(t, err)
	assert.True(t, cols["status"], "status should be promoted after migration")
	assert.True(t, cols["name"], "name should be promoted after migration")

	var count int
	require.NoError(t, handler.DB.QueryRow("SELECT COUNT(*) FROM legacy_items").Scan(&count))
	assert.Equal(t, 5, count, "rows must survive the migration")

	// Filtered search works on the migrated column.
	qb := orm.NewQuery().Filter(orm.TermQuery("status", "st1"))
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})
	res, err := handler.SearchV2(ctx, qb)
	require.NoError(t, err)
	items, _, err := elastic.DecodeHits[TestItem](res)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func queryPlanDetail(t *testing.T, db *sql.DB, query string, args ...interface{}) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
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

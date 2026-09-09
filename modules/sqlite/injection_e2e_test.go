package sqlite

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

func seedItems(t *testing.T, handler *SQLiteORM) {
	t.Helper()
	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "e1", Created: &now, Updated: &now}, Name: "Alice", Status: "active", Age: 30},
		{ORMObjectBase: orm.ORMObjectBase{ID: "e2", Created: &now, Updated: &now}, Name: "Bob", Status: "pending", Age: 25},
		{ORMObjectBase: orm.ORMObjectBase{ID: "e3", Created: &now, Updated: &now}, Name: "Carol", Status: "active", Age: 35},
	}
	for _, item := range items {
		require.NoError(t, handler.Create(nil, item))
	}
}

func newSearchCtx() *orm.Context {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})
	return ctx
}

func countHits(t *testing.T, res *orm.SearchResult) int64 {
	t.Helper()
	require.NotNil(t, res)
	var response map[string]interface{}
	require.NoError(t, util.FromJSONBytes(res.Payload.([]byte), &response))
	hits, _ := response["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	value, _ := total["value"].(float64)
	return int64(value)
}

// The original vulnerability: GET /transfer/jobs?filter=no_such_field'+OR+'1'='1:x
// made the resolver splice the raw field into json_extract and return
// "SQL logic error" (500), with a working boolean-blind injection behind it.
// After the fix the same query must behave exactly like a filter on a
// nonexistent field: valid SQL, zero rows, no error.
func TestSQLiteORM_SearchV2_InjectionViaFilter(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()
	seedItems(t, handler)

	payloads := []string{
		"no_such_field' OR '1'='1",
		"a' or '1'='1') or '1'='2",
		"id; DROP TABLE test_items--",
		"x') UNION SELECT name FROM sqlite_master--",
		"status' AND 1=('1",
	}
	for _, payload := range payloads {
		qb := orm.NewQuery()
		qb.Filter(orm.TermQuery(payload, "x"))
		qb.Size(10)
		res, err := handler.SearchV2(newSearchCtx(), qb)
		assert.NoError(t, err, "payload %q must not produce a SQL error", payload)
		if err == nil {
			assert.Equal(t, int64(0), countHits(t, res), "injected filter must match nothing, payload=%q", payload)
		}
	}

	// Regression: the table still exists and legit filters still work.
	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("status", "active"))
	res, err := handler.SearchV2(newSearchCtx(), qb)
	require.NoError(t, err)
	assert.Equal(t, int64(2), countHits(t, res))
}

// The exact HTTP attack surface: NewQueryBuilderFromRequest parses
// ?filter=FIELD:value and ?sort=FIELD:dir with attacker-controlled FIELD.
func TestSQLiteORM_SearchV2_InjectionViaHTTPRequest(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()
	seedItems(t, handler)

	// raw query strings as an attacker's client would send them (spaces
	// are fine unencoded in the query component; quotes may appear raw)
	attacks := []string{
		"filter=" + urlEscape("no_such_field' OR '1'='1:x") + "&size=10",
		"sort=" + urlEscape("name' OR '1'='1:desc") + "&size=10",
		"sort=" + urlEscape("age; DROP TABLE test_items--:asc") + "&size=10",
		"filter=" + urlEscape("a') UNION SELECT 1--:b") + "&sort=" + urlEscape("id':desc") + "&size=10",
	}
	for _, rawQuery := range attacks {
		req := httptest.NewRequest("GET", "/search?"+rawQuery, nil)
		qb, err := orm.NewQueryBuilderFromRequest(req)
		require.NoError(t, err, "attack %q must parse", rawQuery)
		res, err := handler.SearchV2(newSearchCtx(), qb)
		assert.NoError(t, err, "attack %q must not produce a SQL error", rawQuery)
		if err == nil {
			require.NotNil(t, res)
		}
	}

	// Nothing was destroyed by the sort/filter payloads.
	count, err := handler.Count(&TestItem{}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func urlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "'", "%27")
}

// Hostile sort fields must not error; ordering degrades to a constant while
// legit sorts keep ordering correctly.
func TestSQLiteORM_SearchV2_InjectionViaSort(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()
	seedItems(t, handler)

	for _, hostile := range []string{"name' OR '1'='1", "age;--", "id)"} {
		qb := orm.NewQuery()
		qb.SortBy(orm.Sort{Field: hostile, SortType: orm.DESC})
		qb.Size(10)
		res, err := handler.SearchV2(newSearchCtx(), qb)
		assert.NoError(t, err, "hostile sort %q must not error", hostile)
		require.NotNil(t, res)
	}

	// Legit sort still orders (age desc => ids e3,e1,e2).
	qb := orm.NewQuery()
	qb.SortBy(orm.Sort{Field: "age", SortType: orm.DESC})
	res, err := handler.SearchV2(newSearchCtx(), qb)
	require.NoError(t, err)
	ids := requireSearchResultIDs(t, res)
	assert.Equal(t, []string{"e3", "e1", "e2"}, ids)
}

// DeleteByQuery with an injected field must delete nothing — the injection
// must never escalate from a read to data loss.
func TestSQLiteORM_DeleteByQuery_InjectionDeletesNothing(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()
	seedItems(t, handler)

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("no_such_field' OR '1'='1", "x"))
	resp, err := handler.DeleteByQuery(newSearchCtx(), qb)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.Deleted, "injected filter must not match (let alone delete) anything")

	count, err := handler.Count(&TestItem{}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// GroupBy's group field is also a raw identifier site.
func TestSQLiteORM_GroupBy_Injection(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()
	seedItems(t, handler)

	err, groups := handler.GroupBy(&TestItem{}, "", "status' OR '1'='1", "", nil)
	assert.NoError(t, err, "hostile group field must not error")
	require.NotNil(t, groups)
	// The degraded NULL expression groups everything into the empty-string
	// bucket — no leaked breakout, no SQL error.
	assert.Equal(t, int64(3), groups[""])

	// Legit grouping still works.
	err, groups = handler.GroupBy(&TestItem{}, "", "status", "", nil)
	assert.NoError(t, err)
	require.NotNil(t, groups)
	assert.Equal(t, int64(2), groups["active"])
	assert.Equal(t, int64(1), groups["pending"])
}

// buildLegacyWhere (legacy Search API conds) neutralizes hostile fields too.
func TestBuildLegacyWhere_Injection(t *testing.T) {
	conds := []*orm.Cond{
		{Field: "no_such_field' OR '1'='1", QueryType: orm.Match, Value: "x"},
		{Field: "ok", QueryType: orm.Match, Value: "y"},
	}
	clauses, args := buildLegacyWhere(conds)
	require.NotEmpty(t, clauses)
	joined := strings.Join(clauses, " AND ")
	assert.Contains(t, joined, "NULL")
	assert.NotContains(t, joined, "'1'='1")
	assert.Equal(t, "y", args[len(args)-1])
}

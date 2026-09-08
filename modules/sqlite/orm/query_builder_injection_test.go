package orm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"infini.sh/framework/core/orm"
)

// injectionPayloads is the canonical attack corpus for field names. These
// arrive from ?filter=FIELD:value and ?sort=FIELD query parameters, where
// FIELD is fully attacker-controlled.
var injectionPayloads = []string{
	"no_such_field' OR '1'='1",
	"x') OR ('1'='1",
	"id; DROP TABLE transfer-jobs--",
	"id') UNION SELECT access_token FROM agents--",
	"a\"b",
	"foo bar",
	"status' AND json_extract(raw, '$.password') LIKE 'a",
	"\\' OR 1=1--",
}

// hostileMarkers are fragments that must never survive into generated SQL
// (they would mean the payload crossed the string-literal boundary or was
// interpreted as SQL syntax).
func assertNoBreakout(t *testing.T, where string) {
	t.Helper()
	lower := strings.ToLower(where)
	for _, marker := range []string{"'1'='1", "or 1=1", "union", "drop table", ";", "--", "\""} {
		assert.NotContains(t, lower, marker, "generated SQL contains breakout marker %q: %s", marker, where)
	}
}

// Every leaf operator that renders a field expression must neutralize a
// hostile field name: the expression degrades to NULL, and the value stays
// a bound parameter.
func TestBuildWhereClause_Injection_AllOperators(t *testing.T) {
	type opCase struct {
		name   string
		clause *orm.Clause
		values []interface{} // expected bound args (subset match, order kept)
	}
	for _, payload := range injectionPayloads {
		cases := []opCase{
			{"term", orm.TermQuery(payload, "v"), []interface{}{"v"}},
			{"terms", orm.TermsQuery(payload, []string{"a", "b"}), []interface{}{"a", "b"}},
			{"in", orm.InQuery(payload, []interface{}{1, 2}), []interface{}{1, 2}},
			{"notin", orm.NotInQuery(payload, []interface{}{1}), []interface{}{1}},
			{"match", orm.MatchQuery(payload, "v"), []interface{}{"v"}},
			{"multimatch", orm.MultiMatchQuery([]string{payload, "other"}, "v"), []interface{}{"%v%"}},
			{"prefix", orm.PrefixQuery(payload, "v"), []interface{}{"v%"}},
			{"wildcard", orm.WildcardQuery(payload, "a*b"), []interface{}{"a%b"}},
			{"regexp", orm.RegexpQuery(payload, "v"), []interface{}{"%v%"}},
			{"exists", orm.ExistsQuery(payload), nil},
			{"fuzzy", orm.FuzzyQuery(payload, "v", 1), []interface{}{"%v%"}},
			{"matchphrase", orm.MatchPhraseQuery(payload, "v", 0), []interface{}{"%v%"}},
			{"querystring", orm.QueryStringQuery(payload, "v", "AND"), []interface{}{"%v%"}},
			{"range_gte", orm.Range(payload).Gte(10), []interface{}{10}},
			{"range_lte", orm.Range(payload).Lte(10), []interface{}{10}},
			{"range_gt", orm.Range(payload).Gt(10), []interface{}{10}},
			{"range_lt", orm.Range(payload).Lt(10), []interface{}{10}},
		}
		for _, tc := range cases {
			t.Run(tc.name+"/"+payload, func(t *testing.T) {
				qb := orm.NewQuery()
				qb.Filter(tc.clause)
				where, args := BuildWhereClause(qb, nil)
				if where == "" {
					// notin on hostile can legitimately produce "NOT NULL" or
					// empty via termsToSQL's empty branch — both are safe.
					assert.LessOrEqual(t, len(where), 8, "unexpected SQL: %s", where)
					return
				}
				assert.Contains(t, where, "NULL", "hostile field must degrade to NULL, got: %s", where)
				assertNoBreakout(t, where)
				for _, v := range tc.values {
					assert.Contains(t, args, v, "value must remain a bound parameter")
				}
			})
		}
	}
}

// Compound clauses (must / filter / must_not / should) neutralize hostile
// children the same way.
func TestBuildWhereClause_Injection_CompoundClauses(t *testing.T) {
	payload := "no_such_field' OR '1'='1"
	evil := orm.TermQuery(payload, "v")
	good := orm.TermQuery("status", "active")

	qb := orm.NewQuery()
	qb.Must(evil, good)
	where, _ := BuildWhereClause(qb, nil)
	assert.Contains(t, where, "NULL")
	assert.Contains(t, where, "json_extract(raw, '$.status')")
	assertNoBreakout(t, where)

	qb = orm.NewQuery()
	qb.Filter(orm.MustNotQuery(evil))
	where, _ = BuildWhereClause(qb, nil)
	assert.Contains(t, where, "NULL")
	assertNoBreakout(t, where)

	qb = orm.NewQuery()
	qb.Should(evil, good)
	where, _ = BuildWhereClause(qb, nil)
	assert.Contains(t, where, "NULL")
	assertNoBreakout(t, where)
}

// The regression guard: legitimate dotted paths still render the same SQL
// as before the fix.
func TestBuildWhereClause_LegitimateFieldsUnchanged(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("system.name", "x"))
	where, args := BuildWhereClause(qb, nil)
	assert.Equal(t, "json_extract(raw, '$.system.name') = ?", where)
	assert.Equal(t, []interface{}{"x"}, args)
}

// A resolver that maps some fields (the schema-promoted case) must keep
// returning its whitelisted expressions untouched, while unmapped hostile
// paths degrade via the resolver's own fallback.
func TestExprFor_ResolverPaths(t *testing.T) {
	resolve := func(path string) (string, string, *FTSPlan) {
		if path == "status" {
			return `["status"]`, "", nil
		}
		return SafeJSONPathExpr(path), "", nil // mirrors tableSchema.resolver fallback
	}
	assert.Equal(t, `["status"]`, ExprFor(resolve, "status"))
	assert.Equal(t, "NULL", ExprFor(resolve, "no_such_field' OR '1'='1"))
	rangeExpr, rangeEpoch := RangeExprFor(resolve, "status")
	assert.Equal(t, `["status"]`, rangeExpr)
	assert.False(t, rangeEpoch)
	hostileExpr, _ := RangeExprFor(resolve, "a' OR '1'='1")
	assert.Equal(t, "NULL", hostileExpr)
}

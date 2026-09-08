package orm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The identifiers below mirror real model field paths: dotted JSON paths of
// registered structs, with optional dashes and numeric array indices.
func TestSafeJSONPathExpr_ValidPaths(t *testing.T) {
	valid := map[string]string{
		"status":       "json_extract(raw, '$.status')",
		"system.name":  "json_extract(raw, '$.system.name')",
		"a_b.c-d":      "json_extract(raw, '$.a_b.c-d')",
		"arr[0]":       "json_extract(raw, '$.arr[0]')",
		"x9.y-8_z":     "json_extract(raw, '$.x9.y-8_z')",
		"metadata.ip0": "json_extract(raw, '$.metadata.ip0')",
	}
	for path, want := range valid {
		assert.Equal(t, want, SafeJSONPathExpr(path), "path %q should render as json_extract", path)
	}
}

// Every classic string-literal breakout, comment, stacked-statement and
// format-string probe must degrade to the literal NULL. NULL in a WHERE
// predicate matches nothing and in ORDER BY / GROUP BY is a constant — the
// query stays valid SQL with attacker content fully neutralized.
func TestSafeJSONPathExpr_RejectsInjection(t *testing.T) {
	payloads := []string{
		"",
		"' OR '1'='1",
		"x') OR ('1'='1",
		"no_such_field' OR '1'='1",
		"id; DROP TABLE transfer-jobs--",
		"id') UNION SELECT password FROM users--",
		"a\"b",
		"foo bar",
		"$(id)",
		"`id`",
		"%s%s%s",
		"'); INSERT INTO x VALUES(1)--",
		"*/ --",
		"\\' OR 1=1--",
		"status\u0027 OR \u00271\u0027=\u00271",
		"путь",
		"$.secret",
		"line\nbreak",
		"tab\there",
	}
	for _, p := range payloads {
		got := SafeJSONPathExpr(p)
		assert.Equal(t, "NULL", got, "payload %q must degrade to NULL", p)
	}
}

// Whatever the input, the only quotes in a rendered expression are the
// '$...' path delimiters themselves: any input that carries a quote, a
// parenthesis, a semicolon or a comment marker is rejected outright, and
// inputs that pass the whitelist cannot contain them by construction.
func TestSafeJSONPathExpr_OutputNeverBreaksOut(t *testing.T) {
	nasty := []string{"'", "''", "a'b", "';--", "')", "((", ")?", "%'", "\"x"}
	for _, p := range nasty {
		got := SafeJSONPathExpr(p)
		assert.Equal(t, "NULL", got, "input %q carries breakout characters and must be rejected", p)
	}
	// And a passing identifier's output contains exactly the two delimiter
	// quotes of the path literal, nothing more.
	out := SafeJSONPathExpr("ok_field")
	assert.Equal(t, 2, strings.Count(out, "'"), "only the path delimiters may be quotes: %s", out)
}

func TestSafeSortDirection(t *testing.T) {
	assert.Equal(t, "ASC", SafeSortDirection("asc"))
	assert.Equal(t, "ASC", SafeSortDirection("ASC"))
	assert.Equal(t, "ASC", SafeSortDirection(""))
	assert.Equal(t, "ASC", SafeSortDirection("  asc  "))
	assert.Equal(t, "DESC", SafeSortDirection("desc"))
	assert.Equal(t, "DESC", SafeSortDirection(" DESC "))

	// Anything that is not a bare direction degrades to ASC instead of
	// being spliced into the ORDER BY fragment.
	hostile := []string{
		"DESC; DROP TABLE x--",
		"DESC, (SELECT password FROM users)",
		"desc'",
		"anything else",
		"\u0000",
	}
	for _, h := range hostile {
		got := SafeSortDirection(h)
		assert.True(t, got == "ASC" || got == "DESC", "direction for %q must collapse to ASC/DESC, got %q", h, got)
		assert.False(t, strings.ContainsAny(got, ";'(),"), "direction must stay a bare token, got %q", got)
	}
}

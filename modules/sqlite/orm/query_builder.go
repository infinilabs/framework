// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the License, or (at your option) any later version.
// See the GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package orm

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/orm"
)

// FieldResolver maps a dotted JSON path to its query-side plan: the SQL
// comparison expression, the integer-epoch shadow expression for date
// fields (range predicates on it become integer comparisons — index- and
// covering-friendly, no per-row TEXT parse), and the FTS target when the
// path is a text field synced into an FTS5 table. A nil resolver (or nil
// plan parts) falls back to json_extract for everything.
type FieldResolver func(path string) (expr, epochExpr string, fts *FTSPlan)

// FTSPlan identifies the FTS5 table/column backing a text field.
type FTSPlan struct {
	Table  string
	Column string
}

// BuildWhereClause translates an orm.QueryBuilder into a SQL WHERE clause
// string and corresponding parameter arguments. Returns empty string if no
// conditions exist. The resolver (may be nil) decides per path whether the
// comparison hits a promoted generated column or a json_extract fallback.
func BuildWhereClause(qb *orm.QueryBuilder, resolve FieldResolver) (string, []interface{}) {
	if qb == nil {
		return "", nil
	}

	root := qb.Root()
	if root == nil {
		return "", nil
	}

	where, args := clauseToSQL(root, resolve)
	return where, args
}

// ExprFor resolves a JSON path to its comparison expression via the
// resolver (generated column when promoted, json_extract otherwise).
func ExprFor(resolve FieldResolver, path string) string {
	if resolve == nil {
		return fmt.Sprintf("json_extract(raw, '$.%s')", path)
	}
	expr, _, _ := resolve(path)
	return expr
}

// RangeExprFor picks the comparison expression for a range predicate: the
// epoch shadow when one exists (the value is rewritten to integer epoch
// seconds by the caller), else the regular expression.
func RangeExprFor(resolve FieldResolver, path string) (expr string, epoch bool) {
	if resolve == nil {
		return fmt.Sprintf("json_extract(raw, '$.%s')", path), false
	}
	plain, epochExpr, _ := resolve(path)
	if epochExpr != "" {
		return epochExpr, true
	}
	return plain, false
}

// ToEpochSeconds converts a range value to integer epoch seconds when
// possible (RFC3339 strings, time.Time, numeric epochs).
func ToEpochSeconds(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.Unix(), true
	case string:
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts.Unix(), true
		}
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	}
	return 0, false
}

func exprFor(resolve FieldResolver, path string) string {
	return ExprFor(resolve, path)
}

// rangeFor builds a range predicate, rewriting date fields with an epoch
// shadow to integer comparisons (the constant is parsed once, here).
func rangeFor(resolve FieldResolver, path string, op string, value interface{}) (string, []interface{}) {
	expr, isEpoch := RangeExprFor(resolve, path)
	if isEpoch {
		if secs, ok := ToEpochSeconds(value); ok {
			return fmt.Sprintf("%s %s ?", expr, op), []interface{}{secs}
		}
	}
	return fmt.Sprintf("%s %s ?", expr, op), []interface{}{value}
}

// unsupportedOnce deduplicates warnings for operators sqlite cannot honor;
// per-query spam would hide the first occurrence in logs.
var unsupportedOnce sync.Map

func warnUnsupported(op orm.QueryType) {
	if _, loaded := unsupportedOnce.LoadOrStore(op, true); !loaded {
		log.Warnf("sqlite orm: %s queries are not supported on this backend; "+
			"the clause matches no documents (grace period: warning only, will become an error in a future release)", op)
	}
}

// clauseToSQL recursively translates a Clause tree into a SQL WHERE expression.
func clauseToSQL(clause *orm.Clause, resolve FieldResolver) (string, []interface{}) {
	if clause == nil {
		return "", nil
	}

	// Leaf node
	if clause.IsLeaf() {
		return leafToSQL(clause, resolve)
	}

	var parts []string
	var allArgs []interface{}

	// filter and must are combined with AND
	for _, sub := range clause.FilterClauses {
		sql, args := clauseToSQL(sub, resolve)
		if sql != "" {
			parts = append(parts, sql)
			allArgs = append(allArgs, args...)
		}
	}

	for _, sub := range clause.MustClauses {
		sql, args := clauseToSQL(sub, resolve)
		if sql != "" {
			parts = append(parts, sql)
			allArgs = append(allArgs, args...)
		}
	}

	// must_not is combined with AND NOT
	for _, sub := range clause.MustNotClauses {
		sql, args := clauseToSQL(sub, resolve)
		if sql != "" {
			parts = append(parts, fmt.Sprintf("NOT (%s)", sql))
			allArgs = append(allArgs, args...)
		}
	}

	// should clauses: respect minimum_should_match semantics.
	//
	// In Elasticsearch:
	//   - If minimum_should_match >= 1: at least that many should clauses must match
	//   - If no must/filter clauses exist: at least 1 should clause must match by default
	//   - If must/filter exist and minimum_should_match is unset: should is optional (scoring only)
	//
	// For SQLite (no scoring), we handle two key patterns:
	//   - minimum_should_match=1 → OR join (at least one must match)
	//   - Single should clause + min_should_match=1 → mandatory (equivalent to must)
	if len(clause.ShouldClauses) > 0 {
		minShouldMatch := 0
		if clause.Parameters != nil {
			minShouldMatch, _ = clause.Parameters.GetInt("minimum_should_match", 0)
		}

		// Determine if should clauses must participate in filtering.
		// When there are no must/filter clauses, at least one should must match
		// (Elasticsearch default). When minimum_should_match >= 1, it's explicit.
		hasMustOrFilter := len(clause.FilterClauses) > 0 || len(clause.MustClauses) > 0
		shouldRequired := minShouldMatch >= 1 || !hasMustOrFilter

		if shouldRequired {
			var shouldParts []string
			for _, sub := range clause.ShouldClauses {
				sql, args := clauseToSQL(sub, resolve)
				if sql != "" {
					shouldParts = append(shouldParts, sql)
					allArgs = append(allArgs, args...)
				}
			}
			if len(shouldParts) == 1 {
				// Single should clause with minimum_should_match >= 1:
				// the clause is mandatory, add directly without OR wrapping
				parts = append(parts, shouldParts[0])
			} else if len(shouldParts) > 1 {
				// Multiple should clauses: join with OR (at least one must match)
				shouldExpr := "(" + strings.Join(shouldParts, " OR ") + ")"
				parts = append(parts, shouldExpr)
			}
		}
		// When shouldRequired is false (minimum_should_match=0 with must/filter present),
		// should clauses are optional (scoring-only in ES) and safely skipped in SQLite
	}

	if len(parts) == 0 {
		return "", nil
	}
	if len(parts) == 1 {
		return parts[0], allArgs
	}
	return "(" + strings.Join(parts, " AND ") + ")", allArgs
}

// ftsMatchSQL builds a rowid subquery against the FTS5 table. Terms are
// quoted so user input can't inject FTS query syntax. A phrase query keeps
// its words in order inside one quoted string; a plain match joins its
// words with OR — matching ES's analyzed match semantics (any term).
func ftsMatchSQL(plan *FTSPlan, phrase bool, value interface{}) (string, []interface{}) {
	raw := fmt.Sprintf("%v", value)
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	term := quote(raw)
	if !phrase {
		if words := strings.Fields(raw); len(words) > 1 {
			quoted := make([]string, len(words))
			for i, w := range words {
				quoted[i] = quote(w)
			}
			term = strings.Join(quoted, " OR ")
		}
	}
	return fmt.Sprintf("rowid IN (SELECT rowid FROM [%s] WHERE [%s] MATCH ?)", plan.Table, plan.Table), []interface{}{term}
}

// leafToSQL converts a single leaf Clause to a SQL fragment.
func leafToSQL(clause *orm.Clause, resolve FieldResolver) (string, []interface{}) {
	field := clause.Field
	value := clause.Value

	var fts *FTSPlan
	if resolve != nil {
		_, _, fts = resolve(field)
	}

	switch clause.Operator {
	case orm.QuerySemantic, orm.QueryHybrid, orm.QueryNested:
		// These cannot be approximated on sqlite (vector search, nested docs).
		// Previously they silently compiled to a never-matching equality; now
		// the same empty result carries a one-time warning.
		warnUnsupported(clause.Operator)
		return "1 = 0", nil

	case orm.QueryMatch:
		if fts != nil {
			return ftsMatchSQL(fts, false, value)
		}
		return fmt.Sprintf("%s = ?", exprFor(resolve, field)), []interface{}{value}

	case orm.QueryTerm:
		return fmt.Sprintf("%s = ?", exprFor(resolve, field)), []interface{}{value}

	case orm.QueryMultiMatch:
		// field = "title,category" → split into multiple fields, match with OR.
		// FTS-backed fields use MATCH; others fall back to LIKE containment.
		fields := strings.Split(field, ",")
		var parts []string
		var args []interface{}
		for _, f := range fields {
			f = strings.TrimSpace(f)
			var fPlan *FTSPlan
			if resolve != nil {
				_, _, fPlan = resolve(f)
			}
			if fPlan != nil {
				sql, a := ftsMatchSQL(fPlan, false, value)
				parts = append(parts, sql)
				args = append(args, a...)
				continue
			}
			parts = append(parts, fmt.Sprintf("%s LIKE ?", exprFor(resolve, f)))
			args = append(args, fmt.Sprintf("%%%v%%", value))
		}
		return "(" + strings.Join(parts, " OR ") + ")", args

	case orm.QueryTerms, orm.QueryIn:
		return termsToSQL(exprFor(resolve, field), value)

	case orm.QueryNotIn:
		sql, args := termsToSQL(exprFor(resolve, field), value)
		if sql != "" {
			return "NOT " + sql, args
		}
		return "", nil

	case orm.QueryPrefix:
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{fmt.Sprintf("%v%%", value)}

	case orm.QueryWildcard:
		val := strings.ReplaceAll(fmt.Sprintf("%v", value), "*", "%")
		val = strings.ReplaceAll(val, "?", "_")
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{val}

	case orm.QueryRegexp:
		// SQLite doesn't have native regexp by default; fallback to LIKE
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryExists:
		return fmt.Sprintf("%s IS NOT NULL", exprFor(resolve, field)), nil

	case orm.QueryFuzzy:
		// Fuzzy search approximated with LIKE (fuzziness distance not honored)
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryMatchPhrase:
		if fts != nil {
			return ftsMatchSQL(fts, true, value)
		}
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryQueryString:
		if fts != nil {
			return ftsMatchSQL(fts, true, value)
		}
		return fmt.Sprintf("%s LIKE ?", exprFor(resolve, field)), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryRangeGte:
		return rangeFor(resolve, field, ">=", value)

	case orm.QueryRangeLte:
		return rangeFor(resolve, field, "<=", value)

	case orm.QueryRangeGt:
		return rangeFor(resolve, field, ">", value)

	case orm.QueryRangeLt:
		return rangeFor(resolve, field, "<", value)

	default:
		// Fallback: treat as equals
		return fmt.Sprintf("%s = ?", exprFor(resolve, field)), []interface{}{value}
	}
}

// termsToSQL builds an IN clause for terms/in queries.
func termsToSQL(jsonPath string, value interface{}) (string, []interface{}) {
	var args []interface{}
	var placeholders []string

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			placeholders = append(placeholders, "?")
			args = append(args, item)
		}
	case []string:
		for _, item := range v {
			placeholders = append(placeholders, "?")
			args = append(args, item)
		}
	default:
		return fmt.Sprintf("%s = ?", jsonPath), []interface{}{value}
	}

	if len(placeholders) == 0 {
		return "", nil
	}
	return fmt.Sprintf("%s IN (%s)", jsonPath, strings.Join(placeholders, ",")), args
}

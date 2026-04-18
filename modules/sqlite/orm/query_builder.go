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
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package orm

import (
	"fmt"
	"strings"

	"infini.sh/framework/core/orm"
)

// BuildWhereClause translates an orm.QueryBuilder into a SQL WHERE clause string
// and corresponding parameter arguments. Returns empty string if no conditions exist.
func BuildWhereClause(qb *orm.QueryBuilder) (string, []interface{}) {
	if qb == nil {
		return "", nil
	}

	root := qb.Root()
	if root == nil {
		return "", nil
	}

	where, args := clauseToSQL(root)
	return where, args
}

// clauseToSQL recursively translates a Clause tree into SQL WHERE expression.
func clauseToSQL(clause *orm.Clause) (string, []interface{}) {
	if clause == nil {
		return "", nil
	}

	// Leaf node
	if clause.IsLeaf() {
		return leafToSQL(clause)
	}

	var parts []string
	var allArgs []interface{}

	// filter and must are combined with AND
	for _, sub := range clause.FilterClauses {
		sql, args := clauseToSQL(sub)
		if sql != "" {
			parts = append(parts, sql)
			allArgs = append(allArgs, args...)
		}
	}

	for _, sub := range clause.MustClauses {
		sql, args := clauseToSQL(sub)
		if sql != "" {
			parts = append(parts, sql)
			allArgs = append(allArgs, args...)
		}
	}

	// must_not is combined with AND NOT
	for _, sub := range clause.MustNotClauses {
		sql, args := clauseToSQL(sub)
		if sql != "" {
			parts = append(parts, fmt.Sprintf("NOT (%s)", sql))
			allArgs = append(allArgs, args...)
		}
	}

	// should is combined with OR
	if len(clause.ShouldClauses) > 0 {
		var shouldParts []string
		for _, sub := range clause.ShouldClauses {
			sql, args := clauseToSQL(sub)
			if sql != "" {
				shouldParts = append(shouldParts, sql)
				allArgs = append(allArgs, args...)
			}
		}
		if len(shouldParts) > 0 {
			shouldExpr := "(" + strings.Join(shouldParts, " OR ") + ")"
			parts = append(parts, shouldExpr)
		}
	}

	if len(parts) == 0 {
		return "", nil
	}
	if len(parts) == 1 {
		return parts[0], allArgs
	}
	return "(" + strings.Join(parts, " AND ") + ")", allArgs
}

// leafToSQL converts a single leaf Clause to a SQL fragment.
func leafToSQL(clause *orm.Clause) (string, []interface{}) {
	field := clause.Field
	value := clause.Value

	jsonPath := fmt.Sprintf("json_extract(raw, '$.%s')", field)

	switch clause.Operator {
	case orm.QueryMatch, orm.QueryTerm:
		return fmt.Sprintf("%s = ?", jsonPath), []interface{}{value}

	case orm.QueryMultiMatch:
		// field = "title,category" → split into multiple fields, match with OR
		fields := strings.Split(field, ",")
		var parts []string
		var args []interface{}
		for _, f := range fields {
			f = strings.TrimSpace(f)
			jp := fmt.Sprintf("json_extract(raw, '$.%s')", f)
			parts = append(parts, fmt.Sprintf("%s LIKE ?", jp))
			args = append(args, fmt.Sprintf("%%%v%%", value))
		}
		return "(" + strings.Join(parts, " OR ") + ")", args

	case orm.QueryTerms, orm.QueryIn:
		return termsToSQL(jsonPath, value)

	case orm.QueryNotIn:
		sql, args := termsToSQL(jsonPath, value)
		if sql != "" {
			return "NOT " + sql, args
		}
		return "", nil

	case orm.QueryPrefix:
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{fmt.Sprintf("%v%%", value)}

	case orm.QueryWildcard:
		val := strings.ReplaceAll(fmt.Sprintf("%v", value), "*", "%")
		val = strings.ReplaceAll(val, "?", "_")
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{val}

	case orm.QueryRegexp:
		// SQLite doesn't have native regexp by default; fallback to LIKE
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryExists:
		return fmt.Sprintf("%s IS NOT NULL", jsonPath), nil

	case orm.QueryFuzzy:
		// Fuzzy search approximated with LIKE
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryMatchPhrase:
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryQueryString:
		return fmt.Sprintf("%s LIKE ?", jsonPath), []interface{}{fmt.Sprintf("%%%v%%", value)}

	case orm.QueryRangeGte:
		return fmt.Sprintf("%s >= ?", jsonPath), []interface{}{value}

	case orm.QueryRangeLte:
		return fmt.Sprintf("%s <= ?", jsonPath), []interface{}{value}

	case orm.QueryRangeGt:
		return fmt.Sprintf("%s > ?", jsonPath), []interface{}{value}

	case orm.QueryRangeLt:
		return fmt.Sprintf("%s < ?", jsonPath), []interface{}{value}

	default:
		// Fallback: treat as equals
		return fmt.Sprintf("%s = ?", jsonPath), []interface{}{value}
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

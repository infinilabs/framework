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
	"infini.sh/framework/core/param"
	"strconv"
	"strings"
)

type QueryType string

const (
	QueryMatch       QueryType = "match"
	QueryMultiMatch  QueryType = "multi_match"
	QueryTerm        QueryType = "term"
	QueryTerms       QueryType = "terms"
	QueryPrefix      QueryType = "prefix"
	QueryWildcard    QueryType = "wildcard"
	QueryRegexp      QueryType = "regexp"
	QueryFuzzy       QueryType = "fuzzy"
	QueryExists      QueryType = "exists"
	QueryRangeGte    QueryType = "range_gte"
	QueryRangeLte    QueryType = "range_lte"
	QueryRangeGt     QueryType = "range_gt"
	QueryRangeLt     QueryType = "range_lt"
	QueryIn          QueryType = "in"
	QueryNotIn       QueryType = "not_in"
	QueryMatchPhrase QueryType = "match_phrase"
)

type Clause struct {
	BoolType   BoolType
	SubClauses []*Clause  // nested conditions
	Query      *LeafQuery // only one if leaf
	Boost      int
}

func (c *Clause) SetBoost(boost int) *Clause {
	c.Boost = boost
	return c
}

func (c *Clause) SetQueryParameter(key param.ParaKey, val interface{}) *Clause {
	if c.Query.Parameters == nil {
		c.Query.Parameters = &param.Parameters{}
	}
	c.Query.Parameters.Set(key, val)
	return c
}

type LeafQuery struct {
	Field      string
	Operator   QueryType
	Value      interface{}
	Parameters *param.Parameters
}

type QueryBuilder struct {
	root *Clause
	sort []Sort
	from int
	size int
}

func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		root: &Clause{BoolType: Must},
	}
}

func (q *QueryBuilder) Must(clauses ...*Clause) *QueryBuilder {
	q.root.SubClauses = append(q.root.SubClauses, BoolQuery(Must, clauses...))
	return q
}

func (q *QueryBuilder) Should(clauses ...*Clause) *QueryBuilder {
	q.root.SubClauses = append(q.root.SubClauses, BoolQuery(Should, clauses...))
	return q
}

func (q *QueryBuilder) Not(clauses ...*Clause) *QueryBuilder {
	q.root.SubClauses = append(q.root.SubClauses, BoolQuery(MustNot, clauses...))
	return q
}

func newLeaf(field string, op QueryType, value interface{}) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: field, Operator: op, Value: value},
	}
}

func newAdvancedLeaf(field string, op QueryType, value interface{}, parameters *param.Parameters) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: field, Operator: op, Value: value, Parameters: parameters},
	}
}

func BoolQuery(boolType BoolType, clauses ...*Clause) *Clause {
	return &Clause{
		BoolType:   boolType,
		SubClauses: clauses,
	}
}

func MustQuery(clauses ...*Clause) *Clause {
	return BoolQuery(Must, clauses...)
}

func ShouldQuery(clauses ...*Clause) *Clause {
	return BoolQuery(Should, clauses...)
}

func MustNotQuery(clauses ...*Clause) *Clause {
	return BoolQuery(MustNot, clauses...)
}

func MatchQuery(field string, value interface{}) *Clause {
	return newLeaf(field, QueryMatch, value)
}

func MultiMatchQuery(fields []string, value interface{}) *Clause {
	return newLeaf(strings.Join(fields, ","), QueryMultiMatch, value)
}

func TermQuery(field string, value interface{}) *Clause {
	return newLeaf(field, QueryTerm, value)
}
func TermsQuery(field string, value []string) *Clause {
	return newLeaf(field, QueryTerms, value)
}

func PrefixQuery(field string, value interface{}) *Clause {
	return newLeaf(field, QueryPrefix, value)
}

func WildcardQuery(field string, value interface{}) *Clause {
	return newLeaf(field, QueryWildcard, value)
}

func RegexpQuery(field string, value interface{}) *Clause {
	return newLeaf(field, QueryRegexp, value)
}

const fuzzyFuzziness = "fuzziness"
const phraseSlop = "slop"

func FuzzyQuery(field string, value interface{}, fuzziness int) *Clause {
	param := param.Parameters{}
	param.Set(fuzzyFuzziness, fuzziness)
	return newAdvancedLeaf(field, QueryFuzzy, value, &param)
}

func ExistsQuery(field string) *Clause {
	return newLeaf(field, QueryExists, true)
}

func InQuery(field string, values []interface{}) *Clause {
	return newLeaf(field, QueryIn, values)
}

func NotInQuery(field string, values []interface{}) *Clause {
	return newLeaf(field, QueryNotIn, values)
}

func MatchPhraseQuery(field, value string, slop int) *Clause {
	param := param.Parameters{}
	param.Set(phraseSlop, slop)
	return newAdvancedLeaf(field, QueryMatchPhrase, value, &param)
}

type RangeQueryBuilder struct {
	field  string
	clause *Clause
}

func Range(field string) *RangeQueryBuilder {
	return &RangeQueryBuilder{
		field:  field,
		clause: &Clause{BoolType: Must},
	}
}

func (r *RangeQueryBuilder) Gte(v interface{}) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: r.field, Operator: QueryRangeGte, Value: v},
	}
}

func (r *RangeQueryBuilder) Lte(v interface{}) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: r.field, Operator: QueryRangeLte, Value: v},
	}
}
func (r *RangeQueryBuilder) Gt(v interface{}) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: r.field, Operator: QueryRangeGt, Value: v},
	}
}

func (r *RangeQueryBuilder) Lt(v interface{}) *Clause {
	return &Clause{
		Query: &LeafQuery{Field: r.field, Operator: QueryRangeLt, Value: v},
	}
}

func (q *QueryBuilder) From(from int) *QueryBuilder {
	q.from = from
	return q
}

func (q *QueryBuilder) Size(size int) *QueryBuilder {
	q.size = size
	return q
}

func (q *QueryBuilder) SortBy(sort ...Sort) *QueryBuilder {
	q.sort = sort
	return q
}

func (q *QueryBuilder) Root() *Clause {
	return q.root
}

func (q *QueryBuilder) Sorts() []Sort {
	return q.sort
}

func (q *QueryBuilder) FromVal() int {
	return q.from
}

func (q *QueryBuilder) SizeVal() int {
	return q.size
}

func (q *QueryBuilder) ToString() string {
	var b strings.Builder
	b.WriteString("QueryBuilder:\n")
	b.WriteString("From: ")
	b.WriteString(strconv.Itoa(q.from))
	b.WriteString(", Size: ")
	b.WriteString(strconv.Itoa(q.size))
	b.WriteString("\nSort: ")
	if len(q.sort) == 0 {
		b.WriteString("none\n")
	} else {
		for i, s := range q.sort {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(s.String())
		}
		b.WriteString("\n")
	}
	b.WriteString("Root:\n")
	b.WriteString(q.root.toStringIndented(1))
	return b.String()
}

func (c *Clause) toStringIndented(indentLevel int) string {
	indent := strings.Repeat("  ", indentLevel)
	var b strings.Builder

	// Print bool type for this clause
	if c.BoolType != "" {
		b.WriteString(indent)
		b.WriteString("BoolType: ")
		b.WriteString(c.BoolType.String())
		b.WriteString("\n")
	}

	// Print leaf query if present
	if c.Query != nil {
		b.WriteString(indent)
		b.WriteString(c.Query.toString())
		b.WriteString("\n")
	}

	// Print subclauses recursively
	for _, sub := range c.SubClauses {
		b.WriteString(sub.toStringIndented(indentLevel + 1))
	}

	return b.String()
}

func (q *LeafQuery) toString() string {
	return fmt.Sprintf("Query: {Field: %s, Operator: %s, Value: %v}", q.Field, q.Operator, q.Value)
}

func (s Sort) String() string {
	return fmt.Sprintf("%s:%s", s.Field, s.SortType)
}

// Assuming BoolType has a String() method, if not add this:
func (b BoolType) String() string {
	switch b {
	case Must:
		return "Must"
	case Should:
		return "Should"
	case MustNot:
		return "MustNot"
	default:
		return "UnknownBoolType"
	}
}

// Assuming SortType is a string alias, if not, add:
func (st SortType) String() string {
	return string(st)
}

func (b *QueryBuilder) Simplify() {
	b.root = simplifyClause(b.root)
}

func simplifyClause(c *Clause) *Clause {
	if c == nil {
		return nil
	}

	// Simplify subclauses first
	var newSubs []*Clause
	for _, sub := range c.SubClauses {
		s := simplifyClause(sub)
		// Only keep subclauses that are not nil and have meaning
		if s != nil && (s.Query != nil || len(s.SubClauses) > 0) {
			newSubs = append(newSubs, s)
		}
	}
	c.SubClauses = newSubs

	// Flatten single-child Bool clauses with same BoolType
	if len(c.SubClauses) == 1 {
		child := c.SubClauses[0]
		if child.BoolType == c.BoolType && child.Query == nil {
			return child
		}
	}

	return c
}

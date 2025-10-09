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
	"strconv"
	"strings"
	"unicode"

	"infini.sh/framework/core/param"
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
	QueryQueryString QueryType = "query_string"
)

type Clause struct {
	// Leaf clause fields (set if it's a leaf)
	Field      string
	Operator   QueryType
	Value      interface{}
	Parameters *param.Parameters

	// Compound clause fields (set if it's a bool)
	FilterClauses  []*Clause
	MustClauses    []*Clause
	ShouldClauses  []*Clause
	MustNotClauses []*Clause

	Boost float32
}

func (c *Clause) NumOfClauses() int {
	return (len(c.FilterClauses) + len(c.MustClauses) + len(c.MustNotClauses) + len(c.ShouldClauses))
}

func (c *Clause) IsLeaf() bool {
	return c.Operator != ""
}

func (c *Clause) SetBoost(boost float32) *Clause {
	c.Boost = boost
	return c
}

func (c *Clause) Parameter(key param.ParaKey, val interface{}) *Clause {
	if c.Parameters == nil {
		c.Parameters = &param.Parameters{}
	}
	c.Parameters.Set(key, val)
	return c
}

type QueryBuilder struct {
	root      *Clause
	sort      []Sort
	from      int
	size      int
	fuzziness int

	query    string
	includes []string
	excludes []string

	defaultOperator       string
	defaultQueryFields    []string
	defaultFilterFields   []string
	defaultFilterOperator string

	//indicate fuzziness query is built or not
	builtFuzziness bool

	// keep original filter clauses
	filters []*Clause

	allowRequestBodyBytes bool
	requestBodyBytes      []byte
	Aggs                  map[string]Aggregation
}

func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		root: &Clause{},
	}
}

func (q *QueryBuilder) SetRequestBodyBytes(bytes []byte) {
	q.requestBodyBytes = bytes
}

func (q *QueryBuilder) EnableBodyBytes() {
	q.allowRequestBodyBytes = true
}

func (q *QueryBuilder) DisableBodyBytes() {
	q.allowRequestBodyBytes = false
	q.requestBodyBytes = nil
}

// SetAggregations sets the aggregations for the query builder.
func (q *QueryBuilder) SetAggregations(aggs map[string]Aggregation) {
	q.Aggs = aggs
}

func (q *QueryBuilder) RequestBodyBytesVal() []byte {
	if !q.allowRequestBodyBytes {
		return nil
	}
	return q.requestBodyBytes
}

func (q *QueryBuilder) Parameter(key param.ParaKey, val interface{}) *QueryBuilder {
	if q.root == nil {
		q.root = &Clause{}
	}
	q.root.Parameter(key, val)
	return q
}

func (q *QueryBuilder) DefaultQueryField(field ...string) *QueryBuilder {
	q.defaultQueryFields = append(q.defaultQueryFields, field...)
	return q
}

func (q *QueryBuilder) DefaultQueryFieldsVal() []string {
	return q.defaultQueryFields
}

func (q *QueryBuilder) DefaultFilterFieldsVal() []string {
	return q.defaultFilterFields
}

func (q *QueryBuilder) DefaultOperator(op string) *QueryBuilder {
	q.defaultOperator = op
	return q
}

func (q *QueryBuilder) DefaultOperatorVal() string {
	return q.defaultOperator
}

func (q *QueryBuilder) DefaultFilterField(field ...string) *QueryBuilder {
	q.defaultFilterFields = append(q.defaultFilterFields, field...)
	return q
}

func (q *QueryBuilder) Include(field ...string) *QueryBuilder {
	q.includes = append(q.includes, field...)
	return q
}

func (q *QueryBuilder) Fuzziness(fuzziness int) *QueryBuilder {
	q.fuzziness = fuzziness
	return q
}

func (q *QueryBuilder) FuzzinessVal() int {
	return q.fuzziness
}

func (q *QueryBuilder) IncludesVal() []string {
	return q.includes
}

func (q *QueryBuilder) Exclude(field ...string) *QueryBuilder {
	q.excludes = append(q.excludes, field...)
	return q
}

func (q *QueryBuilder) ExcludesVal() []string {
	return q.excludes
}

func (q *QueryBuilder) Filter(filter ...*Clause) *QueryBuilder {
	q.root.FilterClauses = append(q.root.FilterClauses, filter...)
	return q
}

func (q *QueryBuilder) Must(clauses ...*Clause) *QueryBuilder {
	q.root.MustClauses = append(q.root.MustClauses, clauses...)
	return q
}

func (q *QueryBuilder) Should(clauses ...*Clause) *QueryBuilder {
	q.root.ShouldClauses = append(q.root.ShouldClauses, clauses...)
	return q
}

func (q *QueryBuilder) Not(clauses ...*Clause) *QueryBuilder {
	q.root.MustNotClauses = append(q.root.MustNotClauses, clauses...)
	return q
}

func newLeaf(field string, op QueryType, value interface{}, params ...*param.Parameters) *Clause {
	var p *param.Parameters
	if len(params) > 0 {
		p = params[0]
	}
	return &Clause{
		Field:      field,
		Operator:   op,
		Value:      value,
		Parameters: p,
	}
}

func BoolQuery(boolType BoolType, clauses ...*Clause) *Clause {
	clause := &Clause{}
	if len(clauses) > 0 {
		switch boolType {
		case Filter:
			clause.FilterClauses = clauses
			break
		case Must:
			clause.MustClauses = clauses
			break
		case Should:
			clause.ShouldClauses = clauses
			break
		case MustNot:
			clause.MustNotClauses = clauses
			break
		}

	}
	return clause
}

func FilterQuery(clauses ...*Clause) *Clause {
	return &Clause{
		FilterClauses: clauses,
	}
}

func MustQuery(clauses ...*Clause) *Clause {
	return &Clause{
		MustClauses: clauses,
	}
}

func ShouldQuery(clauses ...*Clause) *Clause {
	return &Clause{
		ShouldClauses: clauses,
	}
}

func MustNotQuery(clauses ...*Clause) *Clause {
	return &Clause{
		MustNotClauses: clauses,
	}
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

// TermsQuery creates a terms query clause from a generic slice
func TermsQuery[T any](field string, value []T) *Clause {
	// Convert []T to []interface{}
	values := make([]interface{}, len(value))
	for i, v := range value {
		values[i] = v
	}
	return newLeaf(field, QueryTerms, values)
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
const queryStringDefaultOperator = "default_operator"

func FuzzyQuery(field string, value interface{}, fuzziness int) *Clause {
	param := param.Parameters{}
	param.Set(fuzzyFuzziness, fuzziness)
	return newLeaf(field, QueryFuzzy, value, &param)
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
	return newLeaf(field, QueryMatchPhrase, value, &param)
}

func QueryStringQuery(field string, value string, defaultOperator string) *Clause {
	param := param.Parameters{}
	if defaultOperator != "" {
		param.Set(queryStringDefaultOperator, defaultOperator)
	}
	return newLeaf(field, QueryQueryString, value, &param)
}

type RangeQueryBuilder struct {
	field string
}

func Range(field string) *RangeQueryBuilder {
	return &RangeQueryBuilder{
		field: field,
	}
}

func (r *RangeQueryBuilder) Gte(v interface{}) *Clause {
	return &Clause{
		Field: r.field, Operator: QueryRangeGte, Value: v,
	}
}

func (r *RangeQueryBuilder) Lte(v interface{}) *Clause {
	return &Clause{
		Field: r.field, Operator: QueryRangeLte, Value: v,
	}
}
func (r *RangeQueryBuilder) Gt(v interface{}) *Clause {
	return &Clause{
		Field: r.field, Operator: QueryRangeGt, Value: v,
	}
}

func (r *RangeQueryBuilder) Lt(v interface{}) *Clause {
	return &Clause{
		Field: r.field, Operator: QueryRangeLt, Value: v,
	}
}

func (q *QueryBuilder) From(from int) *QueryBuilder {
	q.from = from
	return q
}

func (q *QueryBuilder) Query(query string) *QueryBuilder {
	q.query = query
	return q
}

func (q *QueryBuilder) QueryVal() string {
	return q.query
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

	if len(q.includes) > 0 {
		b.WriteString(", Includes: ")
		b.WriteString(strings.Join(q.includes, ","))
	}

	if len(q.excludes) > 0 {
		b.WriteString(", Excludes: ")
		b.WriteString(strings.Join(q.excludes, ","))
	}

	if len(q.defaultQueryFields) > 0 {
		b.WriteString(", Default Operator: ")
		b.WriteString(q.defaultOperator)
	}
	if len(q.defaultQueryFields) > 0 {
		b.WriteString(", Default Fields: ")
		b.WriteString(strings.Join(q.defaultQueryFields, ","))
	}

	if len(q.defaultQueryFields) > 0 {
		b.WriteString(", Default QueryFields: ")
		b.WriteString(strings.Join(q.defaultQueryFields, ","))
	}
	if len(q.defaultFilterFields) > 0 {
		b.WriteString(", Default FilterFields: ")
		b.WriteString(strings.Join(q.defaultFilterFields, ","))
	}

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
	b.WriteString("Query:\n")
	b.WriteString(q.root.toStringIndented(1))

	return b.String()
}

func (c *Clause) toString() string {
	if !c.IsLeaf() {
		return ""
	}
	return fmt.Sprintf("{Field: %s, Operator: %s, Value: %v, Parameters: %v}", c.Field, c.Operator, c.Value, c.Parameters)
}

func (c *Clause) toStringIndented(indentLevel int) string {
	indent := strings.Repeat("  ", indentLevel)
	var b strings.Builder

	// Print leaf query if present
	if c.IsLeaf() {
		b.WriteString(indent)
		b.WriteString("Query: ")
		b.WriteString(c.toString())
		b.WriteString("\n")
	}

	// Print filter clauses
	if len(c.FilterClauses) > 0 {
		b.WriteString(indent)
		b.WriteString("Filter:\n")
		for _, sub := range c.FilterClauses {
			b.WriteString(sub.toStringIndented(indentLevel + 1))
		}
	}

	// Print must clauses
	if len(c.MustClauses) > 0 {
		b.WriteString(indent)
		b.WriteString("Must:\n")
		for _, sub := range c.MustClauses {
			b.WriteString(sub.toStringIndented(indentLevel + 1))
		}
	}

	// Print should clauses
	if len(c.ShouldClauses) > 0 {
		b.WriteString(indent)
		b.WriteString("Should:\n")
		for _, sub := range c.ShouldClauses {
			b.WriteString(sub.toStringIndented(indentLevel + 1))
		}
	}

	// Print must_not clauses
	if len(c.MustNotClauses) > 0 {
		b.WriteString(indent)
		b.WriteString("MustNot:\n")
		for _, sub := range c.MustNotClauses {
			b.WriteString(sub.toStringIndented(indentLevel + 1))
		}
	}

	return b.String()
}

func (s Sort) String() string {
	return fmt.Sprintf("%s:%s", s.Field, s.SortType)
}

// Assuming BoolType has a String() method, if not add this:
func (b BoolType) String() string {
	switch b {
	case Filter:
		return "Filter"
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

func (b *QueryBuilder) Build() {
	b.buildFuzzinessQuery()

	simp := simplifyClause(b.root)

	b.root = simp
}

func simplifyClause(c *Clause) *Clause {
	if c == nil {
		return nil
	}

	if c.IsLeaf() {
		return c
	}

	// Simplify all clause types
	c.FilterClauses = simplifyList(c.FilterClauses, Filter)
	c.MustClauses = simplifyList(c.MustClauses, Must)
	c.ShouldClauses = simplifyList(c.ShouldClauses, Should)
	c.MustNotClauses = simplifyList(c.MustNotClauses, MustNot)

	// If this is a leaf node, just return it
	if c.IsLeaf() {
		return c
	}

	// Optional: flatten this clause if it has only one type and one child
	if len(c.MustClauses) == 1 && len(c.ShouldClauses) == 0 && len(c.MustNotClauses) == 0 && len(c.FilterClauses) == 0 {
		return c.MustClauses[0]
	}
	if len(c.ShouldClauses) == 1 && len(c.MustClauses) == 0 && len(c.MustNotClauses) == 0 && len(c.FilterClauses) == 0 {
		return c.ShouldClauses[0]
	}

	return c
}

func simplifyList(list []*Clause, typ BoolType) []*Clause {
	var result []*Clause
	for _, sub := range list {
		simplified := simplifyClause(sub)
		if simplified == nil {
			continue
		}

		//skip empty leaf
		if simplified.IsLeaf() && simplified.Field == "" && simplified.Value == nil {
			continue
		}

		//skip empty boolean
		if !simplified.IsLeaf() && simplified.NumOfClauses() == 0 {
			continue
		}

		// Flatten same-type inner bool clauses
		if !simplified.IsLeaf() && len(getClausesByType(simplified, typ)) > 0 &&
			len(getClausesByType(simplified, Filter))+len(getClausesByType(simplified, Must))+len(getClausesByType(simplified, Should))+len(getClausesByType(simplified, MustNot)) == len(getClausesByType(simplified, typ)) {
			result = append(result, getClausesByType(simplified, typ)...)
		} else {
			result = append(result, simplified)
		}
	}
	return result
}

func getClausesByType(c *Clause, typ BoolType) []*Clause {
	switch typ {
	case Filter:
		return c.FilterClauses
	case Must:
		return c.MustClauses
	case Should:
		return c.ShouldClauses
	case MustNot:
		return c.MustNotClauses
	default:
		return nil
	}
}
func (q *QueryBuilder) buildFuzzinessQuery() {
	if q.builtFuzziness {
		return
	}
	q.builtFuzziness = true

	queryStr := q.QueryVal()
	if queryStr == "" {
		return
	}

	fuzzinessVal := q.FuzzinessVal()
	field, value := parseQuery(queryStr)

	// Case 1: Explicit field is provided (possibly with ^boost)
	if field != "" && value != "" {
		// Parse boost from field (e.g., "name^5")
		fieldBoost := float32(1.0)
		if strings.Contains(field, "^") {
			parts := strings.SplitN(field, "^", 2)
			field = parts[0]
			if boostParsed, err := strconv.ParseFloat(parts[1], 32); err == nil {
				fieldBoost = float32(boostParsed)
			}
		}

		// Helper to apply overall boost
		boost := func(base float32) float32 { return base * fieldBoost }

		switch fuzzinessVal {
		case 0, 1:
			q.Must(MatchQuery(field, value).SetBoost(boost(1)))
		case 2:
			q.Must(ShouldQuery(
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(2)),
			))
		case 3:
			q.Must(ShouldQuery(
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 0).SetBoost(boost(2)),
			))
		case 4:
			q.Must(ShouldQuery(
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 1).SetBoost(boost(2)),
				FuzzyQuery(field, value, 1).SetBoost(boost(1)),
			))
		case 5:
			q.Must(ShouldQuery(
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 2).SetBoost(boost(2)),
				FuzzyQuery(field, value, 2).SetBoost(boost(1)),
			))
		}
		return
	}

	// Case 2: No specific field, use default query fields
	if value == "" {
		value = queryStr
	}
	defaultFields := q.DefaultQueryFieldsVal()
	shouldClauses := make([]*Clause, 0, len(defaultFields)*4)

	for _, rawField := range defaultFields {
		// Support boost parsing on default fields too
		field := rawField
		fieldBoost := float32(1.0)
		if strings.Contains(rawField, "^") {
			parts := strings.SplitN(rawField, "^", 2)
			field = parts[0]
			if boostParsed, err := strconv.ParseFloat(parts[1], 32); err == nil {
				fieldBoost = float32(boostParsed)
			}
		}

		boost := func(base float32) float32 { return base * fieldBoost }

		switch fuzzinessVal {
		case 0, 1:
			shouldClauses = append(shouldClauses,
				MatchQuery(field, value).SetBoost(boost(1)),
			)
		case 2:
			shouldClauses = append(shouldClauses,
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(2)),
			)
		case 3:
			shouldClauses = append(shouldClauses,
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 0).SetBoost(boost(2)),
			)
		case 4:
			shouldClauses = append(shouldClauses,
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 1).SetBoost(boost(2)),
				FuzzyQuery(field, value, 1).SetBoost(boost(1)),
			)
		case 5:
			shouldClauses = append(shouldClauses,
				MatchQuery(field, value).SetBoost(boost(5)),
				PrefixQuery(field, value).SetBoost(boost(3)),
				MatchPhraseQuery(field, value, 2).SetBoost(boost(2)),
				FuzzyQuery(field, value, 2).SetBoost(boost(1)),
			)
		}
	}

	if len(shouldClauses) > 0 {
		if len(shouldClauses) == 1 {
			q.Must(shouldClauses[0])
		} else {
			q.Must(ShouldQuery(shouldClauses...))
		}
	}
}

// parseQuery attempts to extract field:value only if the field name is valid
func parseQuery(queryStr string) (field string, value string) {
	parts := strings.SplitN(queryStr, ":", 2)
	if len(parts) == 2 {
		fieldCandidate := strings.TrimSpace(parts[0])
		// Only treat as field:value if fieldCandidate is safe (ASCII only, no punctuations)
		if isValidFieldName(fieldCandidate) {
			return strings.TrimSpace(fieldCandidate), strings.TrimSpace(parts[1])
		}
	}
	return "", strings.TrimSpace(queryStr)
}

func isValidFieldName(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

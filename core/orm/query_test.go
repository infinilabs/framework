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
	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/util"
	"reflect"
	"testing"
)

func TestTermQuery(t *testing.T) {
	clause := TermQuery("status", "active")
	if !clause.IsLeaf() || clause.Operator != QueryTerm || clause.Field != "status" || clause.Value != "active" {
		t.Errorf("Invalid TermQuery clause: %+v", clause)
	}
}

func TestMatchQuery(t *testing.T) {
	clause := MatchQuery("title", "search")
	if !clause.IsLeaf() || clause.Operator != QueryMatch || clause.Field != "title" || clause.Value != "search" {
		t.Errorf("Invalid MatchQuery clause: %+v", clause)
	}
}

func TestMatchQueryWithParameter(t *testing.T) {
	clause := MatchQuery("title", "search").Parameter("analyzer", "whitespace")
	if !clause.IsLeaf() || clause.Operator != QueryMatch || clause.Field != "title" || clause.Value != "search" {
		t.Errorf("Invalid MatchQuery clause: %+v", clause)
	}
	v, ok := clause.Parameters.GetString("analyzer")
	assert.True(t, ok)
	assert.Equal(t, "whitespace", v)
}

func TestPrefixQuery(t *testing.T) {
	clause := PrefixQuery("name", "med")
	if !clause.IsLeaf() || clause.Operator != QueryPrefix || clause.Field != "name" || clause.Value != "med" {
		t.Errorf("Invalid PrefixQuery clause: %+v", clause)
	}
}

func TestBoolQueryWrapping(t *testing.T) {
	a := TermQuery("status", "active")
	b := TermQuery("category", "books")
	boolClause := BoolQuery(Must, a, b)

	if len(boolClause.MustClauses) != 2 {
		t.Errorf("Invalid BoolQuery: %+v", boolClause)
	}
}

func TestMustQuery(t *testing.T) {
	a := TermQuery("a", 1)
	b := TermQuery("b", 2)
	c := MustQuery(a, b)

	if len(c.MustClauses) != 2 {
		t.Errorf("MustQuery failed: %+v", c)
	}
}

func TestShouldQuery(t *testing.T) {
	a := MatchQuery("name", "foo")
	b := MatchQuery("desc", "bar")
	c := ShouldQuery(a, b)

	if len(c.ShouldClauses) != 2 {
		t.Errorf("ShouldQuery failed: %+v", c)
	}
}

func TestMustNotQuery(t *testing.T) {
	a := TermQuery("deleted", true)
	c := MustNotQuery(a)

	if len(c.MustNotClauses) != 1 {
		t.Errorf("MustNotQuery failed: %+v", c)
	}
}

func TestNestedBool(t *testing.T) {
	a := TermQuery("type", "doc")
	b := PrefixQuery("title", "go")
	nested := MustQuery(a, b)

	c := TermQuery("published", true)
	outer := ShouldQuery(nested, c)

	if len(outer.ShouldClauses) != 2 {
		t.Errorf("Invalid nested query structure: %+v", outer)
	}
}

func TestRangeQuery(t *testing.T) {
	rangeQuery := Range("age")
	gteClause := rangeQuery.Gte(18)
	lteClause := rangeQuery.Lte(30)

	if !gteClause.IsLeaf() || gteClause.Field != "age" || gteClause.Operator != QueryRangeGte || gteClause.Value != 18 {
		t.Errorf("Invalid Gte clause: %+v", gteClause)
	}
	if !gteClause.IsLeaf() || lteClause.Field != "age" || lteClause.Operator != QueryRangeLte || lteClause.Value != 30 {
		t.Errorf("Invalid Lte clause: %+v", lteClause)
	}
}

func TestQueryBuilder(t *testing.T) {
	q := NewQuery().
		Must(TermQuery("status", "active")).
		Should(MatchQuery("title", "golang")).
		Not(TermQuery("deleted", true)).
		From(10).
		Size(20).
		SortBy(Sort{Field: "created_at", SortType: DESC})

	if q.from != 10 || q.size != 20 {
		t.Errorf("From/Size values not set properly: from=%d, size=%d", q.from, q.size)
	}
	if q.Root().NumOfClauses() != 3 {
		t.Errorf("Expected 3 top-level clauses, got %d", q.root.NumOfClauses())
	}
	expectedSort := []Sort{{Field: "created_at", SortType: DESC}}
	if !reflect.DeepEqual(q.sort, expectedSort) {
		t.Errorf("Sort clause mismatch: %+v", q.sort)
	}
}

func TestWildcardQuery(t *testing.T) {
	q := WildcardQuery("user", "med*")
	assertLeaf(t, q, "user", QueryWildcard, "med*")
}

func TestRegexpQuery(t *testing.T) {
	q := RegexpQuery("path", "/docs/.*")
	assertLeaf(t, q, "path", QueryRegexp, "/docs/.*")
}

func TestFuzzyQuery(t *testing.T) {
	q := FuzzyQuery("name", "medcl", 0)
	assertLeaf(t, q, "name", QueryFuzzy, "medcl")
}

func TestExistsQuery(t *testing.T) {
	q := ExistsQuery("updated_at")
	assertLeaf(t, q, "updated_at", QueryExists, true)
}

func TestInQuery(t *testing.T) {
	values := []interface{}{"go", "rust"}
	q := InQuery("tag", values)
	assertLeaf(t, q, "tag", QueryIn, values)
}

func TestNotInQuery(t *testing.T) {
	values := []interface{}{"foo", "bar"}
	q := NotInQuery("tag", values)
	assertLeaf(t, q, "tag", QueryNotIn, values)
}

func TestRangeGtQuery(t *testing.T) {
	q := Range("score").Gt(90)
	assertLeaf(t, q, "score", QueryRangeGt, 90)
}

func TestRangeLtQuery(t *testing.T) {
	q := Range("score").Lt(10)
	assertLeaf(t, q, "score", QueryRangeLt, 10)
}

// Helper: check leaf structure
func assertLeaf(t *testing.T, clause *Clause, field string, op QueryType, value interface{}) {
	t.Helper()
	if !clause.IsLeaf() {
		t.Fatalf("expected leaf query, got nil")
	}
	if clause.Field != field {
		t.Errorf("expected field %q, got %q", field, clause.Field)
	}
	if clause.Operator != op {
		t.Errorf("expected operator %q, got %q", op, clause.Operator)
	}
	if !reflect.DeepEqual(clause.Value, value) {
		t.Errorf("expected value %+v, got %+v", value, clause.Value)
	}
}

func TestComplexQuery(t *testing.T) {
	NewQuery().Must(TermQuery("hello", "world"), BoolQuery(Must))
}

func TestSimplify_SingleRedundantBool(t *testing.T) {
	root := &Clause{
		MustClauses: []*Clause{
			&Clause{
				Field: "title", Operator: Match, Value: "golang",
			},
		},
	}

	builder := &QueryBuilder{root: root}
	builder.Build()

	simplified := builder.Root()
	if len(simplified.MustClauses) != 0 {
		t.Fatalf("Expected 0 clause, got %d", len(simplified.MustClauses))
	}
	if !simplified.IsLeaf() || simplified.Field != "title" {
		t.Errorf("Expected simplified clause to have field 'title', got %v", simplified.Field)
	}
}

func TestSimplify_MultipleNestedSameType(t *testing.T) {
	root := &Clause{
		MustClauses: []*Clause{
			TermQuery("a", "1"),
			TermQuery("b", "2"),
		},
	}

	builder := &QueryBuilder{root: root}
	builder.Build()

	simplified := builder.Root()
	if len(simplified.MustClauses) != 2 {
		t.Fatalf("Expected 2 flattened clauses, got %d", len(simplified.MustClauses))
	}
}

func TestSimplify_MustNotFlattened(t *testing.T) {
	root := &Clause{
		MustNotClauses: []*Clause{
			TermQuery("title", "bad"),
		},
	}

	builder := &QueryBuilder{root: root}
	builder.Build()

	println(builder.ToString())

	simplified := builder.Root()
	if len(simplified.MustNotClauses) != 1 {
		t.Errorf("Expected MustNot clause, got %v", len(simplified.MustNotClauses))
	}

	if simplified.MustNotClauses[0].Field != "title" || simplified.MustNotClauses[0].Value != "bad" {
		t.Errorf("Expected query on 'title' with value 'bad', got field: %s, value: %v", simplified.MustNotClauses[0].Field, simplified.MustNotClauses[0].Value)
	}
}

func TestSimplify_NoOpEmptyClause(t *testing.T) {
	root := &Clause{
		MustClauses: []*Clause{
			{}, // empty clause
			TermQuery("name", "test"),
		},
	}

	builder := &QueryBuilder{root: root}
	builder.Build()

	println(builder.ToString())

	simplified := builder.Root()
	if !simplified.IsLeaf() || len(simplified.MustClauses) != 0 {
		fmt.Println(util.MustToJSON(simplified.MustClauses))
		t.Fatalf("Expected 0 non-empty clause, got %d", len(simplified.MustClauses))
	}
	if simplified.Field != "name" {
		t.Errorf("Expected clause with field 'name', got %s", simplified.Field)
	}
}

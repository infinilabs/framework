package orm

import (
	"fmt"
	"infini.sh/framework/core/param"
	"net/http"
	"testing"
)

func TestParseQueryParamsToBuilder(t *testing.T) {
	rawQuery := "query=golang" +
		"&filter=type:doc" +
		"&filter=-title:go" +
		"&filter=published:true" +
		"&filter=age>=18" +
		"&filter=age<=30" +
		"&filter=deleted:false" +
		"&sort=created_at:desc,age:asc" +
		"&from=10&size=20"

	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()

	// Check top-level Must clauses count (should be 7: 1 query + 6 filters)
	if builder.Root().NumOfClauses() != 7 {
		t.Errorf("Expected 7 Must clauses, got %d", len(builder.Root().MustClauses))
	}

	// Check from/size
	if builder.FromVal() != 10 {
		t.Errorf("Expected from=10, got %d", builder.FromVal())
	}
	if builder.SizeVal() != 20 {
		t.Errorf("Expected size=20, got %d", builder.SizeVal())
	}

	// Check sorting
	if len(builder.Sorts()) != 2 {
		t.Errorf("Expected 2 sort fields, got %d", len(builder.Sorts()))
	}
	if builder.Sorts()[0].Field != "created_at" || builder.Sorts()[0].SortType != SortType("desc") {
		t.Errorf("Unexpected sort 0: %+v", builder.Sorts()[0])
	}
	if builder.Sorts()[1].Field != "age" || builder.Sorts()[1].SortType != SortType("asc") {
		t.Errorf("Unexpected sort 1: %+v", builder.Sorts()[1])
	}
}

func TestParseQueryParamsToBuilder1(t *testing.T) {
	rawQuery := "query=golang" +
		"&filter=type:doc" +
		"&filter=-title:go" +
		"&filter=published:true" +
		"&filter=age>=18" +
		"&filter=age<=30" +
		"&filter=deleted:false" +
		"&sort=created_at:desc,age:asc" +
		"&from=10&size=20"

	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()

	// Define expected flattened clauses
	type expectedClause struct {
		logicalType string // "must", "must_not", "should"
		field       string
		operator    QueryType
		value       interface{}
	}
	expected := []expectedClause{
		{"must", "type", QueryTerm, "doc"},
		{"must_not", "title", QueryTerm, "go"},
		{"must", "published", QueryTerm, true},
		{"must", "age", QueryRangeGte, 18},
		{"must", "age", QueryRangeLte, 30},
		{"must", "deleted", QueryTerm, false},
		{"must", "content", QueryMatch, "golang"},
	}

	// Recursively collect leaf clauses with logical type
	var flatClauses []struct {
		logicalType string
		clause      *Clause
	}
	var walk func(c *Clause, context string)
	walk = func(c *Clause, context string) {
		if c == nil {
			return
		}
		if c.Operator != "" {
			flatClauses = append(flatClauses, struct {
				logicalType string
				clause      *Clause
			}{context, c})
		}
		for _, sub := range c.MustClauses {
			walk(sub, "must")
		}
		for _, sub := range c.ShouldClauses {
			walk(sub, "should")
		}
		for _, sub := range c.MustNotClauses {
			walk(sub, "must_not")
		}
	}
	walk(root, "must")

	// Check count
	if len(flatClauses) != len(expected) {
		t.Fatalf("Expected %d clauses, got %d", len(expected), len(flatClauses))
	}

	// Check fields
	for i, got := range flatClauses {
		exp := expected[i]

		if got.logicalType != exp.logicalType {
			t.Errorf("Clause %d: expected logicalType %s, got %s", i, exp.logicalType, got.logicalType)
		}
		if got.clause.Field != exp.field {
			t.Errorf("Clause %d: expected field %s, got %s", i, exp.field, got.clause.Field)
		}
		if got.clause.Operator != exp.operator {
			t.Errorf("Clause %d: expected operator %s, got %s", i, exp.operator, got.clause.Operator)
		}
		if fmt.Sprint(got.clause.Value) != fmt.Sprint(exp.value) {
			t.Errorf("Clause %d: expected value %v, got %v", i, exp.value, got.clause.Value)
		}
	}

	// Check from/size
	if builder.FromVal() != 10 {
		t.Errorf("Expected from=10, got %d", builder.FromVal())
	}
	if builder.SizeVal() != 20 {
		t.Errorf("Expected size=20, got %d", builder.SizeVal())
	}

	// Check sort
	expectedSort := []Sort{
		{Field: "created_at", SortType: "desc"},
		{Field: "age", SortType: "asc"},
	}
	actualSorts := builder.Sorts()
	if len(actualSorts) != len(expectedSort) {
		t.Fatalf("Expected %d sort fields, got %d", len(expectedSort), len(actualSorts))
	}
	for i := range expectedSort {
		if actualSorts[i] != expectedSort[i] {
			t.Errorf("Sort %d: expected %+v, got %+v", i, expectedSort[i], actualSorts[i])
		}
	}
}

func TestParseQueryWithFieldPrefix(t *testing.T) {
	rawQuery := "query=title:golang" // query on field "title"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()

	leafClause := builder.Root()
	if !leafClause.IsLeaf() {
		t.Fatal("Expected a query clause, got nil")
	}

	if len(leafClause.MustClauses) != 0 {
		t.Errorf("Expected Must clause, got %v", len(leafClause.MustClauses))
	}

	if leafClause.Field != "title" {
		t.Errorf("Expected field 'title', got '%s'", leafClause.Field)
	}
	if leafClause.Value != "golang" {
		t.Errorf("Expected value 'golang', got '%v'", leafClause.Value)
	}
}

func TestParseSortParams(t *testing.T) {
	rawQuery := "sort=_score,created:desc,title" // _score -> desc (default), created -> desc, title -> asc (default)
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()

	sorts := builder.Sorts()
	if len(sorts) != 3 {
		t.Fatalf("Expected 3 sort fields, got %d", len(sorts))
	}

	tests := []struct {
		field string
		order SortType
	}{
		{"_score", DESC},
		{"created", DESC},
		{"title", ASC},
	}

	for i, test := range tests {
		if sorts[i].Field != test.field {
			t.Errorf("Sort[%d]: Expected field '%s', got '%s'", i, test.field, sorts[i].Field)
		}
		if sorts[i].SortType != test.order {
			t.Errorf("Sort[%d]: Expected order '%s', got '%s'", i, test.order, sorts[i].SortType)
		}
	}
}

func TestParseQueryWithNotClause(t *testing.T) {
	rawQuery := "filter=-title:golang"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()

	root := builder.Root()
	if root.IsLeaf() {
		t.Fatalf("Expected not leaf clause, got %v, %v", root.IsLeaf(), root.Operator)
	}

	if len(root.MustNotClauses) != 1 {
		t.Fatalf("Expected 1 not clause, got %d", len(root.MustNotClauses))
	}

	if root.MustNotClauses[0].Field != "title" {
		t.Errorf("Expected field 'title', got '%s'", root.MustNotClauses[0].Field)
	}

	if root.MustNotClauses[0].Value != "golang" {
		t.Errorf("Expected value 'golang', got '%v'", root.MustNotClauses[0].Value)
	}
}

func TestParseQueryWithFuzziness5(t *testing.T) {
	rawQuery := "query=%E6%B5%8B%E8%AF%95%2C%3D%3A&filter=id%3A1%252C123%253A98%253D2%2C2&fuzziness=5"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()
	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	if len(root.MustClauses) != 2 {
		t.Fatalf("Expected 2 MustClauses, got %d", len(root.MustClauses))
	}

	// Validate first clause (filter)
	filterClause := root.MustClauses[0]
	if filterClause.Field != "id" || filterClause.Operator != QueryTerm {
		t.Errorf("Expected filter on id with term, got field=%s operator=%s", filterClause.Field, filterClause.Operator)
	}
	if val, ok := filterClause.Value.(string); !ok || val != "1,123:98=2,2" {
		t.Errorf("Expected id filter value '1,123:98=2,2', got %v", filterClause.Value)
	}

	// Validate second clause (query expansion)
	queryClause := root.MustClauses[1]
	if len(queryClause.ShouldClauses) != 4 {
		t.Fatalf("Expected 4 ShouldClauses for fuzziness=5, got %d", len(queryClause.ShouldClauses))
	}

	expected := []struct {
		operator    QueryType
		boost       float32
		expectParam string // "slop", "fuzziness", or "" for none
		expectValue interface{}
	}{
		{QueryMatch, 5, "", nil},
		{QueryPrefix, 3, "", nil},
		{QueryMatchPhrase, 2, "slop", 2},
		{QueryFuzzy, 1, "fuzziness", 2},
	}

	for i, clause := range queryClause.ShouldClauses {
		exp := expected[i]
		if clause.Field != "content" {
			t.Errorf("Clause %d: expected field 'content', got '%s'", i, clause.Field)
		}
		if clause.Operator != exp.operator {
			t.Errorf("Clause %d: expected operator '%s', got '%s'", i, exp.operator, clause.Operator)
		}
		if clause.Boost != exp.boost {
			t.Errorf("Clause %d: expected boost '%v', got '%v'", i, exp.boost, clause.Boost)
		}
		if clause.Value != "测试,=:" {
			t.Errorf("Clause %d: expected value '测试,=:', got '%v'", i, clause.Value)
		}
		if exp.expectParam != "" {
			if clause.Parameters == nil {
				t.Errorf("Clause %d: expected parameters, got nil", i)
				continue
			}
			val, exists := clause.Parameters.GetInt(param.ParaKey(exp.expectParam), 0)
			if !exists {
				t.Errorf("Clause %d: expected parameter '%s' not found", i, exp.expectParam)
			} else if val != exp.expectValue {
				t.Errorf("Clause %d: expected %s=%v, got %v", i, exp.expectParam, exp.expectValue, val)
			}
		}
	}
}

func TestParseQueryWithFuzziness3(t *testing.T) {
	rawQuery := "query=%E6%B5%8B%E8%AF%95%2C%3D%3A&filter=id%3A1%252C123%253A98%253D2%2C2&fuzziness=3"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()
	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}
	fmt.Println(builder.ToString())
	if len(root.MustClauses) != 2 {
		t.Fatalf("Expected 2 MustClauses, got %d", len(root.MustClauses))
	}

	// Validate filter clause
	filterClause := root.MustClauses[0]
	if filterClause.Field != "id" || filterClause.Operator != QueryTerm {
		t.Errorf("Expected filter on id with term, got field=%s operator=%s", filterClause.Field, filterClause.Operator)
	}
	if val, ok := filterClause.Value.(string); !ok || val != "1,123:98=2,2" {
		t.Errorf("Expected id filter value '1,123:98=2,2', got %v", filterClause.Value)
	}

	// Validate fuzziness-generated should clause
	queryClause := root.MustClauses[1]
	if len(queryClause.ShouldClauses) != 3 {
		t.Fatalf("Expected 3 ShouldClauses from fuzzy query, got %d", len(queryClause.ShouldClauses))
	}

	expectedClauses := []struct {
		field    string
		operator QueryType
		boost    float32
	}{
		{"content", QueryMatch, 5},
		{"content", QueryPrefix, 3},
		{"content", QueryMatchPhrase, 2},
	}

	for i, sub := range queryClause.ShouldClauses {
		expect := expectedClauses[i]
		if sub.Field != expect.field {
			t.Errorf("Clause %d: expected field '%s', got '%s'", i, expect.field, sub.Field)
		}
		if sub.Operator != expect.operator {
			t.Errorf("Clause %d: expected operator '%s', got '%s'", i, expect.operator, sub.Operator)
		}
		if sub.Boost != expect.boost {
			t.Errorf("Clause %d: expected boost '%v', got '%v'", i, expect.boost, sub.Boost)
		}
		if sub.Value != "测试,=:" {
			t.Errorf("Clause %d: expected value '测试,=:', got '%v'", i, sub.Value)
		}
	}
}

func TestParseQueryWithURLEncodedValue(t *testing.T) {
	rawQuery := "query=%E6%B5%8B%E8%AF%95%2C%3D%3A&filter=id%3A1%252C123%253A98%253D2%2C2&fuzziness=1"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()

	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	fmt.Println(root.toStringIndented(1))

	// Collect all leaf clauses with context
	var foundContent, foundID bool

	var walk func(c *Clause, context string)
	walk = func(c *Clause, context string) {
		if c == nil {
			return
		}
		if c.Operator != "" {
			// Check decoded query value correctness
			if c.Field == "content" && c.Value == "测试,=:" {
				foundContent = true
			}
			if c.Field == "id" && c.Value == "1,123:98=2,2" {
				foundID = true
			}
		}
		for _, sub := range c.MustClauses {
			walk(sub, "must")
		}
		for _, sub := range c.ShouldClauses {
			walk(sub, "should")
		}
		for _, sub := range c.MustNotClauses {
			walk(sub, "must_not")
		}
	}

	walk(root, "must")

	if !foundContent {
		t.Errorf("Expected query on field 'content' with value '测试,=:', but not found")
	}
	if !foundID {
		t.Errorf("Expected query on field 'id' with value '1,123:98=2,2', but not found")
	}
}

func TestParseQueryWithExistsFilter(t *testing.T) {
	rawQuery := "query=pizzaabc123&filter=exists(title)"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()
	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	var foundQuery, foundExists bool

	// Check all MustClauses
	for _, clause := range root.MustClauses {
		if clause.Operator == QueryMatch &&
			clause.Field == "content" &&
			clause.Value == "pizzaabc123" {
			foundQuery = true
		}
		if clause.Operator == QueryExists &&
			clause.Field == "title" {
			foundExists = true
		}
	}

	if !foundQuery {
		t.Errorf("Expected match query for 'pizzaabc123', but not found")
	}
	if !foundExists {
		t.Errorf("Expected exists query on 'title', but not found")
	}
}
func TestParseQueryWithNotExistsFilter(t *testing.T) {
	rawQuery := "query=pizzaabc123&filter=!exists(title)"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	builder.Build()
	root := builder.Root()
	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	fmt.Println(root.toStringIndented(3))

	var foundQuery, foundNotExists bool

	// Check MustClauses
	for _, clause := range root.MustClauses {
		if clause.Operator == QueryMatch &&
			clause.Field == "content" &&
			clause.Value == "pizzaabc123" {
			foundQuery = true
		}

		// Check if this must-clause contains MustNot with Exists
		for _, notClause := range clause.MustNotClauses {
			if notClause.Operator == QueryExists && notClause.Field == "title" {
				foundNotExists = true
			}
		}
	}

	if !foundQuery {
		t.Errorf("Expected match query for 'pizzaabc123', but not found")
	}
	if !foundNotExists {
		t.Errorf("Expected NOT exists query on 'title', but not found")
	}
}

package orm

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"infini.sh/framework/core/param"
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

	// Match clauses ignoring order
	matched := make([]bool, len(expected))

	for _, got := range flatClauses {
		for i, exp := range expected {
			if matched[i] {
				continue
			}
			if got.logicalType == exp.logicalType &&
				got.clause.Field == exp.field &&
				got.clause.Operator == exp.operator &&
				fmt.Sprint(got.clause.Value) == fmt.Sprint(exp.value) {
				matched[i] = true
				break
			}
		}
	}

	for i, ok := range matched {
		if !ok {
			t.Errorf("Expected clause not found: %+v", expected[i])
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

func TestParseAnyTermsQuery(t *testing.T) {
	rawQuery := "filter=!id:any(1,2,3,4)"
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
	fmt.Println(root.toStringIndented(3))
	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	if len(root.MustNotClauses) != 1 {
		t.Fatalf("Expected 1 must_not clause, got %d", len(root.MustNotClauses))
	}

	clause := root.MustNotClauses[0]
	if clause.Operator != QueryTerms {
		t.Fatalf("Expected terms query, got %v", clause.Operator)
	}

	if clause.Field != "id" {
		t.Errorf("Expected field 'id', got %s", clause.Field)
	}

	values, ok := clause.Value.([]interface{})
	if !ok {
		t.Fatalf("Expected clause.Value to be []interface{}, got %T", clause.Value)
	}

	expectedValues := []interface{}{1, 2, 3, 4}
	if len(values) != len(expectedValues) {
		t.Fatalf("Expected %d values, got %d", len(expectedValues), len(values))
	}

	for i, val := range values {
		if val != expectedValues[i] {
			t.Errorf("Expected value[%d] to be %v, got %v", i, expectedValues[i], val)
		}
	}
}

func TestParseQueryWithMergedTermFilters(t *testing.T) {
	rawQuery := "query=hello&filter=id:default&filter=id:ai_overview"
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

	var foundQuery, foundTerms bool

	for _, clause := range root.MustClauses {
		if clause.Operator == QueryMatch &&
			clause.Field == "content" &&
			clause.Value == "hello" {
			foundQuery = true
		}
		if clause.Operator == QueryTerms &&
			clause.Field == "id" {
			values, ok := clause.Value.([]interface{})
			if !ok {
				t.Errorf("Expected terms clause to contain a slice of values")
			}
			if len(values) != 2 || values[0] != "default" || values[1] != "ai_overview" {
				t.Errorf("Unexpected terms values: %v", values)
			}
			foundTerms = true
		}
	}

	if !foundQuery {
		t.Errorf("Expected match query, not found")
	}
	if !foundTerms {
		t.Errorf("Expected merged terms query for 'id', not found")
	}
}

func TestMergeTermQueries_SingleField(t *testing.T) {
	clauses := []*Clause{
		TermQuery("status", "active"),
		TermQuery("status", "pending"),
	}

	merged := mergeTermQueries(clauses)

	if len(merged) != 1 {
		t.Fatalf("Expected 1 merged clause, got %d", len(merged))
	}

	clause := merged[0]
	if clause.Operator != QueryTerms {
		t.Errorf("Expected QueryTerms, got %v", clause.Operator)
	}
	if clause.Field != "status" {
		t.Errorf("Expected field 'status', got %s", clause.Field)
	}
	values, ok := clause.Value.([]interface{})
	if !ok || len(values) != 2 || values[0] != "active" || values[1] != "pending" {
		t.Errorf("Unexpected values: %v", clause.Value)
	}
}

func TestMergeTermQueries_MultipleFields(t *testing.T) {
	clauses := []*Clause{
		TermQuery("status", "active"),
		TermQuery("type", "admin"),
		TermQuery("status", "pending"),
	}

	merged := mergeTermQueries(clauses)

	if len(merged) != 2 {
		t.Fatalf("Expected 2 merged clauses, got %d", len(merged))
	}

	var foundStatus, foundType bool

	for _, clause := range merged {
		switch clause.Field {
		case "status":
			if clause.Operator != QueryTerms {
				t.Errorf("Expected QueryTerms for 'status', got %v", clause.Operator)
			}
			foundStatus = true
		case "type":
			if clause.Operator != QueryTerm {
				t.Errorf("Expected QueryTerm for 'type', got %v", clause.Operator)
			}
			foundType = true
		default:
			t.Errorf("Unexpected field: %s", clause.Field)
		}
	}

	if !foundStatus || !foundType {
		t.Errorf("Missing expected merged clauses")
	}
}

func TestMergeTermQueries_WithNonTermQueries(t *testing.T) {
	rangeClause := &Clause{
		Field:    "age",
		Operator: QueryRangeGte,
		Value:    30,
	}
	existsClause := ExistsQuery("email")

	clauses := []*Clause{
		TermQuery("id", "123"),
		rangeClause,
		TermQuery("id", "456"),
		existsClause,
	}

	merged := mergeTermQueries(clauses)

	if len(merged) != 3 {
		t.Fatalf("Expected 3 clauses (1 merged + 2 untouched), got %d", len(merged))
	}

	var foundTerms, foundRange, foundExists bool

	for _, clause := range merged {
		switch clause.Operator {
		case QueryTerms:
			if clause.Field == "id" {
				values := clause.Value.([]interface{})
				if len(values) != 2 {
					t.Errorf("Expected 2 terms in merged clause, got %v", values)
				}
				foundTerms = true
			}
		case QueryRangeGte:
			if clause.Field == "age" {
				foundRange = true
			}
		case QueryExists:
			if clause.Field == "email" {
				foundExists = true
			}
		}
	}

	if !foundTerms || !foundRange || !foundExists {
		t.Errorf("Expected all clause types to be present after merge")
	}
}

func TestMergeTermQueries_NoMergingNeeded(t *testing.T) {
	clauses := []*Clause{
		Range("score").Gte(90),
		ExistsQuery("email"),
	}

	merged := mergeTermQueries(clauses)

	if len(merged) != 2 {
		t.Errorf("Expected 2 unchanged clauses, got %d", len(merged))
	}
}

func TestParseQueryWithMultipleFilterValues(t *testing.T) {
	rawQuery := "query=world&filter=tag:news&filter=tag:tech&filter=tag:ai"
	req, err := http.NewRequest("GET", "/search?"+rawQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	builder, err := NewQueryBuilderFromRequest(req, "content")
	if err != nil {
		t.Fatal(err)
	}

	// Example assertion: check if "tag" filters merged into a single clause
	if len(builder.filters) == 0 {
		t.Fatal("Expected at least one filter clause")
	}
	found := false
	values := []string{}
	for _, f := range builder.filters {
		if f.Field == "tag" {
			found = true
			// Ensure it's a slice of values
			value, ok := f.Value.(string)
			if !ok {
				t.Fatalf("Expected []string for tag filter, got %T", f.Value)
			}
			values = append(values, value)
		}
	}
	expected := []string{"news", "tech", "ai"}
	if !reflect.DeepEqual(values, expected) {
		t.Errorf("Expected values %v, got %v", expected, values)
	}

	if !found {
		t.Fatal("Expected tag filter to be present")
	}
}

// Test case for preserving '+' in filter values (e.g. timezone offsets)
//
// str:         "created>=2025-01-01T00:00:00+08:00"
// URL encoded: "created%3E%3D2025-01-01T00%3A00%3A00%2B08%3A00"
func TestParseQueryWithPlusSign(t *testing.T) {
	rawQuery := "filter=created>=2025-01-01T00:00:00%2B08:00"
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

	// Find the range query
	var found bool

	checkClause := func(clause *Clause) {
		if clause.Field == "created" && clause.Operator == QueryRangeGte {
			val, ok := clause.Value.(string)
			if !ok {
				t.Errorf("Expected string value, got %T", clause.Value)
				return
			}
			if val == "2025-01-01T00:00:00+08:00" {
				found = true
			} else {
				t.Errorf("Expected value '2025-01-01T00:00:00+08:00', got '%s'", val)
			}
		}
	}

	if root.IsLeaf() {
		checkClause(root)
	} else {
		for _, clause := range root.MustClauses {
			checkClause(clause)
		}
	}

	if !found {
		t.Error("Expected range query on 'created' with value '2025-01-01T00:00:00+08:00' not found")
	}
}

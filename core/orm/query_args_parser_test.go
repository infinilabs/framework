package orm

import (
	"fmt"
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

	builder.Simplify()
	fmt.Println(builder.ToString())

	// Check top-level Must clauses count (should be 7: 1 query + 6 filters)
	if len(builder.Root().SubClauses) != 7 {
		t.Errorf("Expected 7 Must clauses, got %d", len(builder.Root().SubClauses))
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

	fmt.Println(builder.ToString())

	if len(builder.Root().SubClauses) != 1 {
		t.Errorf("Expected 1 Must clause, got %d", len(builder.Root().SubClauses))
	}

	clause := builder.Root().SubClauses[0]
	leafClause := findFirstLeafClause(clause)
	if leafClause == nil || leafClause.Query == nil {
		t.Fatal("Expected a query clause, got nil")
	}

	if leafClause.Query.Field != "title" {
		t.Errorf("Expected field 'title', got '%s'", leafClause.Query.Field)
	}
	if leafClause.Query.Value != "golang" {
		t.Errorf("Expected value 'golang', got '%v'", leafClause.Query.Value)
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

	builder.Simplify()

	fmt.Println(builder.ToString())

	root := builder.Root()
	if len(root.SubClauses) != 1 {
		t.Fatalf("Expected 1 root clause, got %d", len(root.SubClauses))
	}

	notClause := root.SubClauses[0]
	if notClause.BoolType != MustNot {
		t.Fatalf("Expected clause type NOT, got %v", notClause.BoolType)
	}

	leaf := findFirstLeafClause(notClause)
	if leaf == nil || leaf.Query == nil {
		t.Fatal("Expected a query clause under NOT, got nil")
	}

	if leaf.Query.Field != "title" {
		t.Errorf("Expected field 'title', got '%s'", leaf.Query.Field)
	}
	if leaf.Query.Value != "golang" {
		t.Errorf("Expected value 'golang', got '%v'", leaf.Query.Value)
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

	builder.Simplify()
	root := builder.Root()

	if root == nil {
		t.Fatal("Expected root clause to be non-nil")
	}

	// Expecting two clauses: one for query and one for filter
	if len(root.SubClauses) != 2 {
		t.Fatalf("Expected 2 subclauses, got %d", len(root.SubClauses))
	}

	// Check full-text query (first clause, Should inside Must)
	queryClause := root.SubClauses[0]
	if queryClause.BoolType != Must {
		t.Fatalf("Expected first clause to be MUST, got %v", queryClause.BoolType)
	}
	if len(queryClause.SubClauses) != 1 {
		t.Fatalf("Expected 1 subclause under queryClause, got %d", len(queryClause.SubClauses))
	}
	shouldClause := queryClause.SubClauses[0]
	if shouldClause.BoolType != Should {
		t.Fatalf("Expected SHOULD clause under queryClause, got %v", shouldClause.BoolType)
	}

	foundContent := false
	for _, sub := range shouldClause.SubClauses {
		if sub.Query != nil && sub.Query.Field == "content" && sub.Query.Value == "测试,=:" {
			foundContent = true
			break
		}
	}
	if !foundContent {
		t.Errorf("Expected query on field 'content' with value '测试,=:', but not found")
	}

	// Check filter clause
	filterClause := root.SubClauses[1]
	if filterClause.BoolType != Must {
		t.Fatalf("Expected filter clause to be MUST, got %v", filterClause.BoolType)
	}

	if len(filterClause.SubClauses) != 1 {
		t.Fatalf("Expected 1 subclause in filter, got %d", len(filterClause.SubClauses))
	}
	filterLeaf := filterClause.SubClauses[0]
	if filterLeaf.Query == nil {
		t.Fatal("Expected filter clause to have a query")
	}
	if filterLeaf.Query.Field != "id" {
		t.Errorf("Expected filter field to be 'id', got '%s'", filterLeaf.Query.Field)
	}
	if filterLeaf.Query.Value != "1,123:98=2,2" {
		t.Errorf("Expected filter value '1,123:98=2,2', got '%v'", filterLeaf.Query.Value)
	}
}

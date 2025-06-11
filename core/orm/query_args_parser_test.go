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

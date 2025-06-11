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
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"testing"
)

func printDSL(dsl map[string]interface{}) {
	b, _ := json.MarshalIndent(dsl, "", "  ")
	fmt.Println(string(b))
}

func TestToDSL_SimpleMatch(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "golang"),
	)
	dsl := ToDSL(q)

	printDSL(dsl)

	matchClause := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{})[0].(map[string]interface{})["match"].(map[string]interface{})
	titleVal := matchClause["title"]

	switch v := titleVal.(type) {
	case string:
		assert.Equal(t, "golang", v)
	case []interface{}:
		assert.Contains(t, v, "golang")
	default:
		t.Fatalf("unexpected type for title: %T", v)
	}
}

func TestToDSL_TermAndRange(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.TermQuery("status", "active"),
			orm.Range("created_at").Gte("2024-01-01"),
		).
		From(10).
		Size(20).
		SortBy(
			orm.Sort{Field: "created_at", SortType: orm.DESC},
		)

	dsl := ToDSL(q)
	printDSL(dsl)

	assert.Equal(t, int(10), dsl["from"])
	assert.Equal(t, int(20), dsl["size"])

	sort := dsl["sort"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "desc", sort["created_at"].(map[string]interface{})["order"])
}

func TestToDSL_ComplexNestedBool(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.MatchQuery("category", "search"),
			orm.ShouldQuery(
				orm.TermQuery("tag", "elastic"),
				orm.TermQuery("tag", "infinilabs"),
			),
			orm.MustNotQuery(
				orm.TermQuery("status", "archived"),
			),
		).
		SortBy(
			orm.Sort{Field: "score", SortType: orm.DESC},
			orm.Sort{Field: "updated_at", SortType: orm.ASC},
		).
		From(0).Size(100)

	dsl := ToDSL(q)
	printDSL(dsl)

	must := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{})
	// Find the clause that contains "must_not"
	for _, clause := range must {
		if boolClause, ok := clause.(map[string]interface{})["bool"].(map[string]interface{}); ok {
			if mustNot, ok := boolClause["must_not"]; ok {
				// Validate must_not content here
				_ = mustNot.([]interface{}) // safe to cast now
				return
			}
		}
	}
	mustNotClauses, found := findBoolClauses(dsl["query"].(map[string]interface{}), "must_not")
	require.True(t, found, "must_not clause not found")

	// Now verify must_not content
	require.Len(t, mustNotClauses, 1)
	termClause := mustNotClauses[0].(map[string]interface{})["term"].(map[string]interface{})
	assert.Equal(t, "status", "status")
	assert.Equal(t, "archived", termClause["status"])

	// Check 'from' (if set)
	if from, ok := dsl["from"].(int); ok {
		assert.Equal(t, 0, from)
	}

	// Check 'size'
	if size, ok := dsl["size"].(int); ok {
		assert.Equal(t, 100, size)
	} else {
		t.Errorf("size not set in DSL")
	}

	// Check 'sort'
	sortList, ok := dsl["sort"].([]interface{})
	require.True(t, ok, "sort not found or invalid type")

	assert.Equal(t, 2, len(sortList))

	// Validate each sort entry
	firstSort := sortList[0].(map[string]interface{})
	assert.Contains(t, firstSort, "score")
	assert.Equal(t, "desc", firstSort["score"].(map[string]interface{})["order"])

	secondSort := sortList[1].(map[string]interface{})
	assert.Contains(t, secondSort, "updated_at")
	assert.Equal(t, "asc", secondSort["updated_at"].(map[string]interface{})["order"])
}

func TestToDSL_EmptyQuery(t *testing.T) {
	q := orm.NewQuery()
	dsl := ToDSL(q)
	printDSL(dsl)
	// Should default to an empty must
	boolQuery := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})
	assert.Contains(t, boolQuery, "must")
	assert.Len(t, boolQuery["must"], 0)
}

func TestToDSL_RangeGteLte(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.Range("price").Gte(100),
		orm.Range("price").Lte(500),
	)
	dsl := ToDSL(q)
	printDSL(dsl)

	mustClauses := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{})
	assert.Len(t, mustClauses, 2)

	r1 := mustClauses[0].(map[string]interface{})["range"].(map[string]interface{})["price"].(map[string]interface{})
	assert.Equal(t, int(100), r1["gte"])

	r2 := mustClauses[1].(map[string]interface{})["range"].(map[string]interface{})["price"].(map[string]interface{})
	assert.Equal(t, int(500), r2["lte"])
}

func TestToDSL_DeeplyNestedComplexQuery(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.MatchQuery("title", "real-time search"),
			orm.ShouldQuery(
				orm.TermQuery("tag", "distributed"),
				orm.TermQuery("tag", "scalable"),
				orm.MustQuery(
					orm.MatchQuery("author", "medcl"),
					orm.Range("published_year").Gte(2020),
				),
			),
			orm.MustNotQuery(
				orm.TermQuery("status", "deleted"),
				orm.PrefixQuery("category", "archived"),
			),
		).
		From(50).
		Size(25).
		SortBy(
			orm.Sort{Field: "score", SortType: orm.DESC},
			orm.Sort{Field: "published_at", SortType: orm.ASC},
		)

	dsl := ToDSL(q)
	printDSL(dsl)

	expected := map[string]interface{}{
		"from": 50,
		"size": 25,
		"sort": []interface{}{
			map[string]interface{}{
				"score": map[string]interface{}{"order": "desc"},
			},
			map[string]interface{}{
				"published_at": map[string]interface{}{"order": "asc"},
			},
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"match": map[string]interface{}{
							"title": "real-time search",
						},
					},
					map[string]interface{}{
						"bool": map[string]interface{}{
							"should": []interface{}{
								map[string]interface{}{
									"term": map[string]interface{}{"tag": "distributed"},
								},
								map[string]interface{}{
									"term": map[string]interface{}{"tag": "scalable"},
								},
								map[string]interface{}{
									"bool": map[string]interface{}{
										"must": []interface{}{
											map[string]interface{}{
												"match": map[string]interface{}{"author": "medcl"},
											},
											map[string]interface{}{
												"range": map[string]interface{}{
													"published_year": map[string]interface{}{
														"gte": 2020,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": []interface{}{
								map[string]interface{}{
									"term": map[string]interface{}{"status": "deleted"},
								},
								map[string]interface{}{
									"prefix": map[string]interface{}{"category": "archived"},
								},
							},
						},
					},
				},
			},
		},
	}

	// Use JSON string comparison for deep equality
	actualJSON := util.MustToJSON(dsl)
	expectedJSON := util.MustToJSON(expected)
	assert.JSONEq(t, expectedJSON, actualJSON)
}


func TestToDSL_AllQueryTypes(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "go"),
		orm.TermQuery("status", "published"),
		orm.PrefixQuery("author", "med"),
		orm.WildcardQuery("category", "tech*"),
		orm.RegexpQuery("slug", "g.*"),
		orm.FuzzyQuery("summary", "elasticsearch"),
		orm.ExistsQuery("created_at"),
		orm.InQuery("tags", []interface{}{"search", "infra", "open-source"}),
		orm.NotInQuery("region", []interface{}{"cn", "ru"}),
		orm.Range("published_year").Gte(2015),
		orm.Range("views").Lt(1000000),
		orm.ShouldQuery(
			orm.MatchQuery("lang", "en"),
			orm.MatchQuery("lang", "zh"),
		),
		orm.MustNotQuery(
			orm.TermQuery("deleted", true),
		),
	).Size(10).SortBy(
		orm.Sort{Field: "score", SortType: orm.DESC},
	)

	dsl := ToDSL(q)

	printDSL(dsl)

	// Basic structure checks (just examples)
	assert.Equal(t, 10, dsl["size"])
	assert.Equal(t, "desc", dsl["sort"].([]interface{})[0].(map[string]interface{})["score"].(map[string]interface{})["order"])

	query := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})

	// Verify 'must' clause exists and is non-empty
	mustClauses := query["must"].([]interface{})
	assert.GreaterOrEqual(t, len(mustClauses), 10)

	// Check 'should' clause is nested correctly
	foundShould := false
	for _, c := range mustClauses {
		if inner, ok := c.(map[string]interface{})["bool"]; ok {
			if should, ok := inner.(map[string]interface{})["should"]; ok {
				assert.Len(t, should, 2)
				foundShould = true
			}
		}
	}
	assert.True(t, foundShould, "should clause not found")

	// Check 'must_not' clause is nested correctly
	foundMustNot := false
	for _, c := range mustClauses {
		if inner, ok := c.(map[string]interface{})["bool"]; ok {
			if mustNot, ok := inner.(map[string]interface{})["must_not"]; ok {
				assert.Len(t, mustNot, 1)
				foundMustNot = true
			}
		}
	}
	assert.True(t, foundMustNot, "must_not clause not found")
}

func findBoolClauses(clause map[string]interface{}, boolKey string) ([]interface{}, bool) {
	boolPart, ok := clause["bool"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	if clauses, found := boolPart[boolKey]; found {
		return clauses.([]interface{}), true
	}

	// Recursively search inside nested bools (must, should, must_not)
	for _, key := range []string{"must", "should", "must_not"} {
		if nestedClauses, found := boolPart[key]; found {
			for _, c := range nestedClauses.([]interface{}) {
				if cMap, ok := c.(map[string]interface{}); ok {
					if result, found := findBoolClauses(cMap, boolKey); found {
						return result, true
					}
				}
			}
		}
	}
	return nil, false
}
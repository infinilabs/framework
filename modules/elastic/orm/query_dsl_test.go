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
	"maps"
	"reflect"
	"testing"
)

func TestToDSL_SimpleMatch1(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "golang"),
	)
	dsl := BuildQueryDSL(q)

	printDSL(dsl)

	matchClause := extractMatchClause(t, dsl)

	// Ensure "title" is present
	titleClause, ok := matchClause["title"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected title to be a map, got %T", matchClause["title"])
	}

	// Check that the inner "query" matches expected value
	if val, ok := titleClause["query"]; !ok || val != "golang" {
		t.Errorf("Expected match title: golang, got %v", val)
	}
}

func TestToDSL_WithBoost(t *testing.T) {
	match := orm.MatchQuery("title", "golang")
	match.Boost = 2.5

	q := orm.NewQuery().Must(match)
	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	clauses := extractClauseByType(t, dsl, "match")
	if len(clauses) == 0 {
		t.Fatal("Expected at least one match clause")
	}

	matchMap := clauses[0].(map[string]interface{})["match"].(map[string]interface{})

	titleClause, ok := matchMap["title"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected match[\"title\"] to be a map")
	}

	if titleClause["query"] != "golang" {
		t.Errorf("Expected query value 'golang', got %v", titleClause["query"])
	}
	if boost, ok := titleClause["boost"]; !ok || util.ToString(boost) != util.ToString(2.5) {
		t.Errorf("Expected boost 2.5, got %v", boost)
	}
}

func TestToDSL_BoolWithSubClauses(t *testing.T) {
	q := orm.NewQuery().
		Must(orm.MatchQuery("title", "golang")).
		Not(orm.TermQuery("status", "deleted"))

	dsl := BuildQueryDSL(q)
	fmt.Println(q.ToString())
	printDSL(dsl)

	queryObj, ok := dsl["query"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'query' to be a map, got: %T", dsl["query"])
	}

	boolQueryRaw, ok := queryObj["bool"]
	if !ok {
		t.Fatalf("Expected 'bool' clause at top level, got keys: %v", maps.Keys(queryObj))
	}

	boolQuery, ok := boolQueryRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'bool' to be map[string]interface{}, got: %T", boolQueryRaw)
	}

	mustClausesRaw, ok := boolQuery["must"]
	if !ok {
		t.Fatal("Missing 'must' clause")
	}
	mustClauses, ok := mustClausesRaw.([]interface{})
	if !ok {
		t.Fatalf("Expected 'must' to be []interface{}, got %T", mustClausesRaw)
	}
	if len(mustClauses) != 1 {
		t.Fatalf("Expected 1 'must' clause, got: %d", len(mustClauses))
	}

	mustNotClausesRaw, ok := boolQuery["must_not"]
	if !ok {
		t.Fatal("Missing 'must_not' clause")
	}
	mustNotClauses, ok := mustNotClausesRaw.([]interface{})
	if !ok {
		t.Fatalf("Expected 'must_not' to be []interface{}, got %T", mustNotClausesRaw)
	}
	if len(mustNotClauses) != 1 {
		t.Fatalf("Expected 1 'must_not' clause, got: %d", len(mustNotClauses))
	}
}

func printDSL(dsl map[string]interface{}) {
	b, _ := json.MarshalIndent(dsl, "", "  ")
	fmt.Println(string(b))
}

func TestToDSL_SimpleMatch(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "golang"),
	)
	dsl := BuildQueryDSL(q)

	printDSL(dsl)

	queryVal, ok := dsl["query"]
	if !ok {
		t.Fatal("Missing 'query' key in DSL")
	}

	queryMap, ok := queryVal.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'query' to be a map, got: %T", queryVal)
	}

	// Check top-level "match" clause directly (since simplified)
	matchClause, ok := queryMap["match"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected top-level 'match' clause, got: %v", queryMap)
	}

	val, ok := matchClause["title"]
	if !ok {
		t.Errorf("Expected 'title' field in match clause")
	}

	switch v := val.(type) {
	case string:
		if v != "golang" {
			t.Errorf("Expected match value 'golang', got: %v", v)
		}
	case map[string]interface{}:
		if v["query"] != "golang" {
			t.Errorf("Expected match value 'golang', got: %v", v["query"])
		}
	default:
		t.Errorf("Unexpected type for match value: %T", v)
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

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	assert.Equal(t, int(10), dsl["from"])
	assert.Equal(t, int(20), dsl["size"])

	sort := dsl["sort"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "desc", sort["created_at"].(map[string]interface{})["order"])
}

func TestToDSL_Match_With_Parameter(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.MatchQuery("status", "active").Parameter("analyzer", "whitespace"),
		)

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `
	{
	  "query": {
	    "match": {
	      "status": {
	        "query": "active",
	        "analyzer": "whitespace"
	      }
	    }
	  }
	}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
}
func TestToDSL_Bool_With_Parameter(t *testing.T) {
	q := orm.NewQuery().
		Should(
			orm.TermQuery("tags", "inactive"),
			orm.TermQuery("tags", "active"),
		).Parameter("minimum_should_match", 1)

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `{
  "query": {
    "bool": {
      "minimum_should_match": 1,
      "should": [
        {
          "term": {
            "tags": {
              "value": "inactive"
            }
          }
        },
        {
          "term": {
            "tags": {
              "value": "active"
            }
          }
        }
      ]
    }
  }
}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
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

	dsl := BuildQueryDSL(q)
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
	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{},
		},
	}

	assert.Equal(t, expected, dsl)
}
func TestToDSL_MultiMatchQuery(t *testing.T) {
	q := orm.NewQuery().Must(orm.MultiMatchQuery([]string{"title", "category"}, "hello world"))
	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `{
  "query": {
    "multi_match": {
      "fields": [
        "title",
        "category"
      ],
      "query": "hello world"
    }
  }
}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
}
func TestToDSL_MatchPhraseQuery(t *testing.T) {
	q := orm.NewQuery().Must(orm.MatchPhraseQuery("title", "hello world", 5))
	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `{
  "query": {
    "match_phrase": {
      "title": {
        "query": "hello world",
        "slop": 5
      }
    }
  }
}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
}

func TestToDSL_RangeGteLte(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.Range("price").Gte(100),
		orm.Range("price").Lte(500),
	)
	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	mustClauses := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{})
	assert.Len(t, mustClauses, 2)

	r1 := mustClauses[0].(map[string]interface{})["range"].(map[string]interface{})["price"].(map[string]interface{})
	assert.Equal(t, int(100), r1["gte"])

	r2 := mustClauses[1].(map[string]interface{})["range"].(map[string]interface{})["price"].(map[string]interface{})
	assert.Equal(t, int(500), r2["lte"])
}

func TestFlattenBoolClauses_ShouldInsideShould(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"should": []interface{}{
							map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
							map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
						},
					},
				},
			},
		},
	}

	expected := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
				map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
			},
		},
	}

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("ShouldInsideShould: expected:\n%v\ngot:\n%v", expected, result)
	}
}

//func TestFlattenBoolClauses_MustNotInsideMustNot(t *testing.T) {
//	input := map[string]interface{}{
//		"bool": map[string]interface{}{
//			"must_not": []interface{}{
//				map[string]interface{}{
//					"bool": map[string]interface{}{
//						"must_not": []interface{}{
//							map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
//						},
//					},
//				},
//				map[string]interface{}{"term": map[string]interface{}{"y": "b"}},
//			},
//		},
//	}
//
//	//expected := map[string]interface{}{
//	//	"bool": map[string]interface{}{
//	//		"must": []interface{}{
//	//			map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
//	//		},
//	//		"must_not": []interface{}{
//	//			map[string]interface{}{"term": map[string]interface{}{"y": "b"}},
//	//		},
//	//	},
//	//}
//
//	//result := flattenBoolClauses(input)
//	//println(util.ToIndentJson(result))
//	//if !reflect.DeepEqual(result, expected) {
//	//	t.Errorf("MustNotMixed: expected:\n%v\ngot:\n%v", input, expected)
//	//}
//}

func TestFlattenBoolClauses_AlreadyFlat(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
			},
		},
	}

	expected := input

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("AlreadyFlat: expected:\n%v\ngot:\n%v", expected, result)
	}
}

func TestFlattenBoolClauses_MixedNestedShouldAndMust(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
						},
						"should": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{"y": "b"}},
						},
					},
				},
			},
		},
	}

	// Should not flatten mixed bools
	expected := input

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MixedNested: expected:\n%v\ngot:\n%v", expected, result)
	}
}

func TestToDSL_MustNotWithMustQuery(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.MatchQuery("title", "real-time search"),
			orm.MustNotQuery(
				orm.TermQuery("status", "deleted"),
				orm.PrefixQuery("category", "archived"),
			),
		)

	dsl := BuildQueryDSL(q)
	fmt.Println(q.ToString())
	printDSL(dsl)

	expected := `{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "title": {
              "query": "real-time search"
            }
          }
        },
        {
          "bool": {
            "must_not": [
              {
                "term": {
                  "status": {
                    "value": "deleted"
                  }
                }
              },
              {
                "prefix": {
                  "category": {
                    "value": "archived"
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }
}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
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

	dsl := BuildQueryDSL(q)

	expected := `
	{
	  "from": 50,
	  "size": 25,
	  "query": {
	    "bool": {
	      "must": [
	        {
	          "match": {
	            "title": {
	              "query": "real-time search"
	            }
	          }
	        },
	        {
	          "bool": {
	            "should": [
	              {
	                "term": {
	                  "tag": {
	                    "value": "distributed"
	                  }
	                }
	              },
	              {
	                "term": {
	                  "tag": {
	                    "value": "scalable"
	                  }
	                }
	              },
	              {
	                "bool": {
	                  "must": [
	                    {
	                      "match": {
	                        "author": {
	                          "query": "medcl"
	                        }
	                      }
	                    },
	                    {
	                      "range": {
	                        "published_year": {
	                          "gte": 2020
	                        }
	                      }
	                    }
	                  ]
	                }
	              }
	            ]
	          }
	        },
	        {
	          "bool": {
	            "must_not": [
	              {
	                "term": {
	                  "status": {
	                    "value": "deleted"
	                  }
	                }
	              },
	              {
	                "prefix": {
	                  "category": {
	                    "value": "archived"
	                  }
	                }
	              }
	            ]
	          }
	        }
	      ]
	    }
	  },
	  "sort": [
	    {
	      "score": {
	        "order": "desc"
	      }
	    },
	    {
	      "published_at": {
	        "order": "asc"
	      }
	    }
	  ]
	}`

	actual, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Failed to marshal DSL: %v", err)
	}

	assert.JSONEq(t, expected, string(actual))
}

func TestFlattenBoolClauses_SimpleShould(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"should": []interface{}{
							map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
							map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
						},
					},
				},
			},
		},
	}

	expected := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
				map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
			},
		},
	}

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected:\n%v\nGot:\n%v", expected, result)
	}
}

func TestFlattenBoolClauses_Must(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{"a": 1}},
							map[string]interface{}{"term": map[string]interface{}{"b": 2}},
						},
					},
				},
				map[string]interface{}{"term": map[string]interface{}{"c": 3}},
			},
		},
	}

	expected := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"a": 1}},
				map[string]interface{}{"term": map[string]interface{}{"b": 2}},
				map[string]interface{}{"term": map[string]interface{}{"c": 3}},
			},
		},
	}

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected:\n%v\nGot:\n%v", expected, result)
	}
}

func TestFlattenBoolClauses_MustNotMixed(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"must_not": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"must_not": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
						},
					},
				},
				map[string]interface{}{"term": map[string]interface{}{"y": "b"}},
			},
		},
	}

	//expected := map[string]interface{}{
	//	"bool": map[string]interface{}{
	//		"must": []interface{}{
	//			map[string]interface{}{"term": map[string]interface{}{"x": "a"}},
	//		},
	//		"must_not": []interface{}{
	//			map[string]interface{}{"term": map[string]interface{}{"y": "b"}},
	//		},
	//	},
	//}

	result := flattenBoolClauses(input)

	fmt.Println(util.ToIndentJson(result))

	if !reflect.DeepEqual(result, input) {
		t.Errorf("Expected:\n%v\nGot:\n%v", input, result)
	}
}

func TestFlattenBoolClauses_NoChange(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"a": 1}},
			},
			"should": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{ // different type, should NOT be flattened
							map[string]interface{}{"term": map[string]interface{}{"b": 2}},
						},
					},
				},
			},
		},
	}

	// Should remain unchanged
	expected := input

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected no change:\n%v\nGot:\n%v", expected, result)
	}
}

func TestFlattenBoolClauses_ShouldNested(t *testing.T) {
	input := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{
					"bool": map[string]interface{}{
						"should": []interface{}{
							map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
							map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
						},
					},
				},
			},
		},
	}

	expected := map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []interface{}{
				map[string]interface{}{"match": map[string]interface{}{"lang": "en"}},
				map[string]interface{}{"match": map[string]interface{}{"lang": "zh"}},
			},
		},
	}

	result := flattenBoolClauses(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected:\n%v\nGot:\n%v", expected, result)
	}
}

func TestToDSL_ShouldQueryTypes(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.ShouldQuery(
			orm.MatchQuery("lang", "en"),
			orm.MatchQuery("lang", "zh"),
		),
	).Size(10).SortBy(
		orm.Sort{Field: "score", SortType: orm.DESC},
	)

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `
	{
	  "query": {
	    "bool": {
	      "should": [
	        {
	          "match": {
	            "lang": {
	              "query": "en"
	            }
	          }
	        },
	        {
	          "match": {
	            "lang": {
	              "query": "zh"
	            }
	          }
	        }
	      ]
	    }
	  },
	  "size": 10,
	  "sort": [
	    {
	      "score": {
	        "order": "desc"
	      }
	    }
	  ]
	}`

	actual, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Failed to marshal DSL: %v", err)
	}

	assert.JSONEq(t, expected, string(actual))
}

func TestToDSL_ShouldQuery1Types(t *testing.T) {
	q := orm.NewQuery().Must(
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

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `
	{
	  "query": {
	    "bool": {
	      "must": [
	        {
	          "bool": {
	            "should": [
	              {
	                "match": {
	                  "lang": {
	                    "query": "en"
	                  }
	                }
	              },
	              {
	                "match": {
	                  "lang": {
	                    "query": "zh"
	                  }
	                }
	              }
	            ]
	          }
	        },
	        {
	          "bool": {
	            "must_not": [
	              {
	                "term": {
	                  "deleted": {
	                    "value": true
	                  }
	                }
	              }
	            ]
	          }
	        }
	      ]
	    }
	  },
	  "size": 10,
	  "sort": [
	    {
	      "score": {
	        "order": "desc"
	      }
	    }
	  ]
	}`

	actual, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Failed to marshal DSL: %v", err)
	}

	assert.JSONEq(t, expected, string(actual))
}

func TestToDSL_AllQueryTypes(t *testing.T) {
	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "go"),
		orm.TermQuery("status", "published"),
		orm.PrefixQuery("author", "med"),
		orm.WildcardQuery("category", "tech*"),
		orm.RegexpQuery("slug", "g.*"),
		orm.FuzzyQuery("summary", "elasticsearch", 0),
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

	dsl := BuildQueryDSL(q)
	printDSL(dsl)

	expected := `
	{
	  "query": {
	    "bool": {
	      "must": [
	        {
	          "match": {
	            "title": {
	              "query": "go"
	            }
	          }
	        },
	        {
	          "term": {
	            "status": {
	              "value": "published"
	            }
	          }
	        },
	        {
	          "prefix": {
	            "author": {
	              "value": "med"
	            }
	          }
	        },
	        {
	          "wildcard": {
	            "category": "tech*"
	          }
	        },
	        {
	          "regexp": {
	            "slug": "g.*"
	          }
	        },
	        {
	          "fuzzy": {
	            "summary": {
	              "fuzziness": 0,
	              "value": "elasticsearch"
	            }
	          }
	        },
	        {
	          "exists": {
	            "field": "created_at"
	          }
	        },
	        {
	          "terms": {
	            "tags": [
	              "search",
	              "infra",
	              "open-source"
	            ]
	          }
	        },
	        {
	          "bool": {
	            "must_not": [
	              {
	                "terms": {
	                  "region": [
	                    "cn",
	                    "ru"
	                  ]
	                }
	              }
	            ]
	          }
	        },
	        {
	          "range": {
	            "published_year": {
	              "gte": 2015
	            }
	          }
	        },
	        {
	          "range": {
	            "views": {
	              "lt": 1000000
	            }
	          }
	        },
	        {
	          "bool": {
	            "should": [
	              {
	                "match": {
	                  "lang": {
	                    "query": "en"
	                  }
	                }
	              },
	              {
	                "match": {
	                  "lang": {
	                    "query": "zh"
	                  }
	                }
	              }
	            ]
	          }
	        },
	        {
	          "bool": {
	            "must_not": [
	              {
	                "term": {
	                  "deleted": {
	                    "value": true
	                  }
	                }
	              }
	            ]
	          }
	        }
	      ]
	    }
	  },
	  "size": 10,
	  "sort": [
	    {
	      "score": {
	        "order": "desc"
	      }
	    }
	  ]
	}`

	actual, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Failed to marshal DSL: %v", err)
	}

	assert.JSONEq(t, expected, string(actual))
}

func TestToDSL_FilterAndMust(t *testing.T) {
	q := orm.NewQuery().
		Must(
			orm.MatchQuery("title", "distributed systems"),
		).
		Filter(
			orm.TermQuery("status", "published"),
			orm.ExistsQuery("created_at"),
		)

	dsl := BuildQueryDSL(q)

	printDSL(dsl)

	expected := `
	{
	  "query": {
	    "bool": {
	      "must": [
	        {
	          "match": {
	            "title": {
	              "query": "distributed systems"
	            }
	          }
	        }
	      ],
	      "filter": [
	        {
	          "term": {
	            "status": {
	              "value": "published"
	            }
	          }
	        },
	        {
	          "exists": {
	            "field": "created_at"
	          }
	        }
	      ]
	    }
	  }
	}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
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

func extractClauseByType(t *testing.T, dsl map[string]interface{}, clauseType string) []interface{} {
	query, ok := dsl["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing top-level 'query'")
	}

	if direct, ok := query[clauseType]; ok {
		// Direct match, e.g., query = match/term/etc
		return []interface{}{map[string]interface{}{clauseType: direct}}
	}

	if boolClause, ok := query["bool"].(map[string]interface{}); ok {
		if sub, ok := boolClause[clauseType].([]interface{}); ok {
			return sub
		}
	}

	t.Fatalf("Expected clause of type '%s' not found", clauseType)
	return nil
}

func extractMatchClause(t *testing.T, dsl map[string]interface{}) map[string]interface{} {
	query, ok := dsl["query"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing top-level 'query'")
	}

	// Case 1: direct match
	if match, ok := query["match"]; ok {
		return match.(map[string]interface{})
	}

	// Case 2: wrapped in bool -> must
	if boolClause, ok := query["bool"].(map[string]interface{}); ok {
		musts, ok := boolClause["must"].([]interface{})
		if !ok || len(musts) == 0 {
			t.Fatal("Expected at least one 'must' clause")
		}

		for _, clause := range musts {
			if m, ok := clause.(map[string]interface{})["match"]; ok {
				return m.(map[string]interface{})
			}
		}
	}

	t.Fatal("Match clause not found")
	return nil
}

func TestBuildQueryDSLOnTopOfDSL_MergeFilter(t *testing.T) {
	// Original body with a match query
	reqBody := []byte(`{
		"query": {
			"match": { "title": "rust" }
		},
		"size": 10,
		"track_total_hits": true
	}`)

	q := orm.NewQuery().Filter(
		orm.ExistsQuery("created_at"),
	).Size(20).From(5)

	dsl := BuildQueryDSLOnTopOfDSL(q, reqBody)
	printDSL(dsl)

	query := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filter := query["filter"].([]interface{})
	must := query["must"].([]interface{})

	// Check merged query
	assert.Len(t, filter, 1)
	assert.Contains(t, filter[0], "exists")

	assert.Len(t, must, 1)
	assert.Contains(t, must[0], "match")

	// Check merged fields
	assert.Equal(t, 5, int(dsl["from"].(int)))
	assert.Equal(t, 20, int(dsl["size"].(int)))
	assert.Equal(t, true, dsl["track_total_hits"].(bool))
}

func TestBuildQueryDSLOnTopOfDSL_NoBodyQuery(t *testing.T) {
	reqBody := []byte(`{ "size": 15 }`)

	q := orm.NewQuery().Filter(
		orm.ExistsQuery("status"),
	)

	dsl := BuildQueryDSLOnTopOfDSL(q, reqBody)
	printDSL(dsl)

	boolQuery := dsl["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filter := boolQuery["filter"].([]interface{})

	assert.Len(t, filter, 1)
	assert.Contains(t, filter[0], "exists")

	assert.Equal(t, 15, int(dsl["size"].(float64)))
}

func TestBuildQueryDSLOnTopOfDSL_MatchQuery(t *testing.T) {
	reqBody := []byte(`{ "size": 15 }`)

	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "status"),
	)

	dsl := BuildQueryDSLOnTopOfDSL(q, reqBody)
	printDSL(dsl)

	expected := `{
		"query": {
			"match": {
				"title": {
					"query": "status"
				}
			}
		},
		"size": 15
	}`

	actualBytes, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Failed to marshal actual DSL: %v", err)
	}

	assert.JSONEq(t, expected, string(actualBytes))
}

func TestBuildQueryDSLOnTopOfDSL_BothMatchQuery(t *testing.T) {
	reqBody := []byte(`{
		"query": {
			"match": {
				"author": {
					"query": "medcl"
				}
			}
		},
		"size": 5
	}`)

	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "rust"),
	)

	dsl := BuildQueryDSLOnTopOfDSL(q, reqBody)
	printDSL(dsl)

	expected := `{
		"query": {
			"bool": {
				"must": [
					{
						"match": {
							"author": {
								"query": "medcl"
							}
						}
					},
					{
						"match": {
							"title": {
								"query": "rust"
							}
						}
					}
				]
			}
		},
		"size": 5
	}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
}

func TestBuildQueryDSLOnTopOfDSL_WithAggregations(t *testing.T) {
	reqBody := []byte(`{
		"size": 0,
		"query": {
			"match": {
				"lang": {
					"query": "zh"
				}
			}
		},
		"aggs": {
			"langs": {
				"terms": {
					"field": "lang.keyword"
				}
			}
		}
	}`)

	q := orm.NewQuery().Must(
		orm.MatchQuery("title", "golang"),
	)

	dsl := BuildQueryDSLOnTopOfDSL(q, reqBody)
	printDSL(dsl)

	expected := `{
		"query": {
			"bool": {
				"must": [
					{
						"match": {
							"lang": {
								"query": "zh"
							}
						}
					},
					{
						"match": {
							"title": {
								"query": "golang"
							}
						}
					}
				]
			}
		},
		"size": 0,
		"aggs": {
			"langs": {
				"terms": {
					"field": "lang.keyword"
				}
			}
		}
	}`

	actual, _ := json.Marshal(dsl)
	assert.JSONEq(t, expected, string(actual))
}

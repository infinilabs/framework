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
	"reflect"
	"testing"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// assertJSONEquals is a helper function to compare if two JSON sources are equivalent.
// It unmarshals them into maps and uses reflect.DeepEqual for a robust comparison,
// ignoring differences in whitespace and key order.
func assertJSONEquals(t *testing.T, got []byte, expected string) {
	t.Helper() // Marks this function as a test helper.

	var gotMap, expectedMap map[string]interface{}

	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatalf("Failed to unmarshal 'got' JSON: %v\nJSON was: %s", err, string(got))
	}
	if err := json.Unmarshal([]byte(expected), &expectedMap); err != nil {
		t.Fatalf("Failed to unmarshal 'expected' JSON: %v\nJSON was: %s", err, expected)
	}

	if !reflect.DeepEqual(gotMap, expectedMap) {
		gotPretty, _ := json.MarshalIndent(gotMap, "", "  ")
		expectedPretty, _ := json.MarshalIndent(expectedMap, "", "  ")
		t.Errorf("JSON mismatch:\n--- GOT ---\n%s\n\n--- EXPECTED ---\n%s", string(gotPretty), string(expectedPretty))
	}
}

// TestBuild_SingleTermsAggregation tests the compilation of a simple, single-level terms aggregation.
func TestBuild_SingleTermsAggregation(t *testing.T) {
	// Arrange: Build an abstract request.
	request := map[string]orm.Aggregation{
			"group_by_brand": &orm.TermsAggregation{
				Field: "brand.keyword",
				Size:  10,
			},
		}

	builder := NewAggreationBuilder()
	resultJSON, err := builder.Build(request)

	// Assert: Check for errors and compare the output JSON.
	if err != nil {
		t.Fatalf("Build() returned an unexpected error: %v", err)
	}

	expectedJSON := `{
			"group_by_brand": {
				"terms": {
					"field": "brand.keyword",
					"size": 10
				}
			}
	}`

	assertJSONEquals(t, util.MustToJSONBytes(resultJSON), expectedJSON)
}

// TestBuild_NestedAggregation tests a terms aggregation with a nested metric aggregation.
func TestBuild_NestedAggregation(t *testing.T) {
	// Arrange
	request := map[string]orm.Aggregation{
			"products_by_type": (&orm.TermsAggregation{
				Field: "type.keyword",
			}).AddNested("average_price", &orm.MetricAggregation{
				Field: "price",
				Type: "avg",
			}),
		}

	// Act
	builder := NewAggreationBuilder()
	resultJSON, err := builder.Build(request)

	// Assert
	if err != nil {
		t.Fatalf("Build() returned an unexpected error: %v", err)
	}

	expectedJSON := `{
			"products_by_type": {
				"terms": {
					"field": "type.keyword"
				},
				"aggs": {
					"average_price": {
						"avg": {
							"field": "price"
						}
					}
				}
			}
	}`

	assertJSONEquals(t, util.MustToJSONBytes(resultJSON), expectedJSON)
}

// TestBuild_MultipleTopLevelAggs tests a request with multiple aggregations at the root level.
func TestBuild_MultipleTopLevelAggs(t *testing.T) {
	// Arrange
	request := map[string]orm.Aggregation{
		"total_sales": &orm.MetricAggregation{
			Type: "avg",
			Field: "price",
		},
		"sales_by_month": &orm.DateHistogramAggregation{
			Field:    "order_date",
			Interval: "1M",
			TimeZone: "UTC",
		},
	}

	builder := NewAggreationBuilder()
	resultJSON, err := builder.Build(request)

	// Assert
	if err != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", err)
	}

	expectedJSON := `{
			"total_sales": {
				"avg": {
					"field": "price"
				}
			},
			"sales_by_month": {
				"date_histogram": {
					"field": "order_date",
					"calendar_interval": "1M",
					"time_zone": "UTC"
				}
			}
	}`

	assertJSONEquals(t, util.MustToJSONBytes(resultJSON), expectedJSON)
}

// TestBuild_NoAggregations tests that a request with no aggregations compiles correctly.
func TestBuild_NoAggregations(t *testing.T) {
	// Arrange
	var request = map[string]orm.Aggregation{}

	builder := NewAggreationBuilder()
	resultJSON, err := builder.Build(request)

	// Assert
	if err != nil {
		t.Fatalf("Build() returned an unexpected error: %v", err)
	}

	// The 'aggs' key should be omitted from the final JSON.
	expectedJSON := `{
	}`

	assertJSONEquals(t, util.MustToJSONBytes(resultJSON), expectedJSON)
}

// unsupportedAgg is a mock struct for testing error handling.
type unsupportedAgg struct{ orm.TermsAggregation }

// TestBuild_UnsupportedAggregationType tests that the compiler returns an error for unknown types.
func TestBuild_UnsupportedAggregationType(t *testing.T) {
	request :=  map[string]orm.Aggregation{
			"bad_agg": &unsupportedAgg{},
		}

	builder := NewAggreationBuilder()
	_, err := builder.Build(request)

	// Assert
	if err == nil {
		t.Fatal("Build() was expected to return an error, but it did not")
	}
}

func TestBuild_ComplexAggregation(t *testing.T) {
	// test cases for complex aggregations
	testCases := []struct {
		name     string
		request  map[string]orm.Aggregation
		expected string
	}{
		{
			name: "Complex Nested Aggregation",
			request: map[string]orm.Aggregation{
				"sales_by_category": (&orm.TermsAggregation{
					Field: "category.keyword",
				}).AddNested("avg_price", &orm.MetricAggregation{
					Field: "price",
					Type:  "avg",
				}).AddNested("total_sales", &orm.MetricAggregation{
					Field: "sales",
					Type:  "sum",
				}),
			},
			expected: `{
				"sales_by_category": {
					"terms": {
						"field": "category.keyword"
					},
					"aggs": {
						"avg_price": {
							"avg": {
								"field": "price"
							}
						},
						"total_sales": {
							"sum": {
								"field": "sales"
							}
						}
					}
				}
			}`,
		},
		{
			name: "Date Histogram with Nested Terms",
			request: map[string]orm.Aggregation{
				"sales_over_time": (&orm.DateHistogramAggregation{
					Field:    "sale_date",
					Interval: "1M",
					TimeZone: "UTC",
				}).AddNested("sales_by_region", (&orm.TermsAggregation{
					Field: "region.keyword",
				}).AddNested("avg_sale", &orm.MetricAggregation{
					Field: "sale_amount",
					Type:  "avg",
				})),
			},
			expected: `{
				"sales_over_time": {
					"date_histogram": {
						"field": "sale_date",
						"calendar_interval": "1M",
						"time_zone": "UTC"
					},
					"aggs": {
						"sales_by_region": {
							"terms": {
								"field": "region.keyword"
							},
							"aggs": {
								"avg_sale": {
									"avg": {
										"field": "sale_amount"
									}
								}
							}
						}
					}
				}
			}`,
		},
		{
			name: "Percentiles Aggregation",
			request: map[string]orm.Aggregation{
				"request_over_time": (&orm.DateHistogramAggregation{
					Field:    "timestamp",
					Interval: "1M",
				}).AddNested("response_percentiles", &orm.PercentilesAggregation{
					Field:    "response_time",
					Percents: []float64{50, 90, 95},
				}),
			},
			expected: `{
				"request_over_time": {
					"date_histogram": {
						"field": "timestamp",
						"calendar_interval": "1M"
					},
					"aggs": {
						"response_percentiles": {
							"percentiles": {
								"field": "response_time",
								"percents": [50, 90, 95]
							}
						}
					}
				}
			}`,
		},
		{
			name: "complex aggregation with derivative",
			request: map[string]orm.Aggregation{
				"sales_over_time": (&orm.DateHistogramAggregation{
					Field:    "sale_date",
					Interval: "1M",
				}).AddNested("avg_sale", &orm.MetricAggregation{
					Field: "sale_amount",
					Type:  "avg",
				}).AddNested("sales_derivative", &orm.DerivativeAggregation{
					BucketsPath: "avg_sale",
				}),
			},
			expected: `{
				"sales_over_time": {
					"date_histogram": {
						"field": "sale_date",
						"calendar_interval": "1M"
					},
					"aggs": {
						"avg_sale": {
							"avg": {
								"field": "sale_amount"
							}
						},
						"sales_derivative": {
							"derivative": {
								"buckets_path": "avg_sale"
							}
						}
					}
				}
			}`,
		},
	}
	// Act & Assert
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewAggreationBuilder()
			resultJSON, err := builder.Build(tc.request)

			if err != nil {
				t.Fatalf("Build() returned an unexpected error: %v", err)
			}

			assertJSONEquals(t, util.MustToJSONBytes(resultJSON), tc.expected)
		})
	}
}

func TestBuildAggsWith(t *testing.T) {
	q := orm.NewQuery()
	q.Must(orm.TermQuery("product_id", "12345"))
  aggs := map[string]orm.Aggregation{
    "sales_over_time": (&orm.DateHistogramAggregation{
      Field:    "sale_date",
      Interval: "1M",
    }).AddNested("sales_by_region", (&orm.TermsAggregation{
      Field: "region.keyword",
    }).AddNested("avg_sale", &orm.MetricAggregation{
      Field: "sale_amount",
      Type:  "avg",
    })),
  }
	q.Aggs = aggs
	dsl := BuildQueryDSL(q)
	expected := `{"aggs":{"sales_over_time":{"date_histogram":{"field":"sale_date","calendar_interval":"1M"},"aggs":{"sales_by_region":{"terms":{"field":"region.keyword"},"aggs":{"avg_sale":{"avg":{"field":"sale_amount"}}}}}}},"query":{"term":{"product_id":{"value":"12345"}}}}`
	if got := util.MustToJSON(dsl); got != expected {
		t.Errorf("BuildQueryDSL() = %v, want %v", got, expected)
	}
}
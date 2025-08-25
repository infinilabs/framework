package orm

import (
	"net/url"
	"reflect"
	"testing"
)

func TestParseAggregationsFromQuery_SingleTerms(t *testing.T) {
	// Arrange
	rawURL := "http://example.com?agg[types][terms][field]=product.keyword&agg[types][terms][size]=5"
	parsedURL, _ := url.Parse(rawURL)
	
	// Expected abstract structure
	expected := map[string]Aggregation{
		"types": &TermsAggregation{
			Field: "product.keyword",
			Size:  5,
		},
	}

	// Act
	result, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err != nil {
		t.Fatalf("ParseAggregationsFromQuery returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Resulting aggregation map does not match expected.\nGot: %#v\nWant: %#v", result, expected)
	}
}

func TestParseAggregationsFromQuery_Nested(t *testing.T) {
	// Arrange
	rawURL := "http://example.com?agg[by_brand][terms][field]=brand&agg[by_brand][aggs][avg_price][avg][field]=price"
	parsedURL, _ := url.Parse(rawURL)

	// Expected
	expected := map[string]Aggregation{
		"by_brand": (&TermsAggregation{Field: "brand"}).AddNested(
			"avg_price", &MetricAggregation{Type: "avg", Field: "price"},
		),
	}

	// Act
	result, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err != nil {
		t.Fatalf("ParseAggregationsFromQuery returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Resulting aggregation map does not match expected.\nGot: %#v\nWant: %#v", result, expected)
	}
}

func TestParseAggregationsFromQuery_MultipleTopLevel(t *testing.T) {
	// Arrange
	rawURL := "http://example.com?agg[types][terms][field]=type&agg[sales_by_month][date_histogram][field]=date&agg[sales_by_month][date_histogram][interval]=1M"
	parsedURL, _ := url.Parse(rawURL)

	// Expected
	expected := map[string]Aggregation{
		"types": &TermsAggregation{
			Field: "type",
		},
		"sales_by_month": &DateHistogramAggregation{
			Field:    "date",
			Interval: "1M",
		},
	}

	// Act
	result, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err != nil {
		t.Fatalf("ParseAggregationsFromQuery returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Resulting aggregation map does not match expected.\nGot: %#v\nWant: %#v", result, expected)
	}
}

func TestParseAggregationsFromQuery_IgnoresNonAggParams(t *testing.T) {
	// Arrange
	rawURL := "http://example.com?query=search&from=10&agg[types][terms][field]=type"
	parsedURL, _ := url.Parse(rawURL)

	// Expected
	expected := map[string]Aggregation{
		"types": &TermsAggregation{Field: "type"},
	}

	// Act
	result, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err != nil {
		t.Fatalf("ParseAggregationsFromQuery returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Resulting aggregation map does not match expected.\nGot: %#v\nWant: %#v", result, expected)
	}
}

func TestParseAggregationsFromQuery_ErrorOnStructureConflict(t *testing.T) {
	// Arrange
	rawURL := "http://example.com?agg[types][terms]=some_value&agg[types][terms][field]=type"
	parsedURL, _ := url.Parse(rawURL)

	// Act
	_, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err == nil {
		t.Fatal("Expected an error due to structure conflict, but got none")
	}
}

func TestParseAggregationsFromQuery_URLEncoded(t *testing.T) {
	// Arrange
	rawURL := "http://localhost:8000/collection/region/_search?sort=created%3Aasc&from=0&size=20&agg[%E4%BE%9B%E5%BA%94%E5%95%86][terms][field]=provider&agg[%E4%BE%9B%E5%BA%94%E5%95%86][terms][size]=100"
	parsedURL, _ := url.Parse(rawURL)

	// Expected
	expected := map[string]Aggregation{
		"供应商": &TermsAggregation{
			Field: "provider",
			Size:  100,
		},
	}

	// Act
	result, err := ParseAggregationsFromQuery(parsedURL.Query())

	// Assert
	if err != nil {
		t.Fatalf("ParseAggregationsFromQuery returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Resulting aggregation map does not match expected.\nGot: %#v\nWant: %#v", result, expected)
	}
}
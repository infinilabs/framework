package elasticsearch

import (
	"testing"

	"github.com/segmentio/encoding/json"
)

func TestStripTermsMissingFromQueryDSL(t *testing.T) {
	source := []byte(`{
		"aggs": {
			"a": {
				"terms": {
					"field": "metadata.labels.cluster_id",
					"missing": "",
					"size": 2
				},
				"aggs": {
					"b": {
						"date_range": {
							"field": "timestamp",
							"ranges": [{"from":"now-1d/d","to":"now/d"}]
						},
						"aggs": {
							"c": {
								"terms": {
									"field": "payload.elasticsearch.cluster_health.status",
									"missing": "",
									"size": 2
								}
							}
						}
					}
				}
			}
		}
	}`)

	got, changed := stripTermsMissingFromQueryDSL(source)
	if !changed {
		t.Fatal("expected query DSL to be changed")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	aggA := parsed["aggs"].(map[string]interface{})["a"].(map[string]interface{})
	termsA := aggA["terms"].(map[string]interface{})
	if _, ok := termsA["missing"]; ok {
		t.Fatalf("expected top-level terms.missing to be removed, got %#v", termsA)
	}

	aggB := aggA["aggs"].(map[string]interface{})["b"].(map[string]interface{})
	aggC := aggB["aggs"].(map[string]interface{})["c"].(map[string]interface{})
	termsC := aggC["terms"].(map[string]interface{})
	if _, ok := termsC["missing"]; ok {
		t.Fatalf("expected nested terms.missing to be removed, got %#v", termsC)
	}
}

func TestShouldRetryWithoutTermsMissing(t *testing.T) {
	if shouldRetryWithoutTermsMissing(400, []byte(`{"error":{"reason":"UnmappedTerms unsupported"}}`)) {
		t.Fatal("should not retry on non-5xx status")
	}
	if !shouldRetryWithoutTermsMissing(500, []byte(`{"error":{"reason":"Aggregation [x] is of type [UnmappedTerms] which is currently unsupported."}}`)) {
		t.Fatal("expected retry to be enabled for UnmappedTerms unsupported error")
	}
}

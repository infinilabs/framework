package elastic

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"infini.sh/framework/core/util"
)

func TestBuildQuantilePercents(t *testing.T) {
	got := buildQuantilePercents(4)
	want := []float64{25, 50, 75}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected percents: got %v want %v", got, want)
	}
}

func TestBuildQuantilePartitionsCreatesOpenEdgeRanges(t *testing.T) {
	partitions := buildQuantilePartitions([]float64{10, 20, 30}, "value", PartitionByNumber, nil)
	if len(partitions) != 2 {
		t.Fatalf("unexpected partition count: %d", len(partitions))
	}

	firstRange := getMustClause(t, partitions[0].Filter)["range"].(util.MapStr)["value"].(util.MapStr)
	if _, ok := firstRange["gt"]; ok {
		t.Fatalf("expected first partition to have no lower bound, got %v", firstRange)
	}
	if got := firstRange["lte"]; got != float64(20) {
		t.Fatalf("unexpected first upper bound: %v", got)
	}

	secondRange := getMustClause(t, partitions[1].Filter)["range"].(util.MapStr)["value"].(util.MapStr)
	if got := secondRange["gt"]; got != float64(20) {
		t.Fatalf("unexpected second lower bound: %v", got)
	}
	if _, ok := secondRange["lte"]; ok {
		t.Fatalf("expected last partition to have no upper bound, got %v", secondRange)
	}
}

func TestBuildQuantilePartitionsSinglePartitionUsesExistsFilter(t *testing.T) {
	partitions := buildQuantilePartitions([]float64{5, 5}, "value", PartitionByNumber, nil)
	if len(partitions) != 1 {
		t.Fatalf("unexpected partition count: %d", len(partitions))
	}

	clause := getMustClause(t, partitions[0].Filter)
	exists, ok := clause["exists"].(util.MapStr)
	if !ok {
		t.Fatalf("expected exists clause, got %v", clause)
	}
	if exists["field"] != "value" {
		t.Fatalf("unexpected exists field: %v", exists["field"])
	}
}

func TestBuildOpenPartitionFilterPreservesDateFormat(t *testing.T) {
	upper := 1000.0
	filter := buildOpenPartitionFilter(nil, &upper, "ts", PartitionByDate, nil)
	rangeFilter := getMustClause(t, filter)["range"].(util.MapStr)["ts"].(util.MapStr)
	if got := rangeFilter["format"]; got != "epoch_millis" {
		t.Fatalf("unexpected date format: %v", got)
	}
	if got := rangeFilter["lte"]; got != int64(1000) {
		t.Fatalf("unexpected upper bound: %v", got)
	}
}

func TestBuildOpenPartitionFilterRoundsDatePercentileBoundaries(t *testing.T) {
	lower := 1779109187904.8455
	upper := 1779109187999.999
	filter := buildOpenPartitionFilter(&lower, &upper, "created_at", PartitionByDate, nil)
	rangeFilter := getMustClause(t, filter)["range"].(util.MapStr)["created_at"].(util.MapStr)
	if got := rangeFilter["gt"]; got != int64(1779109187904) {
		t.Fatalf("unexpected lower bound: %v", got)
	}
	if got := rangeFilter["lte"]; got != int64(1779109187999) {
		t.Fatalf("unexpected upper bound: %v", got)
	}
}

func TestBuildBoundedPartitionFilterRoundsDateBoundaries(t *testing.T) {
	filter := buildBoundedPartitionFilter(1779109187904.1, 1779109187999.9, "created_at", PartitionByDate, nil)
	rangeFilter := getMustClause(t, filter)["range"].(util.MapStr)["created_at"].(util.MapStr)
	if got := rangeFilter["gte"]; got != int64(1779109187905) {
		t.Fatalf("unexpected lower bound: %v", got)
	}
	if got := rangeFilter["lte"]; got != int64(1779109187999) {
		t.Fatalf("unexpected upper bound: %v", got)
	}
}

func TestBuildExactTermPartitionFilter(t *testing.T) {
	filter := buildExactTermPartitionFilter("pmid-1", "pmid.keyword", nil)
	termFilter := getMustClause(t, filter)["term"].(util.MapStr)["pmid.keyword"].(util.MapStr)
	if got := termFilter["value"]; got != "pmid-1" {
		t.Fatalf("unexpected term value: %v", got)
	}
}

func TestBuildOtherTermsPartitionFilter(t *testing.T) {
	filter := buildOtherTermsPartitionFilter([]string{"a", "b"}, "pmid.keyword", nil)
	boolFilter := filter["bool"].(util.MapStr)
	mustNot := boolFilter["must_not"].([]interface{})
	termsFilter := mustNot[0].(util.MapStr)["terms"].(util.MapStr)
	values := termsFilter["pmid.keyword"].([]string)
	if !reflect.DeepEqual(values, []string{"a", "b"}) {
		t.Fatalf("unexpected excluded values: %v", values)
	}
}

func TestBuildHashPartitionFilter(t *testing.T) {
	filter := buildHashPartitionFilter(1, 8, "pmid.keyword", nil)
	scriptFilter := getMustClause(t, filter)["script"].(util.MapStr)["script"].(util.MapStr)
	if scriptFilter["lang"] != "painless" {
		t.Fatalf("unexpected script language: %v", scriptFilter["lang"])
	}
	source, ok := scriptFilter["source"].(string)
	if !ok {
		t.Fatalf("unexpected script source: %T", scriptFilter["source"])
	}
	if !strings.Contains(source, "doc['pmid.keyword']") {
		t.Fatalf("unexpected script source: %s", source)
	}
	if !strings.Contains(source, "value != ''") {
		t.Fatalf("expected empty strings to be excluded from hash partition, got %s", source)
	}
	if strings.Contains(source, "Math.floorMod") {
		t.Fatalf("unexpected script source: %s", source)
	}
	params := scriptFilter["params"].(util.MapStr)
	if params["partition_count"] != 8 || params["partition_id"] != 1 {
		t.Fatalf("unexpected script params: %v", params)
	}
	if _, ok := params["field"]; ok {
		t.Fatalf("field should not be passed as a script param: %v", params)
	}
}

func TestBuildHashPartitionAggQueryAppliesOuterFilter(t *testing.T) {
	query := buildHashPartitionAggQuery("pmid.keyword", 8, util.MapStr{
		"term": util.MapStr{
			"env": util.MapStr{"value": "prod"},
		},
	})

	if !reflect.DeepEqual(query["query"], util.MapStr{
		"term": util.MapStr{
			"env": util.MapStr{"value": "prod"},
		},
	}) {
		t.Fatalf("expected outer filter to be applied at top-level query, got %v", query["query"])
	}

	termsAgg := query["aggs"].(util.MapStr)["partitions"].(util.MapStr)["terms"].(util.MapStr)
	if got := termsAgg["size"]; got != 8 {
		t.Fatalf("unexpected partition size: %v", got)
	}
	if got := termsAgg["value_type"]; got != "long" {
		t.Fatalf("unexpected value_type: %v", got)
	}
	script := termsAgg["script"].(util.MapStr)
	source, ok := script["source"].(string)
	if !ok {
		t.Fatalf("unexpected script source type: %T", script["source"])
	}
	if !strings.Contains(source, "return null") {
		t.Fatalf("expected missing values to be skipped in hash aggregation, got %s", source)
	}
	if !strings.Contains(source, "value == ''") {
		t.Fatalf("expected empty strings to be excluded in hash aggregation, got %s", source)
	}
	params := script["params"].(util.MapStr)
	if got := params["partition_count"]; got != 8 {
		t.Fatalf("unexpected partition_count: %v", got)
	}
}

func TestExtractHashPartitionDocCountsMapsByBucketKey(t *testing.T) {
	counts := extractHashPartitionDocCounts(&SearchResponse{
		Aggregations: map[string]AggregationResponse{
			"partitions": {
				Buckets: []BucketBase{
					{"key": float64(5), "doc_count": float64(12)},
					{"key": "1", "doc_count": float64(7)},
					{"key": float64(99), "doc_count": float64(3)},
				},
			},
		},
	}, 8)

	expected := []int64{0, 7, 0, 0, 0, 12, 0, 0}
	if !reflect.DeepEqual(counts, expected) {
		t.Fatalf("unexpected hash counts: got %v want %v", counts, expected)
	}
}

func TestBuildMissingFieldConditionIncludesEmptyString(t *testing.T) {
	filter := buildMissingFieldCondition("pmid.keyword")
	boolFilter, ok := filter["bool"].(util.MapStr)
	if !ok {
		t.Fatalf("expected bool filter, got %v", filter)
	}
	if got := boolFilter["minimum_should_match"]; got != 1 {
		t.Fatalf("unexpected minimum_should_match: %v", got)
	}
	should, ok := boolFilter["should"].([]interface{})
	if !ok || len(should) != 2 {
		t.Fatalf("expected two should clauses, got %v", boolFilter["should"])
	}
	termFilter := should[1].(util.MapStr)["term"].(util.MapStr)["pmid.keyword"].(util.MapStr)
	if got := termFilter["value"]; got != "" {
		t.Fatalf("unexpected empty-string term filter: %v", termFilter)
	}
}

func TestBuildMissingFieldFilterPreservesOuterFilter(t *testing.T) {
	filter := buildMissingFieldFilter("pmid.keyword", util.MapStr{
		"term": util.MapStr{
			"env": util.MapStr{"value": "prod"},
		},
	})
	boolFilter, ok := filter["bool"].(util.MapStr)
	if !ok {
		t.Fatalf("expected bool filter, got %v", filter)
	}
	must, ok := boolFilter["must"].([]interface{})
	if !ok || len(must) != 2 {
		t.Fatalf("expected two must clauses, got %v", boolFilter["must"])
	}
	innerBool, ok := must[0].(util.MapStr)["bool"].(util.MapStr)
	if !ok {
		t.Fatalf("expected wrapped missing bool filter, got %v", must[0])
	}
	if got := innerBool["minimum_should_match"]; got != 1 {
		t.Fatalf("unexpected minimum_should_match: %v", got)
	}
}

func TestBuildPainlessStringLiteralEscapesSingleQuote(t *testing.T) {
	got := buildPainlessStringLiteral("foo'bar")
	if got != `'foo\'bar'` {
		t.Fatalf("unexpected painless string literal: %s", got)
	}
}

func TestEnsurePartitionSearchResponseOKReturnsBackendReason(t *testing.T) {
	err := ensurePartitionSearchResponseOK(&SearchResponse{
		ResponseBase: ResponseBase{
			StatusCode: http.StatusInternalServerError,
			RawResult: &util.Result{
				Body: []byte(`{"error":{"reason":"runtime script failure"},"status":500}`),
			},
			InternalError: InternalError{
				Error: &ErrorDetail{
					Reason: "runtime script failure",
				},
				Status: http.StatusInternalServerError,
			},
		},
	})
	if err == nil || err.Error() != "runtime script failure" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsurePartitionSearchResponseOKReturnsCausedByReason(t *testing.T) {
	err := ensurePartitionSearchResponseOK(&SearchResponse{
		ResponseBase: ResponseBase{
			StatusCode: http.StatusBadRequest,
			RawResult: &util.Result{
				Body: []byte(`{"error":{"root_cause":[{"reason":"compile error"}],"failed_shards":[{"reason":{"reason":"compile error","caused_by":{"reason":"static method [java.lang.Math, floorMod/2] not found"}}}],"reason":"all shards failed"},"status":400}`),
			},
		},
	})
	if err == nil || err.Error() != "static method [java.lang.Math, floorMod/2] not found" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func getMustClause(t *testing.T, filter util.MapStr) util.MapStr {
	t.Helper()
	boolFilter, ok := filter["bool"].(util.MapStr)
	if !ok {
		t.Fatalf("expected bool filter, got %v", filter)
	}
	must, ok := boolFilter["must"].([]interface{})
	if !ok || len(must) == 0 {
		t.Fatalf("expected must clauses, got %v", boolFilter["must"])
	}
	clause, ok := must[0].(util.MapStr)
	if !ok {
		t.Fatalf("expected util.MapStr clause, got %T", must[0])
	}
	return clause
}

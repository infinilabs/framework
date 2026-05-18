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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"infini.sh/framework/core/util"
)

type PartitionQuery struct {
	IndexName      string      `json:"index_name"`
	FieldType      string      `json:"field_type"`
	FieldName      string      `json:"field_name"`
	Strategy       string      `json:"strategy,omitempty"`
	Step           interface{} `json:"step,omitempty"`
	PartitionCount int         `json:"partition_count,omitempty"`
	Filter         interface{} `json:"filter"`
	DocType        string      `json:"doc_type"`
}

type PartitionInfo struct {
	Key    float64                `json:"key"`
	Start  float64                `json:"start"`
	End    float64                `json:"end"`
	Filter map[string]interface{} `json:"filter"`
	Docs   int64                  `json:"docs"`
	Label  string                 `json:"label,omitempty"`
	Values []string               `json:"values,omitempty"`
	Other  bool
}

type BoundValuesResult struct {
	Min             float64
	Max             float64
	Null            int64
	NotExistsFilter map[string]interface{}
}

const (
	PartitionByDate    = "date"
	PartitionByKeyword = "keyword"
	PartitionByNumber  = "number"

	PartitionStrategyStep     = "step"
	PartitionStrategyQuantile = "quantile"
	PartitionStrategyTerms    = "terms"
	PartitionStrategyHash     = "hash"
)

func GetPartitions(q *PartitionQuery, client API) ([]PartitionInfo, error) {
	if q == nil {
		return nil, fmt.Errorf("patition query can not be empty")
	}
	var (
		vFilter interface{}
		must    []interface{}
	)
	if q.Filter != nil {
		must = append(must, q.Filter)
	}
	if docType := strings.TrimSpace(q.DocType); docType != "" {
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"_type": util.MapStr{
					"value": docType,
				},
			},
		})
	}
	if len(must) > 0 {
		vFilter = util.MapStr{
			"bool": util.MapStr{
				"must": must,
			},
		}
	}

	switch q.FieldType {
	case PartitionByDate, PartitionByNumber:
		result, err := getBoundValues(client, q.IndexName, q.FieldName, vFilter)
		if err != nil {
			return nil, err
		}
		// empty data
		if result.Min == -1 {
			return nil, nil
		}

		var (
			partitions []PartitionInfo
		)

		switch normalizePartitionStrategy(q.Strategy) {
		case PartitionStrategyStep:
			step, err := parsePartitionStep(q.FieldType, q.Step)
			if err != nil {
				return nil, err
			}
			partitions, err = getPartitionsByAgg(client, q.IndexName, q.FieldName, q.FieldType, step, vFilter)
			if err != nil {
				return nil, err
			}
		case PartitionStrategyQuantile:
			partitions, err = getPartitionsByQuantile(client, q.IndexName, q.FieldName, q.FieldType, q.PartitionCount, result.Min, result.Max, vFilter)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported partition strategy: %s", q.Strategy)
		}

		if result.Null > 0 {
			partitions = append(partitions, PartitionInfo{
				Filter: result.NotExistsFilter,
				Other:  true,
				Label:  "Missing values",
				Docs:   result.Null,
			})
		}
		return partitions, nil
	case PartitionByKeyword:
		var (
			partitions []PartitionInfo
			err        error
		)
		switch normalizePartitionStrategy(q.Strategy) {
		case PartitionStrategyTerms:
			partitions, err = getPartitionsByTerms(client, q.IndexName, q.FieldName, q.PartitionCount, vFilter)
			if err != nil {
				return nil, err
			}
		case PartitionStrategyHash:
			partitions, err = getPartitionsByHash(client, q.IndexName, q.FieldName, q.PartitionCount, vFilter)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported partition strategy: %s", q.Strategy)
		}

		missingPartition, err := getMissingPartition(client, q.IndexName, q.FieldName, vFilter)
		if err != nil {
			return nil, err
		}
		if missingPartition != nil {
			partitions = append(partitions, *missingPartition)
		}
		return partitions, nil
	default:
		return nil, fmt.Errorf("unsupported field type: %s", q.FieldType)
	}
}

func normalizePartitionStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", PartitionStrategyStep:
		return PartitionStrategyStep
	case PartitionStrategyQuantile:
		return PartitionStrategyQuantile
	case PartitionStrategyTerms:
		return PartitionStrategyTerms
	case PartitionStrategyHash:
		return PartitionStrategyHash
	default:
		return strings.ToLower(strings.TrimSpace(strategy))
	}
}

func parsePartitionStep(fieldType string, stepValue interface{}) (float64, error) {
	if fieldType == PartitionByDate {
		stepV, ok := stepValue.(string)
		if !ok {
			return 0, fmt.Errorf("expect step value of string type since filedtype is %s", PartitionByDate)
		}
		du, err := util.ParseDuration(stepV)
		if err != nil {
			return 0, fmt.Errorf("parse step duration error: %w", err)
		}
		return float64(du.Milliseconds()), nil
	}

	switch stepValue.(type) {
	case float64:
		return stepValue.(float64), nil
	case string:
		v, err := strconv.Atoi(stepValue.(string))
		if err != nil {
			return 0, fmt.Errorf("convert step error: %w", err)
		}
		return float64(v), nil
	default:
		return 0, fmt.Errorf("invalid parameter step: %v", stepValue)
	}
}

func getPartitionsByAgg(client API, indexName string, fieldName, fieldType string, step float64, filter interface{}) ([]PartitionInfo, error) {
	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"partitions": util.MapStr{
				"histogram": util.MapStr{
					"field":    fieldName,
					"interval": step,
				},
				"aggs": util.MapStr{
					"min": util.MapStr{
						"min": util.MapStr{
							"field": fieldName,
						},
					},
					"max": util.MapStr{
						"max": util.MapStr{
							"field": fieldName,
						},
					},
				},
			},
		},
	}
	if filter != nil {
		queryDsl["query"] = filter
	}
	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}
	var partitions []PartitionInfo
	for _, bk := range res.Aggregations["partitions"].Buckets {
		var docCount float64
		docCount, ok := bk["doc_count"].(float64)
		if !ok || docCount == 0 {
			continue
		}
		var (
			min   float64
			max   float64
			key   float64
			minOK bool
			maxOK bool
		)
		if minM, ok := bk["min"].(map[string]interface{}); ok {
			min, minOK = minM["value"].(float64)
		}
		if maxM, ok := bk["max"].(map[string]interface{}); ok {
			max, maxOK = maxM["value"].(float64)
		}
		if keyM, ok := bk["key"].(float64); ok {
			key = keyM
		}
		if minOK && maxOK {
			partition := PartitionInfo{
				Key:   key,
				Start: min,
				End:   max,
				Docs:  int64(docCount),
				Other: false,
			}
			partition.Filter = buildBoundedPartitionFilter(min, max, fieldName, fieldType, filter)
			partitions = append(partitions, partition)
		}
	}
	return partitions, nil
}

func getPartitionsByQuantile(client API, indexName string, fieldName, fieldType string, partitionCount int, min, max float64, filter interface{}) ([]PartitionInfo, error) {
	if partitionCount <= 0 {
		return nil, fmt.Errorf("invalid parameter partition_count: %d", partitionCount)
	}

	boundaries, err := getQuantileBoundaries(client, indexName, fieldName, partitionCount, min, max, filter)
	if err != nil {
		return nil, err
	}
	partitions := buildQuantilePartitions(boundaries, fieldName, fieldType, filter)
	if len(partitions) == 0 {
		return nil, nil
	}

	counts, err := getPartitionDocCounts(client, indexName, partitions)
	if err != nil {
		return nil, err
	}

	filtered := make([]PartitionInfo, 0, len(partitions))
	for i := range partitions {
		partitions[i].Docs = counts[i]
		if partitions[i].Docs <= 0 {
			continue
		}
		filtered = append(filtered, partitions[i])
	}
	return filtered, nil
}

func getPartitionsByTerms(client API, indexName, fieldName string, partitionCount int, filter interface{}) ([]PartitionInfo, error) {
	if partitionCount <= 0 {
		return nil, fmt.Errorf("invalid parameter partition_count: %d", partitionCount)
	}

	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"partitions": util.MapStr{
				"terms": util.MapStr{
					"field": fieldName,
					"size":  partitionCount,
				},
			},
		},
	}
	if filter != nil {
		queryDsl["query"] = filter
	}

	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}

	var (
		partitions []PartitionInfo
		values     []string
	)
	if partitionsAgg, ok := res.Aggregations["partitions"]; ok {
		for idx, bucket := range partitionsAgg.Buckets {
			value := fmt.Sprintf("%v", bucket["key"])
			docCount := util.GetInt64Value(bucket["doc_count"])
			if docCount <= 0 {
				continue
			}
			values = append(values, value)
			partitions = append(partitions, PartitionInfo{
				Key:    float64(idx),
				Docs:   docCount,
				Label:  value,
				Values: []string{value},
				Filter: buildExactTermPartitionFilter(value, fieldName, filter),
			})
		}
	}

	sumOtherDocCount, _ := jsonparser.GetInt(res.RawResult.Body, "aggregations", "partitions", "sum_other_doc_count")
	if sumOtherDocCount > 0 {
		partitions = append(partitions, PartitionInfo{
			Key:    float64(len(partitions)),
			Docs:   sumOtherDocCount,
			Label:  "Other terms",
			Values: append([]string(nil), values...),
			Filter: buildOtherTermsPartitionFilter(values, fieldName, filter),
			Other:  true,
		})
	}

	return partitions, nil
}

func getPartitionsByHash(client API, indexName, fieldName string, partitionCount int, filter interface{}) ([]PartitionInfo, error) {
	if partitionCount <= 0 {
		return nil, fmt.Errorf("invalid parameter partition_count: %d", partitionCount)
	}

	partitions := make([]PartitionInfo, 0, partitionCount)
	for idx := 0; idx < partitionCount; idx++ {
		partitions = append(partitions, PartitionInfo{
			Key:    float64(idx),
			Label:  fmt.Sprintf("Hash %d/%d", idx+1, partitionCount),
			Filter: buildHashPartitionFilter(idx, partitionCount, fieldName, filter),
		})
	}

	counts, err := getPartitionDocCounts(client, indexName, partitions)
	if err != nil {
		return nil, err
	}

	filtered := make([]PartitionInfo, 0, len(partitions))
	for idx := range partitions {
		partitions[idx].Docs = counts[idx]
		if partitions[idx].Docs <= 0 {
			continue
		}
		filtered = append(filtered, partitions[idx])
	}
	return filtered, nil
}

func getQuantileBoundaries(client API, indexName, fieldName string, partitionCount int, min, max float64, filter interface{}) ([]float64, error) {
	percents := buildQuantilePercents(partitionCount)
	if len(percents) == 0 {
		return []float64{min, max}, nil
	}

	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"partition_percentiles": util.MapStr{
				"percentiles": util.MapStr{
					"field":    fieldName,
					"percents": percents,
					"keyed":    false,
				},
			},
		},
	}
	if filter != nil {
		queryDsl["query"] = filter
	}

	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}

	boundaries := make([]float64, 0, len(percents)+2)
	boundaries = append(boundaries, min)
	_, err = jsonparser.ArrayEach(res.RawResult.Body, func(value []byte, _ jsonparser.ValueType, _ int, err error) {
		if err != nil {
			return
		}
		boundary, parseErr := jsonparser.GetFloat(value, "value")
		if parseErr != nil || math.IsNaN(boundary) || math.IsInf(boundary, 0) {
			return
		}
		boundaries = append(boundaries, boundary)
	}, "aggregations", "partition_percentiles", "values")
	if err != nil {
		return nil, err
	}
	boundaries = append(boundaries, max)
	boundaries = dedupeSortedBoundaries(boundaries)
	if len(boundaries) == 1 {
		return []float64{boundaries[0], boundaries[0]}, nil
	}
	return boundaries, nil
}

func buildQuantilePercents(partitionCount int) []float64 {
	if partitionCount <= 1 {
		return nil
	}
	percents := make([]float64, 0, partitionCount-1)
	for i := 1; i < partitionCount; i++ {
		percents = append(percents, float64(i)*100/float64(partitionCount))
	}
	return percents
}

func dedupeSortedBoundaries(boundaries []float64) []float64 {
	if len(boundaries) == 0 {
		return nil
	}
	sort.Float64s(boundaries)
	result := make([]float64, 0, len(boundaries))
	for _, boundary := range boundaries {
		if len(result) == 0 || !sameBoundary(result[len(result)-1], boundary) {
			result = append(result, boundary)
		}
	}
	return result
}

func sameBoundary(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9
}

func buildQuantilePartitions(boundaries []float64, fieldName, fieldType string, filter interface{}) []PartitionInfo {
	if len(boundaries) < 2 {
		return nil
	}

	partitions := make([]PartitionInfo, 0, len(boundaries)-1)
	if len(boundaries) == 2 {
		partitions = append(partitions, PartitionInfo{
			Key:    boundaries[1],
			Start:  boundaries[0],
			End:    boundaries[1],
			Filter: buildOpenPartitionFilter(nil, nil, fieldName, fieldType, filter),
		})
		return partitions
	}

	for i := 1; i < len(boundaries); i++ {
		lower, upper := boundaries[i-1], boundaries[i]
		if sameBoundary(lower, upper) {
			continue
		}

		var lowerRef, upperRef *float64
		if i > 1 {
			lowerRef = &lower
		}
		if i < len(boundaries)-1 {
			upperRef = &upper
		}

		partitions = append(partitions, PartitionInfo{
			Key:    upper,
			Start:  lower,
			End:    upper,
			Filter: buildOpenPartitionFilter(lowerRef, upperRef, fieldName, fieldType, filter),
		})
	}
	return partitions
}

func getPartitionDocCounts(client API, indexName string, partitions []PartitionInfo) ([]int64, error) {
	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"partitions": util.MapStr{
				"filters": util.MapStr{
					"filters": buildPartitionFiltersMap(partitions),
				},
			},
		},
	}

	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}

	counts := make([]int64, 0, len(partitions))
	for i := range partitions {
		docCount, parseErr := jsonparser.GetInt(res.RawResult.Body, "aggregations", "partitions", "buckets", strconv.Itoa(i), "doc_count")
		if parseErr != nil {
			return nil, parseErr
		}
		counts = append(counts, docCount)
	}
	return counts, nil
}

func buildPartitionFiltersMap(partitions []PartitionInfo) util.MapStr {
	filters := util.MapStr{}
	for i, partition := range partitions {
		filters[strconv.Itoa(i)] = partition.Filter
	}
	return filters
}

func getMissingPartition(client API, indexName, fieldName string, filter interface{}) (*PartitionInfo, error) {
	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"missing_field": util.MapStr{
				"filter": buildMissingFieldCondition(fieldName),
			},
		},
	}
	if filter != nil {
		queryDsl["query"] = filter
	}

	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}

	docCount, err := jsonparser.GetInt(res.RawResult.Body, "aggregations", "missing_field", "doc_count")
	if err != nil || docCount <= 0 {
		return nil, err
	}

	return &PartitionInfo{
		Docs:   docCount,
		Label:  "Missing values",
		Filter: buildMissingFieldFilter(fieldName, filter),
		Other:  true,
	}, nil
}

// NOTE: we assume GetPartitions returned sorted buckets from ES, if not, we need to manually sort source & target partitions by keys
// sourcePartitions & targetPartitions must've been generated with same bucket step & offset
func MergePartitions(sourcePartitions []PartitionInfo, targetPartitions []PartitionInfo, fieldName, fieldType string, filter interface{}) []PartitionInfo {
	sourceIdx, targetIdx := 0, 0
	var ret []PartitionInfo
	for {
		if sourceIdx >= len(sourcePartitions) || targetIdx >= len(targetPartitions) {
			break
		}
		source := sourcePartitions[sourceIdx]
		target := targetPartitions[targetIdx]
		if source.Key < target.Key {
			ret = append(ret, source)
			sourceIdx += 1
			continue
		}
		if source.Key > target.Key {
			ret = append(ret, target)
			targetIdx += 1
			continue
		}
		partition := PartitionInfo{
			Key:   source.Key,
			Start: math.Min(source.Start, target.Start),
			End:   math.Max(source.End, target.End),
			// NOTE: not accurate, don't use
			Docs:  util.MaxInt64(source.Docs, target.Docs),
			Other: false,
		}
		partition.Filter = buildBoundedPartitionFilter(partition.Start, partition.End, fieldName, fieldType, filter)
		ret = append(ret, partition)
		sourceIdx += 1
		targetIdx += 1
	}
	for i := sourceIdx; i < len(sourcePartitions); i += 1 {
		ret = append(ret, sourcePartitions[i])
	}
	for i := targetIdx; i < len(targetPartitions); i += 1 {
		ret = append(ret, targetPartitions[i])
	}
	return ret
}

func buildBoundedPartitionFilter(min, max float64, fieldName, fieldType string, filter interface{}) util.MapStr {
	rv := util.MapStr{
		"gte": min,
		"lte": max,
	}
	if fieldType == PartitionByDate {
		rv["gte"] = normalizeDateRangeBoundary(min, true, true)
		rv["lte"] = normalizeDateRangeBoundary(max, false, true)
		rv["format"] = "epoch_millis"
	}
	must := []interface{}{
		util.MapStr{
			"range": util.MapStr{
				fieldName: rv,
			},
		},
	}
	if filter != nil {
		must = append(must, filter)
	}
	return util.MapStr{
		"bool": util.MapStr{
			"must": must,
		},
	}
}

func buildOpenPartitionFilter(lower, upper *float64, fieldName, fieldType string, filter interface{}) util.MapStr {
	rv := util.MapStr{}
	if lower != nil {
		rv["gt"] = *lower
	}
	if upper != nil {
		rv["lte"] = *upper
	}
	if fieldType == PartitionByDate {
		if lower != nil {
			rv["gt"] = normalizeDateRangeBoundary(*lower, true, false)
		}
		if upper != nil {
			rv["lte"] = normalizeDateRangeBoundary(*upper, false, true)
		}
		rv["format"] = "epoch_millis"
	}
	var condition interface{}
	if len(rv) == 0 || (len(rv) == 1 && rv["format"] != nil) {
		condition = util.MapStr{
			"exists": util.MapStr{
				"field": fieldName,
			},
		}
	} else {
		condition = util.MapStr{
			"range": util.MapStr{
				fieldName: rv,
			},
		}
	}
	must := []interface{}{condition}
	if filter != nil {
		must = append(must, filter)
	}
	return util.MapStr{
		"bool": util.MapStr{
			"must": must,
		},
	}

}

func normalizeDateRangeBoundary(value float64, lower, inclusive bool) int64 {
	switch {
	case lower && inclusive:
		return int64(math.Ceil(value))
	case lower && !inclusive:
		return int64(math.Floor(value))
	case !lower && inclusive:
		return int64(math.Floor(value))
	default:
		return int64(math.Ceil(value))
	}
}

func buildExactTermPartitionFilter(value, fieldName string, filter interface{}) util.MapStr {
	return buildMustPartitionFilter([]interface{}{
		util.MapStr{
			"term": util.MapStr{
				fieldName: util.MapStr{
					"value": value,
				},
			},
		},
	}, filter)
}

func buildOtherTermsPartitionFilter(values []string, fieldName string, filter interface{}) util.MapStr {
	boolFilter := util.MapStr{
		"must": []interface{}{
			util.MapStr{
				"exists": util.MapStr{
					"field": fieldName,
				},
			},
		},
	}
	if filter != nil {
		boolFilter["must"] = append(boolFilter["must"].([]interface{}), filter)
	}
	if len(values) > 0 {
		boolFilter["must_not"] = []interface{}{
			util.MapStr{
				"terms": util.MapStr{
					fieldName: values,
				},
			},
		}
	}
	return util.MapStr{
		"bool": boolFilter,
	}
}

func buildHashPartitionFilter(partitionID, partitionCount int, fieldName string, filter interface{}) util.MapStr {
	fieldLiteral := buildPainlessStringLiteral(fieldName)
	return buildMustPartitionFilter([]interface{}{
		util.MapStr{
			"script": util.MapStr{
				"script": util.MapStr{
					"lang":   "painless",
					"source": fmt.Sprintf("doc[%s].size()!=0 && doc[%s].value != '' && (((doc[%s].value.hashCode() %% params.partition_count) + params.partition_count) %% params.partition_count) == params.partition_id", fieldLiteral, fieldLiteral, fieldLiteral),
					"params": util.MapStr{
						"partition_count": partitionCount,
						"partition_id":    partitionID,
					},
				},
			},
		},
	}, filter)
}

func buildPainlessStringLiteral(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(value) + "'"
}

func searchPartitionWithRawQueryDSL(client API, indexName string, queryDsl util.MapStr) (*SearchResponse, error) {
	res, err := client.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(queryDsl))
	if err != nil {
		return nil, err
	}
	if err := ensurePartitionSearchResponseOK(res); err != nil {
		return nil, err
	}
	return res, nil
}

func ensurePartitionSearchResponseOK(res *SearchResponse) error {
	if res == nil {
		return errors.New("empty search response")
	}
	if res.StatusCode == 0 || res.StatusCode == http.StatusOK {
		return nil
	}
	if res.RawResult != nil && len(res.RawResult.Body) > 0 {
		for _, path := range [][]string{
			{"error", "failed_shards", "[0]", "reason", "caused_by", "reason"},
			{"error", "failed_shards", "[0]", "reason", "reason"},
			{"error", "root_cause", "[0]", "reason"},
			{"error", "reason"},
		} {
			if msg, ok := getJSONPathString(res.RawResult.Body, path...); ok && msg != "" {
				return errors.New(msg)
			}
		}
	}
	if msg := res.Error.Message(); msg != "" {
		return errors.New(msg)
	}
	if res.RawResult != nil && len(res.RawResult.Body) > 0 {
		return errors.New(string(res.RawResult.Body))
	}
	return fmt.Errorf("unexpected search status: %d", res.StatusCode)
}

func getJSONPathString(data []byte, path ...string) (string, bool) {
	v, err := jsonparser.GetString(data, path...)
	if err != nil {
		return "", false
	}
	return v, true
}

func buildMissingFieldCondition(fieldName string) util.MapStr {
	return util.MapStr{
		"bool": util.MapStr{
			"should": []interface{}{
				util.MapStr{
					"bool": util.MapStr{
						"must_not": []interface{}{
							util.MapStr{
								"exists": util.MapStr{
									"field": fieldName,
								},
							},
						},
					},
				},
				util.MapStr{
					"term": util.MapStr{
						fieldName: util.MapStr{
							"value": "",
						},
					},
				},
			},
			"minimum_should_match": 1,
		},
	}
}

func buildMissingFieldFilter(fieldName string, filter interface{}) util.MapStr {
	return buildMustPartitionFilter([]interface{}{
		buildMissingFieldCondition(fieldName),
	}, filter)
}

func buildMustPartitionFilter(mustClauses []interface{}, filter interface{}) util.MapStr {
	must := append([]interface{}{}, mustClauses...)
	if filter != nil {
		must = append(must, filter)
	}
	return util.MapStr{
		"bool": util.MapStr{
			"must": must,
		},
	}
}

func getBoundValues(client API, indexName string, fieldName string, filter interface{}) (*BoundValuesResult, error) {
	nullFilter := util.MapStr{
		"bool": util.MapStr{
			"must_not": []util.MapStr{
				{
					"exists": util.MapStr{
						"field": fieldName,
					},
				},
			},
		},
	}
	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"max_field_value": util.MapStr{
				"max": util.MapStr{
					"field": fieldName,
				},
			},
			"min_field_value": util.MapStr{
				"min": util.MapStr{
					"field": fieldName,
				},
			},
			"null_field_value": util.MapStr{
				"filter": nullFilter,
			},
		},
	}
	if filter != nil {
		queryDsl["query"] = filter
	}
	res, err := searchPartitionWithRawQueryDSL(client, indexName, queryDsl)
	if err != nil {
		return nil, err
	}
	result := BoundValuesResult{
		Min:  -1,
		Max:  -1,
		Null: -1,
	}
	if res.GetTotal() == 0 {
		return &result, nil
	}
	if maxFieldValue, ok := res.Aggregations["max_field_value"]; ok {
		if v, ok := maxFieldValue.Value.(float64); ok {
			result.Max = v
		}
	}
	if minFieldValue, ok := res.Aggregations["min_field_value"]; ok {
		if v, ok := minFieldValue.Value.(float64); ok {
			result.Min = v
		}
	}
	result.Null, _ = jsonparser.GetInt(res.RawResult.Body, "aggregations", "null_field_value", "doc_count")
	if result.Null > 0 {
		result.NotExistsFilter = nullFilter
	}
	return &result, nil
}

func GetIndexTypes(client API, indexName string) (map[string]interface{}, error) {
	version := client.GetMajorVersion()
	if version >= 8 {
		return map[string]interface{}{}, nil
	}
	return getIndexTypes(client, indexName)
}

func getIndexTypes(client API, indexName string) (map[string]interface{}, error) {
	query := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"group_by_index": util.MapStr{
				"terms": util.MapStr{
					"field": "_index",
					"size":  500,
				},
				"aggs": util.MapStr{
					"group_by_type": util.MapStr{
						"terms": util.MapStr{
							"field": "_type",
							"size":  20,
						},
					},
				},
			},
		},
	}
	searchRes, err := client.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(query))
	if err != nil {
		return nil, err
	}
	if searchRes.StatusCode != http.StatusOK {
		return nil, errors.New(string(searchRes.RawResult.Body))
	}
	typeInfo := map[string]interface{}{}
	if indexAggs, ok := searchRes.Aggregations["group_by_index"]; ok {
		for _, bk := range indexAggs.Buckets {
			if iname, ok := bk["key"].(string); ok {
				info := map[string]interface{}{}
				if typeAggs, ok := bk["group_by_type"].(map[string]interface{}); ok {
					if bks, ok := typeAggs["buckets"].([]interface{}); ok {
						for _, sbk := range bks {
							if sbkM, ok := sbk.(map[string]interface{}); ok {
								if key, ok := sbkM["key"].(string); ok {
									info[key] = sbkM["doc_count"]
								}
							}
						}
					}
				}
				typeInfo[iname] = info
			}
		}
	}
	return typeInfo, nil
}

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
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/rubyniu105/framework/core/util"
)

type PartitionQuery struct {
	IndexName string      `json:"index_name"`
	FieldType string      `json:"field_type"`
	FieldName string      `json:"field_name"`
	Step      interface{} `json:"step"`
	Filter    interface{} `json:"filter"`
	DocType   string      `json:"doc_type"`
}

type PartitionInfo struct {
	Key    float64                `json:"key"`
	Start  float64                `json:"start"`
	End    float64                `json:"end"`
	Filter map[string]interface{} `json:"filter"`
	Docs   int64                  `json:"docs"`
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
		var step float64
		if q.FieldType == PartitionByDate {
			if stepV, ok := q.Step.(string); !ok {
				return nil, fmt.Errorf("expect step value of string type since filedtype is %s", PartitionByDate)
			} else {
				du, err := util.ParseDuration(stepV)
				if err != nil {
					return nil, fmt.Errorf("parse step duration error: %w", err)
				}
				step = float64(du.Milliseconds())
			}
		} else {
			switch q.Step.(type) {
			case float64:
				step = q.Step.(float64)
			case string:
				v, err := strconv.Atoi(q.Step.(string))
				if err != nil {
					return nil, fmt.Errorf("convert step error: %w", err)
				}
				step = float64(v)
			default:
				return nil, fmt.Errorf("invalid parameter step: %v", q.Step)
			}
		}

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
		partitions, err = getPartitionsByAgg(client, q.IndexName, q.FieldName, q.FieldType, step, vFilter)
		if err != nil {
			return nil, err
		}
		if result.Null > 0 {
			partitions = append(partitions, PartitionInfo{
				Filter: result.NotExistsFilter,
				Other:  true,
				Docs:   result.Null,
			})
		}
		return partitions, nil
	default:
		return nil, fmt.Errorf("unsupported field type: %s", q.FieldType)
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
	res, err := client.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(queryDsl))
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
			partition.Filter = buildPartitionFilter(min, max, fieldName, fieldType, filter)
			partitions = append(partitions, partition)
		}
	}
	return partitions, nil
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
		partition.Filter = buildPartitionFilter(partition.Start, partition.End, fieldName, fieldType, filter)
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

func buildPartitionFilter(min, max float64, fieldName, fieldType string, filter interface{}) util.MapStr {
	rv := util.MapStr{
		"gte": min,
		"lte": max,
	}
	if fieldType == PartitionByDate {
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
	res, err := client.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(queryDsl))
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
		return nil, fmt.Errorf(string(searchRes.RawResult.Body))
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

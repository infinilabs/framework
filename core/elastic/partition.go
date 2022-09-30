/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"fmt"
	"infini.sh/framework/core/util"
	"time"
)

type PartitionQuery struct {
	IndexName string `json:"index_name"`
	FieldType string `json:"field_type"`
	FieldName string `json:"field_name"`
	Step      interface{}    `json:"step"`
	Filter interface{} `json:"filter"`
}

type PartitionInfo struct {
	Start float64
	End float64
	Filter map[string]interface{}
	Docs int64
}

const (
	PartitionByDate = "date"
	PartitionByKeyword = "keyword"
	PartitionByNumber = "number"
)

func GetPartitions(q *PartitionQuery, client API)([]PartitionInfo, error){
	var (
		vFilter map[string]interface{}
		ok bool
	)
	if vFilter, ok = q.Filter.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("got wrong type of filter: %v", q.Filter)
	}
	switch q.FieldType {
	case PartitionByDate, PartitionByNumber:
		var step float64
		if q.FieldType == PartitionByDate {
			if stepV, ok := q.Step.(string); !ok {
				return nil, fmt.Errorf("expect step value of string type since filedtype is %s", PartitionByDate)
			}else{
				du, err := time.ParseDuration(stepV)
				if err != nil {
					return nil, fmt.Errorf("parse step duration error: %w", err)
				}
				step = float64(du.Milliseconds())
			}
		}else {
			if stepV, ok := q.Step.(float64); !ok {
				return nil, fmt.Errorf("expect step value of number type since filedtype is %s", PartitionByNumber)
			}else{
				step = stepV
			}
		}

		min, max, err := getBoundValues(client, q.IndexName, q.FieldName, vFilter)
		if err != nil {
			return nil, err
		}
		// empty data
		if min == -1 {
			return nil, nil
		}
		var (
			start = min
			end = start + step
			partitions []PartitionInfo
		)
		for {
			query :=  util.MapStr{
				"bool": util.MapStr{
					"must": []util.MapStr{
						vFilter,
						{
							"range": util.MapStr{
								q.FieldName: util.MapStr{
									"gte": start,
									"lt": end,
								},
							},
						},
					},
				},
			}
			queryDsl := util.MapStr{
				"query": query,
			}

			docCount, err := getPartitionDocCount(client, q.IndexName, queryDsl)
			if err != nil {
				return nil, fmt.Errorf("get partition doc count error: %w", err)
			}
			partitions = append(partitions, PartitionInfo{
				Start: start,
				End: end,
				Filter: query,
				Docs: docCount,
			})

			if end >= max {
				break
			}
			start = end
			end = start + step
			if end >= max {
				end = max + 1
			}
		}
		return partitions, nil
	default:
		return nil, fmt.Errorf("unsupported field type: %s", q.FieldType)
	}
}

func getPartitionDocCount(client API, indexName string, queryDsl interface{}) (int64 , error) {
	res, err := client.Count(indexName, util.MustToJSONBytes(queryDsl))
	if err != nil {
		return 0, err
	}
	return res.Count, nil
}

func getBoundValues(client API, indexName string, fieldName string, filter interface{}) (float64, float64, error) {
	queryDsl := util.MapStr{
		"query": filter,
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
		},
	}
	res, err := client.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(queryDsl))
	if err != nil {
		return 0, 0, err
	}
	if res.GetTotal() == 0 {
		return -1, -1, nil
	}
	var (
		max float64
		min float64
	)
	if maxFieldValue, ok := res.Aggregations["max_field_value"]; ok {
		if v, ok := maxFieldValue.Value.(float64); ok {
			max = v
		}
	}
	if minFieldValue, ok := res.Aggregations["min_field_value"]; ok {
		if v, ok := minFieldValue.Value.(float64); ok {
			min = v
		}
	}
	return min, max, nil
}

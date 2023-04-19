/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"fmt"
	"github.com/buger/jsonparser"
	"infini.sh/framework/core/util"
	"net/http"
	"strconv"
	"strings"
)

type PartitionQuery struct {
	IndexName string `json:"index_name"`
	FieldType string `json:"field_type"`
	FieldName string `json:"field_name"`
	Step      interface{}    `json:"step"`
	Filter interface{} `json:"filter"`
	DocType string `json:"doc_type"`
}

type PartitionInfo struct {
	Start float64 `json:"start"`
	End float64 `json:"end"`
	Filter map[string]interface{} `json:"filter"`
	Docs int64 `json:"docs"`
	Other bool
}

type BoundValuesResult struct {
	Min float64
	Max float64
	Null int64
	NotExistsFilter map[string]interface{}
}

const (
	PartitionByDate = "date"
	PartitionByKeyword = "keyword"
	PartitionByNumber = "number"
)

func GetPartitions(q *PartitionQuery, client API)([]PartitionInfo, error){
	if q == nil {
		return nil, fmt.Errorf("patition query can not be empty")
	}
	var (
		vFilter interface{}
		must []interface{}
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
			}else{
				du, err := util.ParseDuration(stepV)
				if err != nil {
					return nil, fmt.Errorf("parse step duration error: %w", err)
				}
				step = float64(du.Milliseconds())
			}
		}else {
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
				Other: true,
				Docs: result.Null,
			})
		}
		return partitions, nil
	default:
		return nil, fmt.Errorf("unsupported field type: %s", q.FieldType)
	}
}

func getPartitionsByAgg(client API, indexName string, fieldName, fieldType string, step float64, filter interface{}) ([]PartitionInfo, error){
	queryDsl := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"partitions": util.MapStr{
				"histogram": util.MapStr{
					"field": fieldName,
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
			min float64
			max float64
			minOK bool
			maxOK bool
		)
		if minM, ok := bk["min"].(map[string]interface{}); ok {
			min, minOK = minM["value"].(float64)
		}
		if maxM, ok := bk["max"].(map[string]interface{}); ok {
			max, maxOK = maxM["value"].(float64)
		}
		if minOK && maxOK {
			partition := PartitionInfo{
				Start: min,
				End: max,
				Docs: int64(docCount),
				Other: false,
			}
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
			partition.Filter = util.MapStr{
				"bool": util.MapStr{
					"must": must,
				},
			}
			partitions = append(partitions, partition)
		}
	}
	return partitions, nil
}

func getBoundValues(client API, indexName string, fieldName string, filter interface{}) (*BoundValuesResult, error) {
	nullFilter := util.MapStr{
		"bool": util.MapStr{
			"must_not":[]util.MapStr{
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
		Min: -1,
		Max: -1,
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

func GetIndexTypes( client API, indexName string) (map[string]interface{} , error) {
	version := client.GetMajorVersion()
	if version >= 8{
		return map[string]interface{}{}, nil
	}
	return getIndexTypes(client, indexName)
}

func getIndexTypes( client API, indexName string) (map[string]interface{} , error) {
	query := util.MapStr{
		"size": 0,
			"aggs": util.MapStr{
			"group_by_index": util.MapStr{
				"terms": util.MapStr{
					"field": "_index",
					"size": 500,
				},
				"aggs": util.MapStr{
					"group_by_type": util.MapStr{
						"terms": util.MapStr{
							"field": "_type",
								"size": 20,
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
			if iname, ok := bk["key"].(string); ok{
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
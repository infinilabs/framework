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
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"strings"
)

func ToDSL(q *orm.QueryBuilder) map[string]interface{} {
	dsl := map[string]interface{}{
		"query": clauseToDSL(q.Root()),
	}

	if q.FromVal() > 0 {
		dsl["from"] = q.FromVal()
	}
	if q.SizeVal() > 0 {
		dsl["size"] = q.SizeVal()
	}
	if len(q.Sorts()) > 0 {
		var sortList []interface{}
		for _, s := range q.Sorts() {
			sortList = append(sortList, map[string]interface{}{
				s.Field: map[string]interface{}{
					"order": string(s.SortType), // correct usage of your SortType
				},
			})
		}
		dsl["sort"] = sortList
	}

	return dsl
}

func flattenClause(clause *orm.Clause) []*orm.Clause {
	if clause.Query != nil {
		// Leaf: no flattening needed
		return []*orm.Clause{clause}
	}

	// Recurse and flatten subclauses if same bool type
	var flattened []*orm.Clause
	for _, sub := range clause.SubClauses {
		if sub.Query == nil && sub.BoolType == clause.BoolType {
			flattened = append(flattened, flattenClause(sub)...)
		} else {
			flattened = append(flattened, sub)
		}
	}
	return flattened
}
func clauseToDSL(clause *orm.Clause) map[string]interface{} {
	if clause.Query != nil {
		switch clause.Query.Operator {
		case orm.QueryMatch:
			return map[string]interface{}{
				"match": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryMultiMatch:
			return map[string]interface{}{
				"multi_match": map[string]interface{}{
					"fields": strings.Split(clause.Query.Field, ","),
					"query":  clause.Query.Value,
				},
			}
		case orm.QueryMatchPhrase:

			if clause.Query.Parameters != nil && len(clause.Query.Parameters.Data) > 0 {
				v := util.MapStr{
					"query": clause.Query.Value,
				}

				for k1, v1 := range clause.Query.Parameters.Data {
					v[k1] = v1
				}

				q := util.MapStr{
					"match_phrase": util.MapStr{
						clause.Query.Field: v,
					},
				}

				return q
			} else {
				return map[string]interface{}{
					"match_phrase": map[string]interface{}{
						clause.Query.Field: clause.Query.Value,
					},
				}
			}
		case orm.QueryTerm:
			return map[string]interface{}{
				"term": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryTerms:
			return map[string]interface{}{
				"terms": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryPrefix:
			return map[string]interface{}{
				"prefix": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryWildcard:
			return map[string]interface{}{
				"wildcard": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryRegexp:
			return map[string]interface{}{
				"regexp": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryFuzzy:

			if clause.Query.Parameters != nil && len(clause.Query.Parameters.Data) > 0 {
				v := util.MapStr{
					"value": clause.Query.Value,
				}

				for k1, v1 := range clause.Query.Parameters.Data {
					v[k1] = v1
				}

				q := util.MapStr{
					"fuzzy": util.MapStr{
						clause.Query.Field: v,
					},
				}

				return q
			} else {
				return map[string]interface{}{
					"fuzzy": map[string]interface{}{
						clause.Query.Field: clause.Query.Value,
					},
				}
			}
		case orm.QueryExists:
			return map[string]interface{}{
				"exists": map[string]interface{}{
					"field": clause.Query.Field,
				},
			}
		case orm.QueryIn:
			return map[string]interface{}{
				"terms": map[string]interface{}{
					clause.Query.Field: clause.Query.Value,
				},
			}
		case orm.QueryNotIn:
			return map[string]interface{}{
				"bool": map[string]interface{}{
					"must_not": []interface{}{
						map[string]interface{}{
							"terms": map[string]interface{}{
								clause.Query.Field: clause.Query.Value,
							},
						},
					},
				},
			}
		case orm.RangeGte:
			return map[string]interface{}{
				"range": map[string]interface{}{
					clause.Query.Field: map[string]interface{}{
						"gte": clause.Query.Value,
					},
				},
			}
		case orm.RangeLte:
			return map[string]interface{}{
				"range": map[string]interface{}{
					clause.Query.Field: map[string]interface{}{
						"lte": clause.Query.Value,
					},
				},
			}
		case orm.QueryRangeGt:
			return map[string]interface{}{
				"range": map[string]interface{}{
					clause.Query.Field: map[string]interface{}{
						"gt": clause.Query.Value,
					},
				},
			}
		case orm.QueryRangeLt:
			return map[string]interface{}{
				"range": map[string]interface{}{
					clause.Query.Field: map[string]interface{}{
						"lt": clause.Query.Value,
					},
				},
			}
		default:
			panic("unsupported operator: " + string(clause.Query.Operator))
		}
	}

	// If it's a bool clause, handle recursively
	subClauses := flattenClause(clause)

	var children []interface{}
	for _, sub := range subClauses {
		children = append(children, clauseToDSL(sub))
	}

	return map[string]interface{}{
		"bool": map[string]interface{}{
			string(clause.BoolType): children,
		},
	}
}

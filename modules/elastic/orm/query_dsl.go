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
	"strings"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

func BuildQueryDSLOnTopOfDSL(q *orm.QueryBuilder, reqBody []byte) map[string]interface{} {
	final := make(map[string]interface{})

	var body map[string]interface{}

	////TODO verify the Query DSL
	//searchRequest := elastic.SearchRequest{}
	//if  len(reqBody) > 0 {
	//	err := util.FromJSONBytes(reqBody, &searchRequest)
	//	if err != nil {
	//		panic(errors.Errorf("invalid query dsl"))
	//	}
	//}else{
	//	return final
	//}

	if err := util.FromJSONBytes(reqBody, &body); err != nil {
		body = make(map[string]interface{})
	}

	// Extract or initialize the query from body
	var bodyQuery map[string]interface{}
	if raw, ok := body["query"]; ok {
		bodyQuery, _ = raw.(map[string]interface{})
	}

	// Generate query from query string
	stringDSL := BuildQueryDSL(q)

	// Merge logic
	mergedQuery := mergeQueries(bodyQuery, stringDSL["query"].(map[string]interface{}))
	final["query"] = mergedQuery

	// Copy other fields from body
	for k, v := range body {
		if k == "query" {
			continue // already merged
		}
		final[k] = v
	}

	// Allow string DSL (query string) to override fields like size, from, etc.
	for k, v := range stringDSL {
		if k != "query" {
			final[k] = v
		}
	}
	return final
}

func mergeQueries(a, b map[string]interface{}) map[string]interface{} {
	// If either is nil, use the other
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	// Neither is bool, wrap both into a must
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []interface{}{a, b},
		},
	}
}

func BuildQueryDSLOnTopOfDSL2(q *orm.QueryBuilder, reqBody []byte) map[string]interface{} {
	// Parse user body
	var original map[string]interface{}
	_ = util.FromJSONBytes(reqBody, &original)

	// Build full DSL from query string
	override := BuildQueryDSL(q)

	// Extract the override query from query string
	overrideQuery, hasOverrideQuery := override["query"]

	// Extract original query
	var baseQuery map[string]interface{}
	switch raw := original["query"].(type) {
	case map[string]interface{}:
		// if already a bool query
		if rawBool, ok := raw["bool"].(map[string]interface{}); ok {
			baseQuery = rawBool
		} else {
			// wrap original query
			baseQuery = map[string]interface{}{
				"must": []interface{}{raw},
			}
		}
	case nil:
		baseQuery = map[string]interface{}{}
	default:
		// wrap raw value
		baseQuery = map[string]interface{}{
			"must": []interface{}{raw},
		}
	}
	// Merge override query into filter
	if hasOverrideQuery {
		if boolPart, ok := overrideQuery.(map[string]interface{})["bool"].(map[string]interface{}); ok {
			// Extract the filters we want to inject
			if filters, ok := boolPart["filter"].([]interface{}); ok {
				if origFilters, ok := baseQuery["filter"].([]interface{}); ok {
					baseQuery["filter"] = append(origFilters, filters...)
				} else {
					baseQuery["filter"] = filters
				}
			} else {
				// fallback: treat override query as standalone filter
				baseQuery["filter"] = append(toInterfaceSlice(baseQuery["filter"]), overrideQuery)
			}
		} else {
			// override is not bool — inject into filter
			baseQuery["filter"] = append(toInterfaceSlice(baseQuery["filter"]), overrideQuery)
		}
	}

	// Now rebuild final DSL
	final := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": baseQuery,
		},
	}

	// Merge non-query fields from override DSL (from query string)
	for k, v := range override {
		if k == "query" {
			continue
		}
		final[k] = v
	}

	// Copy all extra fields from original req body (user-supplied) if not already set
	for k, v := range original {
		if _, exists := final[k]; !exists {
			final[k] = v
		}
	}

	return final
}

func toInterfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return []interface{}{v}
}

func BuildQueryDSL(q *orm.QueryBuilder) map[string]interface{} {

	q.Build()

	//assemble final query dsl
	dsl := map[string]interface{}{}

	query := clauseToDSL(q.Root())

	if query != nil {
		dsl["query"] = flattenBoolClauses(query)
	}

	if q.FromVal() > 0 {
		dsl["from"] = q.FromVal()
	}

	if q.SizeVal() > 0 {
		dsl["size"] = q.SizeVal()
	}

	if q.CollapseVal() != "" {
		dsl["collapse"] = map[string]interface{}{
			"field": q.CollapseVal(),
		}
	}

	if len(q.IncludesVal()) > 0 || len(q.ExcludesVal()) > 0 {
		sources := util.MapStr{}
		if len(q.IncludesVal()) > 0 {
			sources["includes"] = q.IncludesVal()
		}
		if len(q.ExcludesVal()) > 0 {
			sources["excludes"] = q.ExcludesVal()
		}
		dsl["_source"] = sources
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
	if len(q.Aggs) > 0 {
		aggBuilder := NewAggreationBuilder()
		aggs, err := aggBuilder.Build(q.Aggs)
		if err != nil {
			panic("failed to build aggregations: " + err.Error())
		}
		dsl["aggs"] = aggs
	}

	return dsl
}

func flattenBoolClauses(m map[string]interface{}) map[string]interface{} {
	boolClause, ok := m["bool"].(map[string]interface{})
	if !ok {
		return m
	}

	result := map[string][]interface{}{
		"filter":   {},
		"must":     {},
		"should":   {},
		"must_not": {},
	}

	for _, key := range []string{"filter", "must", "should", "must_not"} {
		items, ok := boolClause[key].([]interface{})
		if !ok {
			continue
		}

		for _, item := range items {
			itemMap, isMap := item.(map[string]interface{})
			if !isMap {
				result[key] = append(result[key], item)
				continue
			}

			// Recursively flatten child clauses
			itemMap = flattenBoolClauses(itemMap)

			// Check if it's a nested bool clause
			if innerBool, ok := itemMap["bool"].(map[string]interface{}); ok {
				// Only flatten if the inner bool has **only the same clause type**
				// and no additional parameters like minimum_should_match, boost, etc.
				if len(innerBool) == 1 {
					if innerItems, ok := innerBool[key].([]interface{}); ok {
						if key != "must_not" {
							result[key] = append(result[key], innerItems...)
							continue
						}
					}
				}
			}

			// Default: keep as-is
			result[key] = append(result[key], itemMap)
		}
	}

	// Keep additional parameters from original bool
	boolOut := map[string]interface{}{}
	for k, v := range result {
		if len(v) > 0 {
			boolOut[k] = v
		}
	}

	// Copy over any remaining non-clause keys from the original bool (e.g., minimum_should_match)
	for k, v := range boolClause {
		if _, known := result[k]; !known {
			boolOut[k] = v
		}
	}

	return map[string]interface{}{
		"bool": boolOut,
	}
}

func clauseToDSL(clause *orm.Clause) map[string]interface{} {
	// Leaf node
	if clause.IsLeaf() {
		return buildLeafQuery(clause)
	}

	boolMap := make(map[string][]interface{})

	for _, sub := range clause.FilterClauses {
		if dsl := clauseToDSL(sub); dsl != nil {
			boolMap["filter"] = append(boolMap["filter"], dsl)
		}
	}

	for _, sub := range clause.MustClauses {
		if dsl := clauseToDSL(sub); dsl != nil {
			boolMap["must"] = append(boolMap["must"], dsl)
		}
	}
	for _, sub := range clause.ShouldClauses {
		if dsl := clauseToDSL(sub); dsl != nil {
			boolMap["should"] = append(boolMap["should"], dsl)
		}
	}
	for _, sub := range clause.MustNotClauses {
		if dsl := clauseToDSL(sub); dsl != nil {
			// Flatten nested must_not
			if inner, ok := dsl["bool"].(map[string]interface{}); ok {
				if nested, ok := inner["must_not"].([]interface{}); ok && len(inner) == 1 {
					boolMap["must_not"] = append(boolMap["must_not"], nested...)
					continue
				}
			}
			boolMap["must_not"] = append(boolMap["must_not"], dsl)
		}
	}

	finalBoolMap := toMapInterface(boolMap)
	if clause.Boost > 0 {
		finalBoolMap["boost"] = clause.Boost
	}

	if clause.Parameters != nil {
		for k, v := range clause.Parameters.Data {
			finalBoolMap[k] = v
		}
	}

	return map[string]interface{}{
		"bool": finalBoolMap,
	}
}

func toMapInterface(m map[string][]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range m {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

func buildLeafQuery(clause *orm.Clause) map[string]interface{} {
	field := clause.Field
	value := clause.Value
	params := clause.Parameters
	boost := clause.Boost

	addBoost := func(m map[string]interface{}) map[string]interface{} {
		if boost > 0 {
			m["boost"] = boost
		}
		if params != nil {
			for k, v := range params.Data {
				m[k] = v
			}
		}
		return m
	}

	switch clause.Operator {
	case orm.QueryMatch:
		return map[string]interface{}{"match": map[string]interface{}{field: addBoost(map[string]interface{}{"query": value})}}

	case orm.QueryMatchPhrase:
		return map[string]interface{}{"match_phrase": map[string]interface{}{field: addBoost(map[string]interface{}{"query": value})}}

	case orm.QueryMultiMatch:
		// field = "title,category" → split into []string
		fields := strings.Split(field, ",")
		m := map[string]interface{}{
			"query":  value,
			"fields": fields,
		}
		if boost > 0 {
			m["boost"] = boost
		}
		if params != nil {
			for k, v := range params.Data {
				m[k] = v
			}
		}
		return map[string]interface{}{
			"multi_match": m,
		}
	case orm.QueryTerm:
		return map[string]interface{}{"term": map[string]interface{}{field: addBoost(map[string]interface{}{"value": value})}}

	case orm.QueryTerms, orm.QueryIn:
		return map[string]interface{}{"terms": map[string]interface{}{field: value}}

	case orm.QueryNotIn:
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": []interface{}{
					map[string]interface{}{"terms": map[string]interface{}{field: value}},
				},
			},
		}

	case orm.QueryPrefix:
		return map[string]interface{}{"prefix": map[string]interface{}{field: addBoost(map[string]interface{}{"value": value})}}

	case orm.QueryWildcard:
		return map[string]interface{}{"wildcard": map[string]interface{}{field: value}}

	case orm.QueryRegexp:
		return map[string]interface{}{"regexp": map[string]interface{}{field: value}}

	case orm.QueryExists:
		return map[string]interface{}{"exists": map[string]interface{}{"field": field}}

	case orm.QueryFuzzy:
		m := map[string]interface{}{"value": value}
		if params != nil {
			for k, val := range params.Data {
				m[k] = val
			}
		}
		if boost > 0 {
			m["boost"] = boost
		}
		return map[string]interface{}{"fuzzy": map[string]interface{}{field: m}}

	case orm.QueryRangeGte:
		return map[string]interface{}{
			"range": map[string]interface{}{field: addBoost(map[string]interface{}{"gte": value})},
		}
	case orm.QueryRangeLte:
		return map[string]interface{}{
			"range": map[string]interface{}{field: addBoost(map[string]interface{}{"lte": value})},
		}
	case orm.QueryRangeGt:
		return map[string]interface{}{
			"range": map[string]interface{}{field: addBoost(map[string]interface{}{"gt": value})},
		}
	case orm.QueryRangeLt:
		return map[string]interface{}{
			"range": map[string]interface{}{field: addBoost(map[string]interface{}{"lt": value})},
		}
	case orm.QueryQueryString:
		m := map[string]interface{}{
			"query":  value,
			"fields": strings.Split(field, ","),
		}
		if params != nil {
			for k, val := range params.Data {
				m[k] = val
			}
		}
		return map[string]interface{}{
			"query_string": m,
		}

	case orm.QuerySemantic:
		// Build semantic query DSL
		// {"semantic": {"field": {"query_text": "...", "candidates": 10, "query_strategy": "LSH_COSINE"}}}
		m := map[string]interface{}{}
		if params != nil {
			// Copy all parameters (query_text, candidates, query_strategy)
			for k, val := range params.Data {
				m[k] = val
			}
		}
		return map[string]interface{}{
			"semantic": map[string]interface{}{
				field: m,
			},
		}

	case orm.QueryHybrid:
		// Build hybrid query DSL
		// {"hybrid": {"queries": [...]}}
		queries, ok := value.([]*orm.Clause)
		if !ok {
			panic("hybrid query value must be []*orm.Clause")
		}
		queryList := []interface{}{}
		for _, q := range queries {
			queryList = append(queryList, clauseToDSL(q))
		}
		return map[string]interface{}{
			"hybrid": map[string]interface{}{
				"queries": queryList,
			},
		}

	case orm.QueryNested:
		// Build nested query DSL
		// {"nested": {"path": "...", "query": {...}}}
		nestedQuery, ok := value.(*orm.Clause)
		if !ok {
			panic("nested query value must be *orm.Clause")
		}
		m := map[string]interface{}{
			"path":  params.Data["path"],
			"query": clauseToDSL(nestedQuery),
		}
		return map[string]interface{}{
			"nested": m,
		}

	default:
		panic("unsupported operator: " + string(clause.Operator))
	}
}

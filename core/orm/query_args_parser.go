package orm

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func NewQueryBuilderFromRequest(req *http.Request, defaultField ...string) (*QueryBuilder, error) {
	q := req.URL.Query()

	builder := NewQuery()
	if fuzziness := q.Get("fuzziness"); fuzziness != "" {
		if v, err := strconv.Atoi(fuzziness); err == nil && v >= 0 && v <= 5 {
			builder.fuzziness = v
		}
	}

	if defaultOp := q.Get("default_operator"); defaultOp != "" {
		builder.defaultOperator = defaultOp
	}

	defaultFields := []string{}
	if reqFields := q.Get("default_fields"); reqFields != "" {
		defaultFields = strings.Split(reqFields, ",")
	}

	if reqFields := q.Get("_source_includes"); reqFields != "" {
		builder.Include(strings.Split(reqFields, ",")...)
	}

	if reqFields := q.Get("_source_excludes"); reqFields != "" {
		builder.Exclude(strings.Split(reqFields, ",")...)
	}

	if reqQueryFields := q.Get("default_query_fields"); reqQueryFields != "" {
		builder.defaultQueryFields = strings.Split(reqQueryFields, ",")
	}

	if reqFilterFields := q.Get("default_filter_fields"); reqFilterFields != "" {
		builder.defaultFilterFields = strings.Split(reqFilterFields, ",")
	}

	//only if user didn't pass default fields, if user do, use user's value only
	if len(defaultField) > 0 && len(defaultFields) == 0 {
		defaultFields = append(defaultFields, defaultField...)
	}

	//user didn't specify query fields, use default fields
	if len(builder.defaultQueryFields) == 0 && len(defaultFields) > 0 {
		builder.defaultQueryFields = defaultFields
	}

	//user didn't specify filter fields, use default fields
	if len(builder.defaultFilterFields) == 0 && len(defaultFields) > 0 {
		builder.defaultFilterFields = defaultFields
	}

	// Handle full text or term query
	queryStr := q.Get("query")
	if queryStr != "" {
		builder.Query(queryStr)
	}

	// Handle filters (supporting NOT with '-' prefix)
	for _, filterRaw := range q["filter"] {
		filterStr, err := url.QueryUnescape(filterRaw)
		if err != nil {
			filterStr = filterRaw // fallback if invalid encoding
		}

		clause, err := parseFilterToClause(builder.defaultFilterFields, filterStr)
		if err != nil {
			return nil, err
		}

		builder.Must(clause)
	}

	// Handle sorting
	sortStr := q.Get("sort")
	if sortStr != "" {
		var sorts []Sort
		parts := strings.Split(sortStr, ",")
		for _, part := range parts {
			kv := strings.SplitN(part, ":", 2)

			field := kv[0]
			var order string

			if len(kv) == 2 {
				order = strings.ToLower(kv[1])
				if order != "asc" && order != "desc" {
					order = "" // force fallback
				}
			}

			// Apply smart default if order is missing or invalid
			if order == "" {
				if field == "_score" {
					order = "desc"
				} else {
					order = "asc"
				}
			}

			sorts = append(sorts, Sort{Field: field, SortType: SortType(order)})
		}
		builder.SortBy(sorts...)
	}

	// Pagination example: from and size
	if fromStr := q.Get("from"); fromStr != "" {
		if from, err := strconv.Atoi(fromStr); err == nil {
			builder.From(from)
		}
	}
	if sizeStr := q.Get("size"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil {
			builder.Size(size)
		}
	}

	return builder, nil
}

func parseFilterToClause(defaultFields []string, filterStr string) (*Clause, error) {
	filterStr = strings.TrimSpace(filterStr)

	negate := false
	if strings.HasPrefix(filterStr, "-") || strings.HasPrefix(filterStr, "!") {
		negate = true
		filterStr = strings.TrimSpace(filterStr[1:])
	}

	// Handle exists(field)
	if strings.HasPrefix(filterStr, "exists(") && strings.HasSuffix(filterStr, ")") {
		field := strings.TrimSuffix(strings.TrimPrefix(filterStr, "exists("), ")")
		clause := ExistsQuery(field)
		if negate {
			return MustNotQuery(clause), nil
		}
		return clause, nil
	}

	// Check for any() terms query
	if strings.Contains(filterStr, ":any(") && strings.HasSuffix(filterStr, ")") {
		idx := strings.Index(filterStr, ":any(")
		field := filterStr[:idx]
		valueStr := filterStr[idx+5 : len(filterStr)-1] // inside the parentheses
		items := strings.Split(valueStr, ",")
		values := make([]interface{}, 0, len(items))
		for _, item := range items {
			values = append(values, parseValue(strings.TrimSpace(item)))
		}
		clause := TermsQuery(field, values)
		if negate {
			return MustNotQuery(clause), nil
		}
		return clause, nil
	}

	// Range queries: age>=18, age<=30, age>10, age<40
	rangeOps := []struct {
		opStr  string
		opType QueryType
	}{
		{">=", QueryRangeGte},
		{"<=", QueryRangeLte},
		{">", QueryRangeGt},
		{"<", QueryRangeLt},
	}

	for _, op := range rangeOps {
		if idx := strings.Index(filterStr, op.opStr); idx > 0 {
			field := strings.TrimSpace(filterStr[:idx])
			valueStr := strings.TrimSpace(filterStr[idx+len(op.opStr):])
			value := parseValue(valueStr)
			clause := &Clause{
				Field: field, Operator: op.opType, Value: value,
			}
			if negate {
				return MustNotQuery(clause), nil
			}
			return clause, nil
		}
	}

	// Term queries: try "field:value" or "field=value"
	for _, sep := range []string{":", "="} {
		if idx := strings.Index(filterStr, sep); idx > 0 {
			field := strings.TrimSpace(filterStr[:idx])
			valueStr := strings.TrimSpace(filterStr[idx+1:])
			value := parseValue(valueStr)
			clause := TermQuery(field, value)
			if negate {
				return MustNotQuery(clause), nil
			}
			return clause, nil
		}
	}

	// If nothing matches, treat as a match query on default field or full query string
	if defaultFields != nil && len(defaultFields) > 1 {
		clause := MultiMatchQuery(defaultFields, filterStr)
		if negate {
			return MustNotQuery(clause), nil
		}
		return clause, nil
	} else {
		field := "*"
		if len(defaultFields) == 1 {
			field = defaultFields[0]
		}

		clause := MatchQuery(field, filterStr)
		if negate {
			return MustNotQuery(clause), nil
		}
		return clause, nil
	}
}

func parseValue(val string) interface{} {
	// Try parse as int or bool, fallback string
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}
	return val
}

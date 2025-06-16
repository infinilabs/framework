package orm

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// parseQuery attempts to extract field:value only if the field name is valid
func parseQuery(queryStr string) (field string, value string) {
	parts := strings.SplitN(queryStr, ":", 2)
	if len(parts) == 2 {
		fieldCandidate := strings.TrimSpace(parts[0])
		// Only treat as field:value if fieldCandidate is safe (ASCII only, no punctuations)
		if isValidFieldName(fieldCandidate) {
			return strings.TrimSpace(fieldCandidate), strings.TrimSpace(parts[1])
		}
	}
	return "", strings.TrimSpace(queryStr)
}

func isValidFieldName(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func NewQueryBuilderFromRequest(req *http.Request, defaultField ...string) (*QueryBuilder, error) {
	q := req.URL.Query()

	builder := NewQuery()

	fuzzinessVal := 3 // 0, exact match
	if fuzziness := q.Get("fuzziness"); fuzziness != "" {
		if v, err := strconv.Atoi(fuzziness); err == nil && v >= 0 && v <= 5 {
			fuzzinessVal = v
		}
	}

	fields := []string{}
	if reqFields := q.Get("fields"); reqFields != "" {
		fields = strings.Split(reqFields, ",")
	}
	if len(defaultField) > 0 {
		fields = append(fields, defaultField...)
	}

	// Handle full text or term query
	queryStr := q.Get("query")
	if queryStr != "" {
		var field, value = parseQuery(queryStr)
		if field != "" && value != "" {
			switch fuzzinessVal {
			case 0, 1:
				builder.Must(MatchQuery(field, value).SetBoost(5))
			case 2:
				builder.Must(ShouldQuery(MatchQuery(field, value).SetBoost(5), PrefixQuery(field, value).SetBoost(2)))
			case 3:
				builder.Must(ShouldQuery(MatchQuery(field, value).SetBoost(5), PrefixQuery(field, value).SetBoost(3), MatchPhraseQuery(field, value, 0).SetBoost(2)))
			case 4:
				builder.Must(ShouldQuery(MatchQuery(field, value).SetBoost(5), PrefixQuery(field, value).SetBoost(3)), MatchPhraseQuery(field, value, 1).SetBoost(2), FuzzyQuery(field, value, 1).SetBoost(1))
			case 5:
				builder.Must(ShouldQuery(MatchQuery(field, value).SetBoost(5), PrefixQuery(field, value).SetBoost(3), MatchPhraseQuery(field, value, 2).SetBoost(2), FuzzyQuery(field, value, 2).SetBoost(1)))
			}
		} else if value != "" {
			shouldClauses := []*Clause{}
			//try all fields with query
			for _, field := range fields {
				switch fuzzinessVal {
				case 0, 1:
					shouldClauses = append(shouldClauses, MatchQuery(field, queryStr).SetBoost(5))
				case 2:
					shouldClauses = append(shouldClauses,
						MatchQuery(field, queryStr).SetBoost(5),
						PrefixQuery(field, queryStr).SetBoost(2),
					)
				case 3:
					shouldClauses = append(shouldClauses,
						MatchQuery(field, queryStr).SetBoost(5),
						PrefixQuery(field, queryStr).SetBoost(3),
						MatchPhraseQuery(field, queryStr, 0).SetBoost(2),
					)
				case 4:
					shouldClauses = append(shouldClauses,
						MatchQuery(field, queryStr).SetBoost(5),
						PrefixQuery(field, queryStr).SetBoost(3),
						MatchPhraseQuery(field, queryStr, 1).SetBoost(2),
						FuzzyQuery(field, queryStr, 1).SetBoost(1),
					)
				case 5:
					shouldClauses = append(shouldClauses,
						MatchQuery(field, queryStr).SetBoost(5),
						PrefixQuery(field, queryStr).SetBoost(3),
						MatchPhraseQuery(field, queryStr, 2).SetBoost(2),
						FuzzyQuery(field, queryStr, 2).SetBoost(1),
					)
				}
			}
			if len(shouldClauses) > 0 {
				builder.Must(ShouldQuery(shouldClauses...))
			}
		}
	}

	// Handle filters (supporting NOT with '-' prefix)
	for _, filterRaw := range q["filter"] {
		filterStr, err := url.QueryUnescape(filterRaw)
		if err != nil {
			filterStr = filterRaw // fallback if invalid encoding
		}

		clause, err := parseFilterToClause(fields, filterStr)
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
	if strings.HasPrefix(filterStr, "-") {
		negate = true
		filterStr = strings.TrimSpace(filterStr[1:])
	}

	// Handle exists(field)
	if strings.HasPrefix(filterStr, "exists(") && strings.HasSuffix(filterStr, ")") {
		field := strings.TrimSuffix(strings.TrimPrefix(filterStr, "exists("), ")")
		clause := ExistsQuery(field)
		if negate {
			return &Clause{
				BoolType:   MustNot,
				SubClauses: []*Clause{clause},
			}, nil
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
				Query: &LeafQuery{Field: field, Operator: op.opType, Value: value},
			}
			if negate {
				return &Clause{
					BoolType:   MustNot,
					SubClauses: []*Clause{clause},
				}, nil
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
				return &Clause{
					BoolType:   MustNot,
					SubClauses: []*Clause{clause},
				}, nil
			}
			return clause, nil
		}
	}

	// If nothing matches, treat as a match query on default field or full query string
	if defaultFields != nil && len(defaultFields) > 1 {
		clause := MultiMatchQuery(defaultFields, filterStr)
		if negate {
			return &Clause{
				BoolType:   MustNot,
				SubClauses: []*Clause{clause},
			}, nil
		}
		return clause, nil
	} else {
		field := "*"
		if len(defaultFields) == 1 {
			field = defaultFields[0]
		}

		clause := MatchQuery(field, filterStr)
		if negate {
			return &Clause{
				BoolType:   MustNot,
				SubClauses: []*Clause{clause},
			}, nil
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

func findFirstLeafClause(c *Clause) *Clause {
	if c == nil {
		return nil
	}
	if c.Query != nil {
		return c
	}
	for _, sub := range c.SubClauses {
		if leaf := findFirstLeafClause(sub); leaf != nil {
			return leaf
		}
	}
	return nil
}

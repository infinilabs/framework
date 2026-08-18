package orm

// ──────────────────────────────────────────────────────────────────────────
// Legacy ORM API — frozen.
//
// These functions predate the QueryBuilder/SearchV2 API and are kept only
// for backward compatibility. New code must use:
//   read/search:  SearchV2 + NewQuery/NewQueryBuilderFromRequest
//                 (decode via elastic.DecodeHits[T] / elastic.DecodeSearchResult)
//   get:          GetV2
//   write:        Create / Update / UpdatePartialFields / Save / Delete
//   aggregations: QueryBuilder.SetAggregations + SearchV2
//
// Do not add new callers; existing ones are being migrated (see the
// SEARCH_ORM_REFACTOR_PLAN progress table). This file is deleted once the
// migration count reaches zero.
// ──────────────────────────────────────────────────────────────────────────

import (
	"errors"
	"infini.sh/framework/core/util"
	"reflect"
	"strings"
)

type LegacyORMAPI interface {
	Count(o interface{}, query interface{}) (int64, error)
	GroupBy(o interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{})

	GetIndexName(o interface{}) string
	GetWildcardIndexName(o interface{}) string

	GetBy(field string, value interface{}, o interface{}) (error, Result)
	DeleteBy(o interface{}, query interface{}) error
	UpdateBy(o interface{}, query interface{}) error
	Search(o interface{}, q *Query) (error, Result)
	SearchWithResultItemMapper(resultArrayRef interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *Query) (error, *SimpleResult)
}

// Query is the legacy search request model.
//
// Deprecated: use QueryBuilder (NewQuery / NewQueryBuilderFromRequest) with SearchV2.
type Query struct {
	Sort           *[]Sort
	QueryArgs      *[]util.KV
	From           int
	CollapseField  string
	Size           int
	Conds          []*Cond
	RawQuery       []byte
	TemplatedQuery *TemplatedQuery
	WildcardIndex  bool
	IndexName      string
	Filter         *Cond
}

type TemplatedQuery struct {
	TemplateID string                 `json:"id"`
	Parameters map[string]interface{} `json:"params"`
}

func (q *Query) Collapse(field string) *Query {
	q.CollapseField = field
	return q
}

func (q *Query) AddSort(field string, sortType SortType) *Query {
	if q.Sort == nil {
		q.Sort = &[]Sort{}
	}
	*q.Sort = append(*q.Sort, Sort{Field: field, SortType: sortType})

	return q
}

func (q *Query) AddQueryArgs(name string, value string) *Query {
	if q.QueryArgs == nil {
		q.QueryArgs = &[]util.KV{}
	}
	*q.QueryArgs = append(*q.QueryArgs, util.KV{Key: name, Value: value})

	return q
}

// Cond is a legacy query condition.
//
// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
type Cond struct {
	Field       string
	SQLOperator string
	QueryType   QueryType
	BoolType    BoolType
	Value       interface{}
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Prefix(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " ~= "
	c.QueryType = PrefixQueryType
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func QueryString(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " = "
	c.QueryType = QueryStringType
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Eq(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " = "
	c.QueryType = Match
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func NotEq(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " != "
	c.QueryType = Match
	c.BoolType = MustNot
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func In(field string, value []interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " in "
	c.QueryType = Terms
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func InStringArray(field string, value []string) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " in "
	c.QueryType = StringTerms
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Gt(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " > "
	c.QueryType = RangeGt
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Lt(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " < "
	c.QueryType = RangeLt
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Ge(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " >= "
	c.QueryType = RangeGte
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Le(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " <= "
	c.QueryType = RangeLte
	c.BoolType = Must
	return &c
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Combine(conds ...[]*Cond) []*Cond {
	t := []*Cond{}
	for _, cs := range conds {
		for _, c := range cs {
			t = append(t, c)
		}
	}
	return t
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func And(conds ...*Cond) []*Cond {
	t := []*Cond{}
	for _, c := range conds {
		c.BoolType = Must
		t = append(t, c)
	}
	return t
}

// Deprecated: use the QueryBuilder clause constructors (TermQuery, RangeQuery, Must/Should/...).
func Or(conds ...*Cond) []*Cond {
	t := []*Cond{}
	for _, c := range conds {
		c.BoolType = Should
		t = append(t, c)
	}
	return t
}

// Result is the legacy search result envelope.
//
// Deprecated: use SearchV2 with elastic.DecodeHits / elastic.DecodeSearchResult.
type Result struct {
	Total  int64
	Raw    []byte
	Result []interface{}
}

// SimpleResult is the legacy search result envelope.
//
// Deprecated: use SearchV2 with elastic.DecodeHits / elastic.DecodeSearchResult.
type SimpleResult struct {
	Total int64
	Raw   []byte
}

// Get loads a record by the ID field of o.
//
// Deprecated: use GetV2.
func Get(o interface{}) (bool, error) {

	rValue := reflect.ValueOf(o)

	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		return false, errors.New("id was not found")
	}

	return getHandler().Get(nil, o)
}

// DeleteBy deletes records matching the handler-dialect query.
//
// Deprecated: use DeleteByQuery with a QueryBuilder (the legacy query argument
// is ES DSL bytes on the elastic handler and raw SQL on sqlite — a dialect trap).
func DeleteBy(o interface{}, query interface{}) error {
	return getHandler().DeleteBy(o, query)
}

// UpdateBy updates records matching the handler-dialect query.
//
// Deprecated: use per-ID Update / UpdatePartialFields, or SearchV2 + batch writes.
func UpdateBy(o interface{}, query interface{}) error {
	return getHandler().UpdateBy(o, query)
}

// Count counts records matching the handler-dialect query.
//
// Deprecated: use SearchV2 and read hits.total.
func Count(o interface{}, query interface{}) (int64, error) {
	return getHandler().Count(o, query)
}

// Search runs a legacy query and returns flattened hits.
//
// Deprecated: use SearchV2 with a QueryBuilder.
func Search(o interface{}, q *Query) (error, Result) {
	return getHandler().Search(o, q)
}

// SearchWithResultItemMapper runs a legacy query, mapping each hit via itemMapFunc.
//
// Deprecated: use SearchV2 with elastic.DecodeHits[T].
func SearchWithResultItemMapper(o interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *Query) (error, *SimpleResult) {
	return getHandler().SearchWithResultItemMapper(o, itemMapFunc, q)
}

// SearchWithJSONMapper runs a legacy query, mapping hits via reflection.
//
// Deprecated: use SearchV2 with elastic.DecodeHits[T].
func SearchWithJSONMapper(o interface{}, q *Query) (error, SimpleResult) {
	err, searchResponse := getHandler().SearchWithResultItemMapper(o, MapToStructWithMap, q)
	if err != nil || searchResponse == nil {
		return err, SimpleResult{}
	}

	return nil, *searchResponse
}

// GroupBy runs a legacy aggregation.
//
// Deprecated: use QueryBuilder.SetAggregations with SearchV2.
func GroupBy(o interface{}, selectField, groupField, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {
	return getHandler().GroupBy(o, selectField, groupField, haveQuery, haveValue)
}

type ProtectedFilterKeyType string

//const ProtectedFilterKey ProtectedFilterKeyType = "FILTER_PROTECTED"

// FilterFieldsByProtected filter struct fields by tag protected recursively,
// returns a filtered fields map
func FilterFieldsByProtected(obj interface{}, protected bool) map[string]interface{} {
	buf := util.MustToJSONBytes(obj)
	mapObj := map[string]interface{}{}
	util.MustFromJSONBytes(buf, &mapObj)
	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)
	if t.Kind() == reflect.Ptr {
		if v.IsZero() {
			return nil
		}
		t = t.Elem()
		v = v.Elem()
	}
	for i := 0; i < v.NumField(); i++ {
		fieldType := t.Field(i)
		var jsonName = fieldType.Name
		switch jsonTag := fieldType.Tag.Get("json"); jsonTag {
		case "-":
		case "":
		default:
			parts := strings.Split(jsonTag, ",")
			name := strings.TrimSpace(parts[0])
			if name != "" {
				jsonName = name
			}
		}
		tagVal := fieldType.Tag.Get("protected")
		if strings.ToLower(tagVal) != "true" && protected {
			delete(mapObj, jsonName)
			continue
		} else if strings.ToLower(tagVal) == "true" && !protected {
			delete(mapObj, jsonName)
			continue
		}
		if fieldType.Type.Kind() == reflect.Struct || (fieldType.Type.Kind() == reflect.Ptr && fieldType.Type.Elem().Kind() == reflect.Struct) {
			mapObj[jsonName] = FilterFieldsByProtected(v.Field(i).Interface(), protected)
		}
	}
	return mapObj
}

// GetBy fetches records by an exact field match.
//
// Deprecated: use SearchV2 with a TermQuery filter (or GetV2 for ID lookups).
func GetBy(field string, value interface{}, t interface{}) (error, Result) {

	return getHandler().GetBy(field, value, t)
}

// GetWildcardIndexName resolves the wildcard index name for o.
//
// Deprecated: resolve index names via the ORM handler or model registration directly.
func GetWildcardIndexName(o interface{}) string {
	return getHandler().GetWildcardIndexName(o)
}

// GetIndexName resolves the index/table name for o.
//
// Deprecated: resolve index names via the ORM handler or model registration directly.
func GetIndexName(o interface{}) string {
	return getHandler().GetIndexName(o)
}

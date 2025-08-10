package orm

import (
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

type Cond struct {
	Field       string
	SQLOperator string
	QueryType   QueryType
	BoolType    BoolType
	Value       interface{}
}

func Prefix(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " ~= "
	c.QueryType = PrefixQueryType
	c.BoolType = Must
	return &c
}

func QueryString(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " = "
	c.QueryType = QueryStringType
	c.BoolType = Must
	return &c
}

func Eq(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " = "
	c.QueryType = Match
	c.BoolType = Must
	return &c
}

func NotEq(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " != "
	c.QueryType = Match
	c.BoolType = MustNot
	return &c
}

func In(field string, value []interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " in "
	c.QueryType = Terms
	c.BoolType = Must
	return &c
}

func InStringArray(field string, value []string) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " in "
	c.QueryType = StringTerms
	c.BoolType = Must
	return &c
}

func Gt(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " > "
	c.QueryType = RangeGt
	c.BoolType = Must
	return &c
}

func Lt(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " < "
	c.QueryType = RangeLt
	c.BoolType = Must
	return &c
}

func Ge(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " >= "
	c.QueryType = RangeGte
	c.BoolType = Must
	return &c
}

func Le(field string, value interface{}) *Cond {
	c := Cond{}
	c.Field = field
	c.Value = value
	c.SQLOperator = " <= "
	c.QueryType = RangeLte
	c.BoolType = Must
	return &c
}

func Combine(conds ...[]*Cond) []*Cond {
	t := []*Cond{}
	for _, cs := range conds {
		for _, c := range cs {
			t = append(t, c)
		}
	}
	return t
}

func And(conds ...*Cond) []*Cond {
	t := []*Cond{}
	for _, c := range conds {
		c.BoolType = Must
		t = append(t, c)
	}
	return t
}

func Or(conds ...*Cond) []*Cond {
	t := []*Cond{}
	for _, c := range conds {
		c.BoolType = Should
		t = append(t, c)
	}
	return t
}

type Result struct {
	Total  int64
	Raw    []byte
	Result []interface{}
}

type SimpleResult struct {
	Total int64
	Raw   []byte
}

func DeleteBy(o interface{}, query interface{}) error {
	return getHandler().DeleteBy(o, query)
}
func UpdateBy(o interface{}, query interface{}) error {
	return getHandler().UpdateBy(o, query)
}

func Count(o interface{}, query interface{}) (int64, error) {
	return getHandler().Count(o, query)
}

func Search(o interface{}, q *Query) (error, Result) {
	return getHandler().Search(o, q)
}

func SearchWithResultItemMapper(o interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *Query) (error, *SimpleResult) {
	return getHandler().SearchWithResultItemMapper(o, itemMapFunc, q)
}

func SearchWithJSONMapper(o interface{}, q *Query) (error, SimpleResult) {
	err, searchResponse := getHandler().SearchWithResultItemMapper(o, MapToStructWithMap, q)
	if err != nil || searchResponse == nil {
		return err, SimpleResult{}
	}

	return nil, *searchResponse
}

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

func GetBy(field string, value interface{}, t interface{}) (error, Result) {

	return getHandler().GetBy(field, value, t)
}

func GetWildcardIndexName(o interface{}) string {
	return getHandler().GetWildcardIndexName(o)
}

func GetIndexName(o interface{}) string {
	return getHandler().GetIndexName(o)
}

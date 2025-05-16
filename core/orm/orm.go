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

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package orm

import (
	"context"
	"reflect"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

type Context struct {
	context.Context
	Refresh string
}

const WaitForRefresh = "wait_for"
const ImmediatelyRefresh = "true"

type ORM interface {
	RegisterSchemaWithIndexName(t interface{}, indexName string) error

	GetIndexName(o interface{}) string

	GetWildcardIndexName(o interface{}) string

	Save(ctx *Context, o interface{}) error

	Update(ctx *Context, o interface{}) error

	Delete(ctx *Context, o interface{}) error

	Search(o interface{}, q *Query) (error, Result)

	SearchWithResultItemMapper(resultArrayRef interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *Query) (error, *SimpleResult)

	Get(o interface{}) (bool, error)

	GetBy(field string, value interface{}, o interface{}) (error, Result)

	Count(o interface{}, query interface{}) (int64, error)

	GroupBy(o interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{})
	DeleteBy(o interface{}, query interface{}) error
	UpdateBy(o interface{}, query interface{}) error
}

type ORMObjectBase struct {
	ID      string     `config:"id"  json:"id,omitempty" protected:"true"   elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Created *time.Time `json:"created,omitempty" elastic_mapping:"created: { type: date }"`
	Updated *time.Time `json:"updated,omitempty" elastic_mapping:"updated: { type: date }"`
}

func (obj *ORMObjectBase) GetID() string {
	return obj.ID
}
func (obj *ORMObjectBase) SetID(ID string) {
	obj.ID = ID
}

type Object interface {
	GetID() string
	SetID(ID string)
}

type Sort struct {
	Field    string
	SortType SortType
}

type SortType string

const ASC SortType = "asc"
const DESC SortType = "desc"

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

type BoolType string
type QueryType string

const Must BoolType = "must"
const MustNot BoolType = "must_not"
const Should BoolType = "should"

const Term QueryType = "term"
const PrefixQueryType QueryType = "prefix"
const Wildcard QueryType = "wildcard"
const Regexp QueryType = "regexp" //TODO check
const Match QueryType = "match"
const QueryStringType QueryType = "query_string"
const RangeGt QueryType = "gt"
const RangeGte QueryType = "gte"
const RangeLt QueryType = "lt"
const RangeLte QueryType = "lte"

const StringTerms QueryType = "string_terms"
const Terms QueryType = "terms"

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

func Get(o interface{}) (bool, error) {
	rValue := reflect.ValueOf(o)

	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		return false, errors.New("id was not found")
	}

	return getHandler().Get(o)
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

func getFieldStringValue(rValue reflect.Value, fieldName string) (bool, string) {

	if rValue.Kind() == reflect.Ptr {
		rValue = reflect.Indirect(rValue)
	}

	f := rValue.FieldByName(fieldName)

	if f.IsValid() && f.String() != "" {
		return true, f.String()
	}
	return false, ""
}

func existsNonNullField(rValue reflect.Value, fieldName string) bool {

	if rValue.Kind() == reflect.Ptr {
		rValue = reflect.Indirect(rValue)
	}

	f := rValue.FieldByName(fieldName)
	if f.Kind() == reflect.Ptr {
		return !f.IsNil()
	}

	if f.IsValid() {
		return true
	}
	return false
}

func setFieldValue(v reflect.Value, param string, value interface{}) {

	if v.Kind() == reflect.Ptr {
		v = reflect.Indirect(v)
	}

	f := v.FieldByName(param)

	if f.IsValid() {
		if f.Type().String() == "*time.Time" { //处理time.Time时间类型
			vType := reflect.TypeOf(value).String()
			if vType == "*time.Time" {
				f.Set(reflect.ValueOf(value))
			}
		} else if f.Type().String() == "time.Time" { //处理time.Time时间类型
			//TODO fix this: https://www.cnblogs.com/marshhu/p/12837834.html
			//vType:=reflect.TypeOf(value).String()
			//if vType=="time.Time"{
			//	timeValue := value.(time.Time)
			//	f.Set(reflect.ValueOf(timeValue.String()))
			//}
		} else {
			if f.CanSet() {
				if f.Kind() == reflect.String {
					f.SetString(value.(string))
					return
				} else if f.Kind() == reflect.Struct {
					f.Set(reflect.ValueOf(value))
				}
			}
		}
	}
}

func Create(ctx *Context, o interface{}) error {
	t := reflect.TypeOf(o)
	if t.Kind() != reflect.Ptr {
		return errors.New("only point of object is allowed")
	}

	rValue := reflect.ValueOf(o)
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		setFieldValue(rValue, "ID", util.GetUUID())
	}

	time1 := time.Now()
	setFieldValue(rValue, "Created", &time1)
	setFieldValue(rValue, "Updated", &time1)

	return Save(ctx, o)
}

//TODO support upsert and partial update

func Save(ctx *Context, o interface{}) error {
	rValue := reflect.ValueOf(o)
	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		return errors.New("id was not found")
	}

	createdExists := existsNonNullField(rValue, "Created")
	t := time.Now()
	setFieldValue(rValue, "Updated", &t)
	if !createdExists {
		setFieldValue(rValue, "Created", &t)
	}

	return getHandler().Save(ctx, o)
}

func Update(ctx *Context, o interface{}) error {
	t := reflect.TypeOf(o)
	if t.Kind() != reflect.Ptr {
		return errors.New("only point of the object is allowed")
	}

	////NOTICE: get will affect object after get
	//exists, err := Get(o)
	//if err != nil {
	//	return err
	//}
	//
	//if !exists {
	//	return errors.New("failed to update, object was not found")
	//}

	rValue := reflect.ValueOf(o)
	t1 := time.Now()
	setFieldValue(rValue, "Updated", &t1)

	return getHandler().Update(ctx, o)
}

func Delete(ctx *Context, o interface{}) error {
	return getHandler().Delete(ctx, o)
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
	err, searchResponse := getHandler().SearchWithResultItemMapper(o, MapToStructWithJSONUnmarshal, q)
	if err != nil || searchResponse == nil {
		return err, SimpleResult{}
	}

	return nil, *searchResponse
}

func GroupBy(o interface{}, selectField, groupField, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {
	return getHandler().GroupBy(o, selectField, groupField, haveQuery, haveValue)
}

var registeredSchemas = []util.KeyValue{}

func MustRegisterSchemaWithIndexName(t interface{}, index string) {
	err := RegisterSchemaWithIndexName(t, index)
	if err != nil {
		panic(err)
	}
}

func RegisterSchemaWithIndexName(t interface{}, index string) error {
	registeredSchemas = append(registeredSchemas, util.KeyValue{Key: index, Payload: t})
	return nil
}

func InitSchema() error {
	for _, v := range registeredSchemas {
		err := getHandler().RegisterSchemaWithIndexName(v.Payload, v.Key)
		if err != nil {
			return err
		}
	}
	return nil
}

var handler ORM

func getHandler() ORM {
	if handler == nil {
		panic(errors.New("ORM handler is not registered"))
	}
	return handler
}

var adapters map[string]ORM

func Register(name string, h ORM) {
	if adapters == nil {
		adapters = map[string]ORM{}
	}
	_, ok := adapters[name]
	if ok {
		panic(errors.Errorf("ORM handler with same name: %v already exists", name))
	}

	adapters[name] = h

	handler = h

	log.Debug("register ORM handler: ", name)

}

type ProtectedFilterKeyType string

const ProtectedFilterKey ProtectedFilterKeyType = "FILTER_PROTECTED"

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

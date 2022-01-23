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
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"reflect"
	"time"
)

type ORM interface {
	RegisterSchema(t interface{}) error

	RegisterSchemaWithIndexName(t interface{}, indexName string) error

	GetIndexName(o interface{}) string

	Save(o interface{}) error

	Update(o interface{}) error

	Delete(o interface{}) error

	Search(o interface{}, q *Query) (error, Result)

	Get(o interface{}) (bool, error)

	GetBy(field string, value interface{}, o interface{}) (error, Result)

	Count(o interface{}) (int64, error)

	GroupBy(o interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{})
}

type ORMObjectBase struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id,: { type: keyword }"`
	Created time.Time `json:"created,omitempty" elastic_mapping:"created: { type: date }"`
	Updated time.Time `json:"updated,omitempty" elastic_mapping:"updated: { type: date }"`
}

type Sort struct {
	Field    string
	SortType SortType
}

type SortType string

const ASC SortType = "asc"
const DESC SortType = "desc"

type Query struct {
	Sort     *[]Sort
	From     int
	Size     int
	Conds    []*Cond
	RawQuery []byte
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

const Match QueryType = "match"
const RangeGt QueryType = "gt"
const RangeGte QueryType = "gte"
const RangeLt QueryType = "lt"
const RangeLte QueryType = "lte"

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
		t = append(t, c)
	}
	return t
}

type Result struct {
	Total  int64
	Raw    []byte
	Result interface{}
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

func setFieldValue(v reflect.Value, param string, value interface{}) {

	if v.Kind() == reflect.Ptr {
		v = reflect.Indirect(v)
	}

	f := v.FieldByName(param)
	if f.IsValid() {
		if f.CanSet() {
			if f.Kind() == reflect.String {
				f.SetString(value.(string))
				return
			} else if f.Kind() == reflect.Struct {
				f.Set(reflect.ValueOf(value))
			}
		}
	} else {

	}
}

func Create(o interface{}) error {
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
	setFieldValue(rValue, "Created", time1)
	setFieldValue(rValue, "Updated", time1)

	return Save(o)
}

func Save(o interface{}) error {

	rValue := reflect.ValueOf(o)

	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	nameExists, _ := getFieldStringValue(rValue, "Name")

	if !idExists {
		return errors.New("id was not found")
	}

	if !nameExists {
		return errors.New("name was not found")
	}

	return getHandler().Save(o)
}

func Update(o interface{}) error {
	t := reflect.TypeOf(o)
	if t.Kind() != reflect.Ptr {
		return errors.New("only point of the object is allowed")
	}

	rValue := reflect.ValueOf(o)

	setFieldValue(rValue, "Updated", time.Now())

	exists, err := Get(o)
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("failed to update, object was not found")
	}

	return Save(o)
}

func Delete(o interface{}) error {
	return getHandler().Delete(o)
}

func Count(o interface{}) (int64, error) {
	return getHandler().Count(o)
}

func Search(o interface{}, q *Query) (error, Result) {
	return getHandler().Search(o, q)
}

func GroupBy(o interface{}, selectField, groupField, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {
	return getHandler().GroupBy(o, selectField, groupField, haveQuery, haveValue)
}

func RegisterSchemaWithIndexName(t interface{}, index string) error {
	return getHandler().RegisterSchemaWithIndexName(t, index)
}

func RegisterSchema(t interface{}) error {
	return getHandler().RegisterSchema(t)
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

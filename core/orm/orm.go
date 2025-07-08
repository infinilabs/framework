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
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"reflect"
	"time"
)

type SearchAPI interface {
	SearchV2(ctx *Context, qb *QueryBuilder) (*SearchResult, error)
}

type MetricsAPI interface {
}

type ORM interface {
	LegacyORMAPI

	SearchAPI

	MetricsAPI

	RegisterSchemaWithName(t interface{}, customizedName string) error

	Save(ctx *Context, o interface{}) error

	Update(ctx *Context, o interface{}) error

	Delete(ctx *Context, o interface{}) error

	Get(o interface{}) (bool, error)
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

type BoolType string

const Filter BoolType = "filter"
const Must BoolType = "must"
const MustNot BoolType = "must_not"
const Should BoolType = "should"

type SearchResult struct {
	Error   *error      // pointer to error
	Status  int         // HTTP status or internal status code
	Payload interface{} // raw response body (e.g. JSON)
}

func (r *SearchResult) IsError() bool {
	return r.Error != nil
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

func getFieldStringValue(rValue reflect.Value, fieldName string) (bool, string) {
	// Handle nil pointers
	if !rValue.IsValid() || (rValue.Kind() == reflect.Ptr && rValue.IsNil()) {
		log.Errorf("invalid or nil value for field %s", fieldName)
		return false, ""
	}

	// Dereference if it's a pointer
	if rValue.Kind() == reflect.Ptr {
		rValue = rValue.Elem()
	}

	// Make sure it's a struct
	if rValue.Kind() != reflect.Struct {
		log.Errorf("expected struct for field lookup, got %s", rValue.Kind())
		return false, ""
	}

	// Get the field
	f := rValue.FieldByName(fieldName)
	if !f.IsValid() {
		log.Errorf("field %s not found", fieldName)
		return false, ""
	}

	// Check that it’s a string
	if f.Kind() != reflect.String {
		log.Errorf("field %s is not a string", fieldName)
		return false, ""
	}

	val := f.String()
	if val != "" {
		return true, val
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

// TODO support upsert and partial update
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

func SearchV2(ctx *Context, qb *QueryBuilder) (*SearchResult, error) {
	return getHandler().SearchV2(ctx, qb)
}

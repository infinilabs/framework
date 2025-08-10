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
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"reflect"
	"strings"
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

	Create(ctx *Context, o interface{}) error

	Update(ctx *Context, o interface{}) error

	Delete(ctx *Context, o interface{}) error

	Get(ctx *Context, o interface{}) (bool, error)
}

type ORMObjectBase struct {
	ID      string      `config:"id"  json:"id,omitempty" protected:"true"   elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Created *time.Time  `json:"created,omitempty" elastic_mapping:"created: { type: date }"`
	Updated *time.Time  `json:"updated,omitempty" elastic_mapping:"updated: { type: date }"`
	System  util.MapStr `json:"_system,omitempty" elastic_mapping:"_system: { type: object }"`
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

type SystemFieldAccessor interface {
	GetSystemValue(key string) (interface{}, bool)
	GetSystemString(key string) string
	GetSystemBool(key string) bool
	GetSystemInt(key string) int
	SetSystemValue(key string, value interface{})
	SetSystemValues(m util.MapStr)
}

func (obj *ORMObjectBase) SetSystemValues(m util.MapStr) {
	obj.System = m
}

func (obj *ORMObjectBase) SetSystemValue(key string, value interface{}) {
	if obj.System == nil {
		obj.System = util.MapStr{}
	}
	obj.System[key] = value
}

func (obj *ORMObjectBase) GetSystemValue(key string) (interface{}, bool) {
	if obj.System == nil {
		return nil, false
	}
	val, ok := obj.System[key]
	return val, ok
}

func (obj *ORMObjectBase) GetSystemString(key string) string {
	if val, ok := obj.System[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (obj *ORMObjectBase) GetSystemBool(key string) bool {
	if val, ok := obj.System[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func (obj *ORMObjectBase) GetSystemInt(key string) int {
	if val, ok := obj.System[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
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

func GetWithSystemFields(ctx *Context, o interface{}) (bool, error) {
	ctx.Set(KeepSystemFields, true)
	return GetV2(ctx, o)
}

func GetV2(ctx *Context, o interface{}) (bool, error) {
	rValue := reflect.ValueOf(o)

	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		return false, errors.New("id was not found")
	}

	var err error
	if ctx, o, err = runDataOperationPreHooks(OpGet, ctx, o); err != nil {
		return false, err
	}

	exists, err := getHandler().Get(ctx, o)
	if err != nil || !exists {
		return exists, err
	}

	if ctx, o, err = runDataOperationPostHooks(OpGet, ctx, o); err != nil {
		return false, err
	}

	return exists, err
}

func Get(o interface{}) (bool, error) {
	rValue := reflect.ValueOf(o)

	//check required value
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		return false, errors.New("id was not found")
	}

	return getHandler().Get(nil, o)
}

func getFieldStringValue(rValue reflect.Value, fieldName string) (bool, string) {
	// Handle nil or invalid values
	if !rValue.IsValid() || (rValue.Kind() == reflect.Ptr && rValue.IsNil()) {
		log.Errorf("invalid or nil value for field %s", fieldName)
		return false, ""
	}

	// Dereference pointer
	if rValue.Kind() == reflect.Ptr {
		rValue = rValue.Elem()
	}

	switch rValue.Kind() {
	case reflect.Struct:
		// Struct field access
		f := rValue.FieldByName(fieldName)
		if !f.IsValid() {
			log.Errorf("field %s not found in struct", fieldName)
			return false, ""
		}
		if f.Kind() != reflect.String {
			log.Errorf("field %s is not a string in struct", fieldName)
			return false, ""
		}
		val := f.String()
		return val != "", val

	case reflect.Map:
		// Map key access (assumes map[string]interface{})
		if rValue.Type().Key().Kind() != reflect.String {
			log.Errorf("map key is not string, cannot access field %s", fieldName)
			return false, ""
		}
		key := reflect.ValueOf(fieldName)
		value := rValue.MapIndex(key)
		if !value.IsValid() {
			log.Debugf("key %s not found in map", fieldName)
			return false, ""
		}
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}
		if value.Kind() != reflect.String {
			log.Errorf("value for key %s is not a string", fieldName)
			return false, ""
		}
		val := value.String()
		return val != "", val

	default:
		log.Errorf("unsupported kind %s for field lookup", rValue.Kind())
		return false, ""
	}
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

// findFieldValue searches for a field by name in v, including nested embedded structs.
func findFieldValue(v reflect.Value, name string) reflect.Value {
	if !v.IsValid() {
		return reflect.Value{}
	}

	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if field.Name == name {
			return fv
		}

		// If it's an embedded struct (anonymous), search recursively
		if field.Anonymous {
			found := findFieldValue(fv, name)
			if found.IsValid() {
				return found
			}
		}
	}

	return reflect.Value{}
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

	var err error
	if ctx, o, err = runDataOperationPreHooks(OpCreate, ctx, o); err != nil {
		return err
	}

	err = getHandler().Create(ctx, o)
	if err != nil {
		return err
	}

	if ctx, o, err = runDataOperationPostHooks(OpCreate, ctx, o); err != nil {
		return err
	}

	return err
}

func GetPrevObject(ctx *Context, o interface{}) (interface{}, bool, error) {
	v := reflect.ValueOf(o)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil, false, errors.New("o must be a non-nil pointer")
	}

	idExists, id := getFieldStringValue(v, "ID")
	if !idExists {
		return nil, false, errors.New("id was not found")
	}

	prev := reflect.New(v.Type().Elem()).Interface()
	setFieldValue(reflect.ValueOf(prev), "ID", id)

	exists, err := GetWithSystemFields(ctx, prev)
	return prev, exists, err
}

func copySystemFields(src interface{}, dst interface{}) {
	fields := []string{"ID", "System", "Created", "Updated"}

	for _, field := range fields {
		val := findFieldValue(reflect.ValueOf(src), field)
		if val.IsValid() && val.CanInterface() {
			dstField := findFieldValue(reflect.ValueOf(dst), field)
			if dstField.IsValid() && dstField.CanSet() {
				dstField.Set(val)
			}
		}
	}
}

func Upsert(ctx *Context, o interface{}) error {
	//if not exists then create one
	ctx.Set(CreateIfNotExistsForUpdate, true)
	ctx.Set(AssignToCurrentUserIfNotExists, true)
	ctx.Set(CheckExistsBeforeUpdate, true)
	return Update(ctx, o)
}

// MergeMapToStruct merges a delta (map[string]interface{}) into v (reflect.Value).
// v may be a struct value or pointer-to-struct (e.g. reflect.ValueOf(&obj) or reflect.ValueOf(&obj).Elem()).
func mergeMapToStruct(delta map[string]interface{}, v reflect.Value) error {
	if !v.IsValid() {
		return errors.New("invalid target value")
	}

	// If pointer, deref to struct (require non-nil pointer)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return errors.New("nil pointer passed to MergeMapToStruct")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return errors.New("MergeMapToStruct: target must be a struct or pointer to struct")
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// First handle anonymous (embedded) fields recursively so their inner fields
		// can be matched by JSON keys at top-level of delta.
		if field.Anonymous {
			switch fv.Kind() {
			case reflect.Ptr:
				// if embedded pointer is nil, allocate one (if settable)
				if fv.IsNil() {
					if !fv.CanSet() {
						continue
					}
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				if err := mergeMapToStruct(delta, fv.Elem()); err != nil {
					return err
				}
			case reflect.Struct:
				if err := mergeMapToStruct(delta, fv); err != nil {
					return err
				}
			}
			continue
		}

		// Determine key: json tag first, fallback to field name.
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		tagName := strings.Split(tag, ",")[0]
		key := tagName
		if key == "" {
			key = field.Name
		}

		// Try tag name first; if not present and tag name differs from field name, try field name.
		val, ok := delta[key]
		if !ok && key != field.Name {
			val, ok = delta[field.Name]
		}
		if !ok {
			continue
		}

		// If the delta explicitly sets null, clear the field (works for pointers too).
		if val == nil {
			if fv.CanSet() {
				fv.Set(reflect.Zero(fv.Type()))
			}
			continue
		}

		// If delta value is a nested map and target is struct or pointer-to-struct, recurse.
		if m, isMap := val.(map[string]interface{}); isMap {
			switch fv.Kind() {
			case reflect.Struct:
				if err := mergeMapToStruct(m, fv); err != nil {
					return err
				}
				continue
			case reflect.Ptr:
				if fv.Type().Elem().Kind() == reflect.Struct {
					if fv.IsNil() {
						if !fv.CanSet() {
							continue
						}
						fv.Set(reflect.New(fv.Type().Elem()))
					}
					if err := mergeMapToStruct(m, fv.Elem()); err != nil {
						return err
					}
					continue
				}
				// otherwise fall-through to JSON fallback below
			}
		}

		// Normal scalar/path: attempt assign / convert, else fallback to JSON decode.
		rv := reflect.ValueOf(val)
		if !rv.IsValid() {
			// defensive: shouldn't happen because val != nil above
			continue
		}

		// Direct assignable
		if rv.Type().AssignableTo(fv.Type()) {
			if fv.CanSet() {
				fv.Set(rv)
			}
			continue
		}

		// Convertible
		if rv.Type().ConvertibleTo(fv.Type()) {
			if fv.CanSet() {
				fv.Set(rv.Convert(fv.Type()))
			}
			continue
		}

		// Fallback: try marshalling value back to JSON and unmarshal into the field type
		raw, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("cannot marshal delta value for field %s: %w", field.Name, err)
		}
		newPtr := reflect.New(fv.Type())
		if err := json.Unmarshal(raw, newPtr.Interface()); err != nil {
			return fmt.Errorf("cannot unmarshal into field %s: %w", field.Name, err)
		}
		if fv.CanSet() {
			fv.Set(newPtr.Elem())
		}
	}

	return nil
}

func Update(ctx *Context, o interface{}) error {
	return saveOrUpdate(ctx, o, nil, OpUpdate, false)
}

func UpdatePartialFields(ctx *Context, o interface{}, delta util.MapStr) error {
	return saveOrUpdate(ctx, o, delta, OpUpdate, false)
}

func Save(ctx *Context, o interface{}) error {
	return saveOrUpdate(ctx, o, nil, OpSave, true)
}

func saveOrUpdate(ctx *Context, o interface{}, delta util.MapStr, opType Operation, createIfNotExists bool) error {
	if reflect.TypeOf(o).Kind() != reflect.Ptr || reflect.ValueOf(o).IsNil() {
		return errors.New("only non-nil pointer to object is allowed")
	}

	rValue := reflect.ValueOf(o)
	needCheckExists := ctx.GetBool(CheckExistsBeforeUpdate, true)
	deltaNotEmpty := delta != nil && len(delta) > 0
	mergePartial := ctx.GetBool(MergePartialFieldsBeforeUpdate, true)

	var exists bool
	if needCheckExists || mergePartial || deltaNotEmpty {
		prev, found, err := GetPrevObject(ctx, o)
		if err != nil && !strings.Contains(err.Error(), "record not found") {
			return err
		}
		exists = found

		if !exists && !ctx.GetBool(CreateIfNotExistsForUpdate, createIfNotExists) {
			return errors.New("failed to update, object was not found")
		}

		if exists {
			if mergePartial && deltaNotEmpty {
				if err := mergeMapToStruct(delta, rValue); err != nil {
					return err
				}
			}
			copySystemFields(prev, o)
		}
	}

	// Always update Updated timestamp
	tNow := time.Now()
	setFieldValue(rValue, "Updated", &tNow)

	// For save, ensure Created timestamp exists
	if opType == OpSave && !existsNonNullField(rValue, "Created") {
		setFieldValue(rValue, "Created", &tNow)
	}

	// Hooks
	var err error
	if ctx, o, err = runDataOperationPreHooks(opType, ctx, o); err != nil {
		return err
	}

	// Handler call
	switch opType {
	case OpSave:
		err = getHandler().Save(ctx, o)
	case OpUpdate:
		err = getHandler().Update(ctx, o)
	}
	if err != nil {
		return err
	}

	if ctx, o, err = runDataOperationPostHooks(opType, ctx, o); err != nil {
		return err
	}

	return nil
}

func Delete(ctx *Context, o interface{}) error {

	t := reflect.TypeOf(o)
	if t.Kind() != reflect.Ptr || reflect.ValueOf(o).IsNil() {
		return errors.New("only non-nil pointer to object is allowed")
	}

	if ctx.GetBool(CheckExistsBeforeDelete, true) {
		prev, exists, err := GetPrevObject(ctx, o)
		if err != nil && !strings.Contains(err.Error(), "record not found") {
			return err
		}

		if !exists {
			return errors.New("failed to delete, object was not found")
		}

		if exists {
			// Preserve system fields from the previous object
			copySystemFields(prev, o)
		}
	}

	var err error
	if ctx, o, err = runDataOperationPreHooks(OpDelete, ctx, o); err != nil {
		return err
	}

	err = getHandler().Delete(ctx, o)
	if err != nil {
		return err
	}

	if ctx, o, err = runDataOperationPostHooks(OpDelete, ctx, o); err != nil {
		return err
	}

	return err
}

func SearchV2(ctx *Context, qb *QueryBuilder) (*SearchResult, error) {

	if err := runSearchOperationHooks(ctx, qb); err != nil {
		return nil, err
	}

	return getHandler().SearchV2(ctx, qb)
}

func InjectSystemField(obj interface{}, key string, value interface{}) error {
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		return fmt.Errorf("invalid object")
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("nil pointer object")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	// Find the "System" field
	systemField := v.FieldByName("System")
	if !systemField.IsValid() || !systemField.CanSet() {
		return fmt.Errorf("System field not found or not settable")
	}

	// Initialize if nil
	if systemField.IsNil() {
		systemField.Set(reflect.MakeMap(systemField.Type()))
	}

	// Set key in map
	systemField.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(value))
	return nil
}

func InjectSystemFields(obj interface{}, values map[string]interface{}) error {
	for k, v := range values {
		if err := InjectSystemField(obj, k, v); err != nil {
			return err
		}
	}
	return nil
}

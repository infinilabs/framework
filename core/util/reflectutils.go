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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package util

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Invoke dynamic execute function via function name and parameters
func Invoke(any interface{}, name string, args ...interface{}) {
	inputs := make([]reflect.Value, len(args))
	for i := range args {
		inputs[i] = reflect.ValueOf(args[i])
	}
	reflect.ValueOf(any).MethodByName(name).Call(inputs)
}

type Annotation struct {
	Field      string       `json:"field,omitempty"`
	Type       string       `json:"type,omitempty"`
	Tag        string       `json:"tag,omitempty"`
	Annotation []Annotation `json:"annotation,omitempty"`
}

// source should be a struct, target should be a pointer to the struct
func Copy(sourceStruct interface{}, pointToTarget interface{}) (err error) {
	dst := reflect.ValueOf(pointToTarget)
	if dst.Kind() != reflect.Ptr {
		err = errors.New("target is not a pointer")
		return
	}

	element := dst.Elem()
	if element.Kind() != reflect.Struct {
		err = errors.New("target doesn't point to struct")
		return
	}

	srcValue := reflect.ValueOf(sourceStruct)
	srcType := reflect.TypeOf(sourceStruct)
	if srcType.Kind() != reflect.Struct {
		err = errors.New("source is not a struct")
		return
	}

	for i := 0; i < srcType.NumField(); i++ {
		sf := srcType.Field(i)
		sv := srcValue.FieldByName(sf.Name)
		if dv := element.FieldByName(sf.Name); dv.IsValid() && dv.CanSet() {
			dv.Set(sv)
		}
	}
	return
}

func GetTagsByTagName(any interface{}, tagName string) []Annotation {

	t := reflect.TypeOf(any)

	var result []Annotation

	//check if it is as point
	if PrefixStr(t.String(), "*") {
		t = reflect.TypeOf(any).Elem()
	}

	//fmt.Println("")
	//fmt.Println("o: ",any,", ",tagName)
	//fmt.Println("t: ",t,", ",tagName)

	if t.Kind() == reflect.Struct {

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			v := TrimSpaces(field.Tag.Get(tagName))
			if v == "-" {
				continue
			}
			a := Annotation{Field: field.Name, Type: field.Type.Name(), Tag: v}

			//fmt.Println(field.Name)
			//fmt.Println(field.Type)
			//fmt.Println(field.Type.Kind())
			//fmt.Println(field.Tag)
			//fmt.Println(field.Type.Elem())

			if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Ptr {
				v1 := reflect.New(field.Type.Elem())
				a.Annotation = GetTagsByTagName(v1.Interface(), tagName)
			}

			if field.Type.Kind() == reflect.Struct {
				v1 := reflect.New(field.Type)
				a.Annotation = GetTagsByTagName(v1.Interface(), tagName)
				if field.Anonymous && len(a.Annotation) > 0 {
					result = append(result, a.Annotation...)
					continue
				}
			}

			if len(a.Annotation) > 0 || a.Tag != "" {
				result = append(result, a)
			}
		}

	}

	return result
}

// GetFieldValueByTagName returns the field value which field was tagged with the specified tagName, only supporting string fields.
func GetFieldValueByTagName(any interface{}, tagName string, tagValue string) string {
	t := reflect.TypeOf(any)
	v := reflect.ValueOf(any)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	if PrefixStr(t.String(), "*") {
		t = reflect.TypeOf(any).Elem()
	}

	value, err := getFieldByTagName(v, tagName, tagValue)
	if err != nil {
		panic(fmt.Errorf("tag [%v][%v] was not found: %v", tagName, tagValue, err))
	}
	return value
}

// getFieldByTagName is a recursive helper function for GetFieldValueByTagName.
func getFieldByTagName(value reflect.Value, tagName string, tagValue string) (string, error) {
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			fieldType := value.Type().Field(i)

			switch field.Kind() {
			case reflect.Struct:
				// Recursively search nested structures
				if fieldValue, err := getFieldByTagName(field, tagName, tagValue); err == nil {
					return fieldValue, nil
				}
			default:
				tag := fieldType.Tag.Get(tagName)
				if tag != "" && containTags(tagValue, tag) {
					return field.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("tag not found")
}

// containTags checks if tagValue is present in the comma-separated tags string.
func containTags(tagValue string, tags string) bool {
	tagList := strings.Split(tags, ",")
	for _, tag := range tagList {
		if strings.TrimSpace(tag) == tagValue {
			return true
		}
	}
	return false
}

func GetTypeName(any interface{}, lowercase bool) string {
	_, t := GetTypeAndPackageName(any, lowercase)
	return t
}

func GetTypeAndPackageName(any interface{}, lowercase bool) (string, string) {
	pkg := reflect.Indirect(reflect.ValueOf(any)).Type().PkgPath()
	name := reflect.Indirect(reflect.ValueOf(any)).Type().Name()
	if lowercase {
		name = strings.ToLower(name)
	}
	return pkg, name
}

func TypeIsArray(any interface{}) bool {
	vt := reflect.TypeOf(any)
	if vt.String() == "[]interface {}" {
		return true
	}
	return false
}

func TypeIsMap(any interface{}) bool {
	vt := reflect.TypeOf(any)
	if vt.String() == "map[string]interface {}" {
		return true
	}
	return false
}

func GetInt64Value(any interface{}) int64 {

	vt := reflect.TypeOf(any)
	if vt.String() == "float64" {
		v := any.(float64)
		var y = int64(v)
		return y
	} else if vt.String() == "float32" {
		v := any.(float32)
		var y = int64(v)
		return y
	} else if vt.String() == "int64" {
		v := any.(int64)
		var y = int64(v)
		return y
	} else if vt.String() == "int32" {
		v := any.(int32)
		var y = int64(v)
		return y
	} else if vt.String() == "uint64" {
		v := any.(uint64)
		var y = int64(v)
		return y
	} else if vt.String() == "uint32" {
		v := any.(uint32)
		var y = int64(v)
		return y
	} else if vt.String() == "uint" {
		v := any.(uint)
		var y = int64(v)
		return y
	} else if vt.String() == "int" {
		v := any.(int)
		var y = int64(v)
		return y
	}
	return -1
}

func ContainTags(tag string, tags string) bool {
	if strings.Contains(tags, ",") {
		arr := strings.Split(tags, ",")
		for _, v := range arr {
			if v == tag {
				return true
			}
		}
	}
	return tag == tags
}

// return field and tags, field name is using key: NAME
func GetFieldAndTags(any interface{}, tags []string) []map[string]string {

	fields := []map[string]string{}

	t := reflect.TypeOf(any)
	v := reflect.ValueOf(any)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	if PrefixStr(t.String(), "*") {
		t = reflect.TypeOf(any).Elem()
	}

	for i := 0; i < v.NumField(); i++ {

		field := map[string]string{}
		field["NAME"] = t.Field(i).Name
		field["TYPE"] = t.Field(i).Type.Name()
		field["KIND"] = v.Field(i).Kind().String()

		if v.Field(i).Kind() == reflect.Slice {
			field["TYPE"] = "array"
			field["SUB_TYPE"] = v.Field(i).Type().Elem().String()
		}

		switch v.Field(i).Kind() {
		case reflect.Struct:
			//判断是否是嵌套结构
			if v.Field(i).Type().Kind() == reflect.Struct {
				structField := v.Field(i).Type()
				for j := 0; j < structField.NumField(); j++ {
					for _, tagName := range tags {
						v := structField.Field(j).Tag.Get(tagName)
						if v != "" {
							field[tagName] = v
						}
					}
				}
				continue
			}
			break
		default:
			for _, tagName := range tags {
				v := t.Field(i).Tag.Get(tagName)
				if v != "" {
					field[tagName] = v
				}
			}
			break
		}
		fields = append(fields, field)
	}

	return fields
}

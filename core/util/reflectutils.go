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
			}

			if len(a.Annotation) > 0 || a.Tag != "" {
				result = append(result, a)
			}
		}

	}

	return result
}

// GetFieldValueByTagName return the field value which field was tagged with this tagName, only support string field
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

	for i := 0; i < v.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Struct:
			//判断是否是嵌套结构
			if v.Field(i).Type().Kind() == reflect.Struct {
				structField := v.Field(i).Type()
				for j := 0; j < structField.NumField(); j++ {
					v := structField.Field(i).Tag.Get(tagName)
					if v != "" {
						if ContainTags(tagValue, v) {
							return reflect.Indirect(reflect.ValueOf(any)).FieldByName(structField.Field(i).Name).String()
						}
					}
				}
				continue
			}
			break
		case reflect.String:
			v := t.Field(i).Tag.Get(tagName)
			if v != "" {
				if ContainTags(tagValue, v) {
					return reflect.Indirect(reflect.ValueOf(any)).FieldByName(t.Field(i).Name).String()
				}
			}
			break
		}
	}

	//TODO handle property in parent/inner objects
	panic(fmt.Errorf("tag [%v][%v] was not found", tagName, tagValue))
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
	} else if vt.String() == "uint64" {
		v := any.(uint64)
		var y = int64(v)
		return y
	} else if vt.String() == "uint" {
		v := any.(uint)
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

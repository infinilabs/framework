/*
Copyright Medcl (m AT medcl.net)

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

package param

import (
	"encoding/base64"
	"errors"
	"fmt"
	"infini.sh/framework/core/util"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Parameters struct {
	Data   map[string]interface{} `json:"data,omitempty"`
	l      *sync.RWMutex
	inited bool
}

func (para *Parameters) init() {
	if para.inited {
		return
	}
	if para.l == nil {
		para.l = &sync.RWMutex{}
	}
	para.l.Lock()
	if para.Data == nil {
		para.Data = map[string]interface{}{}
	}
	para.inited = true
	para.l.Unlock()
}

func (para *Parameters) MustGetTime(key ParaKey) time.Time {
	v, ok := para.GetTime(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return v
}

func (para *Parameters) GetTime(key ParaKey) (time.Time, bool) {
	v := para.Get(key)
	s, ok := v.(time.Time)
	if ok {
		return s, ok
	}
	return s, ok
}

func (para *Parameters) GetString(key ParaKey) (string, bool) {
	v := para.Get(key)
	s, ok := v.(string)
	if ok {
		return s, ok
	}
	return s, ok
}

func (para *Parameters) GetBool(key ParaKey, defaultV bool) bool {
	v := para.Get(key)
	s, ok := v.(bool)
	if ok {
		return s
	}
	return defaultV
}

func (para *Parameters) Has(key ParaKey) bool {
	para.init()
	_, ok := para.Data[string(key)]
	return ok
}

func (para *Parameters) GetIntOrDefault(key ParaKey, defaultV int) int {
	v, ok := para.GetInt(key, defaultV)
	if ok {
		return v
	}
	return defaultV
}

func (para *Parameters) GetDurationOrDefault(key ParaKey, defaultV string) time.Duration {
	dur, err := time.ParseDuration(para.GetStringOrDefault(key,defaultV))
	if err!=nil{
		panic(err)
	}
	return dur
}

func (para *Parameters) GetInt(key ParaKey, defaultV int) (int, bool) {
	v, ok := para.GetInt64(key, 0)
	if ok {
		return int(v), ok
	}
	return defaultV, ok
}

func (para *Parameters) GetInt64OrDefault(key ParaKey, defaultV int64) int64 {
	v, ok := para.GetInt64(key, defaultV)
	if ok {
		return v
	}
	return defaultV
}

func (para *Parameters) GetFloat64OrDefault(key ParaKey, defaultV float64) float64 {
	v, ok := para.GetFloat64(key, defaultV)
	if ok {
		return v
	}
	return defaultV
}

func (para *Parameters) GetFloat32OrDefault(key ParaKey, defaultV float32) float32 {
	v, ok := para.GetFloat32(key, defaultV)
	if ok {
		return v
	}
	return defaultV
}

func (para *Parameters) GetFloat64(key ParaKey, defaultV float64) (float64,bool) {
	v := para.Get(key)

	s, ok := v.(float64)
	if ok {
		return s, ok
	}

	s1, ok := v.(float32)
	if ok {
		return float64(s1), ok
	}

	return defaultV,false
}
func (para *Parameters) GetFloat32(key ParaKey, defaultV float32) (float32,bool) {
	v := para.Get(key)

	s1, ok := v.(float32)
	if ok {
		return float32(s1), ok
	}

	s, ok := v.(float64)
	if ok {
		return float32(s), ok
	}

	return defaultV,false
}

func GetInt64OrDefault(v interface{},defaultV int64)(int64,bool)  {

	s, ok := v.(int64)
	if ok {
		return s, ok
	}

	s1, ok := v.(uint64)
	if ok {
		return int64(s1), ok
	}

	s2, ok := v.(int)
	if ok {
		return int64(s2), ok
	}

	s3, ok := v.(uint)
	if ok {
		return int64(s3), ok
	}

	return defaultV, ok
}

func (para *Parameters) GetInt64(key ParaKey, defaultV int64) (int64, bool) {
	v := para.Get(key)

	return GetInt64OrDefault(v,defaultV)

}

func (para *Parameters) MustGet(key ParaKey) interface{} {
	para.init()

	s := string(key)

	para.l.RLock()
	v, ok := para.Data[s]
	para.l.RUnlock()

	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}

	return v
}

func (para *Parameters) GetStringMap(key ParaKey) (result map[string]string, ok bool) {

	m, ok := para.GetMap(key)
	if ok {
		result = map[string]string{}
		for k, v := range m {
			result[k] = v.(string)
		}
		return result, ok
	}

	//try map string string
	f := para.Get(key)
	result, ok = f.(map[string]string)
	if ok {
		return result, ok
	}

	//try string array with map rule: key=>value
	array, ok := para.GetStringArray(key)
	if ok {
		result = map[string]string{}
		for _, v := range array {
			o := strings.Split(v, "->")
			result[util.TrimSpaces(o[0])] = util.TrimSpaces(o[1])
		}
	}
	return result, ok
}

func (para *Parameters) GetMap(key ParaKey) (map[string]interface{}, bool) {
	v := para.Get(key)
	s, ok := v.(map[string]interface{})
	return s, ok
}

func (para *Parameters) GetIntMapOrInit(key ParaKey) (map[string]int, bool) {
	v := para.Get(key)
	s, ok := v.(map[string]int)
	if !ok{
		v=map[string]int{}
		para.Set(key,v)
	}
	return s, ok
}

func (para *Parameters) Config(key ParaKey,obj interface{}) {
	rt := reflect.TypeOf(obj).Elem()
	if rt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("input must be struct, %v, %v",key,obj))
		return
	}

	paraObj, ok :=para.GetMap(key)
	if !ok{
		panic(errors.New(fmt.Sprintf("no config %v found in parameter",key)))
		return
	}

	newPara:=Parameters{Data: paraObj}
	mutable := reflect.ValueOf(obj).Elem()

	for i:=0;i<mutable.NumField();i++{
		f:=mutable.Field(i)
		tag:=rt.Field(i).Tag.Get("config")
		field:=mutable.FieldByName(rt.Field(i).Name)
		//fmt.Println("tag:",tag,",",rt.Field(i).Name,",",i,":",f.Type(),",",f.Kind(),",",f.String(),",",field)
		key:=ParaKey(tag)
		if newPara.Has(key){
			//fmt.Printf("config %s found in parameters,",key)
			switch f.Kind() {
			case reflect.Bool:
				field.SetBool(newPara.GetBool(key,false))
				break
			case reflect.String:
				field.SetString(newPara.GetStringOrDefault(key,""))
				mutable.FieldByName(rt.Field(i).Name).SetString("medcl")
				break
			case reflect.Int64:
			case reflect.Int32:
			case reflect.Int16:
			case reflect.Int8:
			case reflect.Int:
			case reflect.Uint64:
			case reflect.Uint32:
			case reflect.Uint16:
			case reflect.Uint:
				field.SetInt(newPara.GetInt64OrDefault(key,0))
				break
			case reflect.Float32:
			case reflect.Float64:
				field.SetFloat(newPara.GetFloat64OrDefault(key,0))
				break
				//Complex64
				//Complex128

				//Array
				//Interface
				//Map
				//Slice
				//Struct
			}
		}
	}

	//t := reflect.TypeOf(obj)
	//for i:=0;i<t.NumField();i++{
	//	f:=t.Field(i)
	//	fmt.Println(i,":",f)
	//}
	//v := reflect.New(t).Elem()

	//f := obj.FieldByName("Data")
	//if f.IsValid() && f.CanSet() && f.Kind() == reflect.Map {
	//	f.Set(reflect.ValueOf(cfg.Parameters))
	//}
}

func (para *Parameters) GetBytes(key ParaKey) ([]byte, bool) {
	v := para.Get(key)
	if reflect.TypeOf(v).Kind() == reflect.String {
		str := v.(string)
		s, err := base64.StdEncoding.DecodeString(str)
		ok := err != nil
		return s, ok
	} else {
		s, ok := v.([]byte)
		return s, ok
	}
}

func (para *Parameters) MustGetStringArray(key ParaKey) []string {
	result, ok := para.GetStringArray(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return result
}

func (para *Parameters) GetStringArray(key ParaKey) ([]string, bool) {
	array, ok := para.GetArray(key)
	var result []string
	if ok {
		result = []string{}
		for _, v := range array {
			result = append(result, v.(string))
		}
	}
	return result, ok
}

// GetArray will return a array which type of the items are interface {}
func (para *Parameters) GetArray(key ParaKey) ([]interface{}, bool) {
	v := para.Get(key)
	s, ok := v.([]interface{})
	return s, ok
}

func (para *Parameters) MustGetArray(key ParaKey) []interface{} {
	s, ok := para.GetArray(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return s
}

func (para *Parameters) Get(key ParaKey) interface{} {
	para.init()
	para.l.RLock()
	s := string(key)
	v := para.Data[s]
	//if global.Env().IsDebug {
	//	t := reflect.TypeOf(v)
	//	log.Debugf("parameter: %s %v %v", s, v, t)
	//}
	para.l.RUnlock()
	return v
}

func (para *Parameters) GetOrDefault(key ParaKey, val interface{}) interface{} {
	para.init()
	para.l.RLock()
	s := string(key)
	v := para.Data[s]
	para.l.RUnlock()
	if v == nil {
		return val
	}
	return v
}

func (para *Parameters) Set(key ParaKey, value interface{}) {
	para.init()
	para.l.Lock()
	s := string(key)
	para.Data[s] = value
	para.l.Unlock()
}

func (para *Parameters) MustGetString(key ParaKey) string {
	s, ok := para.GetString(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return s
}

func (para *Parameters) GetStringOrDefault(key ParaKey, val string) string {
	s, ok := para.GetString(key)
	if (!ok) || len(s) == 0 {
		return val
	}
	return s
}

func (para *Parameters) MustGetBytes(key ParaKey) []byte {
	s, ok := para.GetBytes(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return s
}

// MustGetInt return 0 if not key was found
func (para *Parameters) MustGetInt(key ParaKey) int {
	v, ok := para.GetInt(key, 0)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return v
}

func (para *Parameters) MustGetInt64(key ParaKey) int64 {
	s, ok := para.GetInt64(key, 0)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return s
}

func (para *Parameters) MustGetMap(key ParaKey) map[string]interface{} {
	s, ok := para.GetMap(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context", key))
	}
	return s
}

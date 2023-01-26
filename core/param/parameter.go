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
	"bytes"
	"encoding/base64"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Parameters struct {
	Timestamp time.Time
	Data      util.MapStr `json:"data,omitempty"`
	Meta      util.MapStr `json:"-"`
	l         sync.RWMutex
	inited    bool
}

var byteReaderPool = &sync.Pool{
	New: func() interface{} {
		return new(bytes.Reader)
	},
}

func (para *Parameters) ResetParameters() {
	para.l.Lock()
	para.Data = util.MapStr{}
	para.Meta = util.MapStr{}
	para.Timestamp = time.Now()
	para.inited = false
	//para.Data[uuid] = util.GetUUID()
	para.l.Unlock()
}

func (para *Parameters) init() {
	para.l.Lock()
	defer para.l.Unlock()

	if para.inited {
		return
	}

	if para.Data == nil {
		para.Data = util.MapStr{}
	}

	if para.Meta == nil {
		para.Meta = util.MapStr{}
	}
	//_,ok:=para.Data[uuid]
	//if !ok{
	//	para.Data[uuid] = util.GetUUID()
	//}
	para.inited = true
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
	if v == nil {
		return time.Time{}, false
	}

	s, ok := v.(time.Time)
	if ok {
		return s, ok
	}
	return s, ok
}

//func (para *Parameters) UUID() (string) {
//	return para.MustGetString(uuid)
//}

func (para *Parameters) GetString(key ParaKey) (string, bool) {
	v := para.Get(key)
	if v == nil {
		return "", false
	}

	s, ok := v.(string)
	if ok {
		return s, ok
	}
	return s, ok
}

func (para *Parameters) GetBool(key ParaKey, defaultV bool) bool {
	v := para.Get(key)
	if v != nil {
		s, ok := v.(bool)
		if ok {
			return s
		}
	}
	return defaultV
}

func (para *Parameters) Has(key ParaKey) bool {
	para.init()
	para.l.Lock()
	_, ok := para.Data[string(key)]
	para.l.Unlock()
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
	dur, err := time.ParseDuration(para.GetStringOrDefault(key, defaultV))
	if err != nil {
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

func (para *Parameters) GetFloat64(key ParaKey, defaultV float64) (float64, bool) {
	v := para.Get(key)
	if v == nil {
		return defaultV, false
	}

	s, ok := v.(float64)
	if ok {
		return s, ok
	}

	s1, ok := v.(float32)
	if ok {
		return float64(s1), ok
	}

	return defaultV, false
}
func (para *Parameters) GetFloat32(key ParaKey, defaultV float32) (float32, bool) {
	v := para.Get(key)
	if v == nil {
		return defaultV, false
	}

	s1, ok := v.(float32)
	if ok {
		return float32(s1), ok
	}

	s, ok := v.(float64)
	if ok {
		return float32(s), ok
	}

	return defaultV, false
}

func GetInt64OrDefault(v interface{}, defaultV int64) (int64, bool) {

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
	if v == nil {
		return defaultV, false
	}
	return GetInt64OrDefault(v, defaultV)
}

func (para *Parameters) MustGet(key ParaKey) interface{} {
	para.init()

	s := string(key)
	para.l.Lock()
	v, ok := para.Data[s]
	para.l.Unlock()
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
			result[k], ok = v.(string)
		}
		return result, ok
	}

	//try map string string
	f := para.Get(key)
	if f == nil {
		return nil, false
	}
	result, ok = f.(map[string]string)
	if ok {
		return result, ok
	}

	//try string array with map rule: key=>value
	array, ok := para.GetStringArray(key)
	if ok {
		result = map[string]string{}
		for _, v := range array {
			k1, v1, err := util.ConvertStringToMap(v, "->")
			if err != nil {
				panic(err)
			}
			result[k1] = v1
		}
	}
	return result, ok
}

func (para *Parameters) GetMapArray(key ParaKey) ([]map[string]interface{}, bool) {
	v := para.Get(key)
	if v == nil {
		return nil, false
	}

	s, ok := v.([]interface{})
	f := []map[string]interface{}{}
	for _, m := range s {
		y, ok := m.(map[string]interface{})
		if ok {
			f = append(f, y)
		}
	}
	return f, ok
}

func (para *Parameters) GetMap(key ParaKey) (map[string]interface{}, bool) {
	v := para.Get(key)
	if v == nil {
		return nil, false
	}
	s, ok := v.(map[string]interface{})
	return s, ok
}

func (para *Parameters) GetIntMapOrInit(key ParaKey) (map[string]int, bool) {
	v := para.Get(key)
	if v == nil {
		return nil, false
	}

	s, ok := v.(map[string]int)
	if !ok {
		v = map[string]int{}
		para.Set(key, v)
	}
	return s, ok
}

func (para *Parameters) Config(key ParaKey, obj interface{}) {
	if obj == nil {
		panic(errors.New("config object can't be nil"))
	}
	paraObj, ok := para.GetMap(key)

	if !ok {
		panic(errors.New(fmt.Sprintf("no config [%v] found in parameter", key)))
		return
	}

	objType := reflect.TypeOf(obj)
	rt := objType.Elem()
	newPara := Parameters{Data: paraObj}
	mutable := reflect.ValueOf(obj).Elem()
	newPara.ConfigBinding(key, rt, &mutable)
}
func (newPara *Parameters) ConfigBinding(key ParaKey, rt reflect.Type, mutable *reflect.Value) {

	if !mutable.IsValid() {
		log.Errorf("invalid config [%v] %v", key)
		return
	}

	for i := 0; i < mutable.NumField(); i++ {
		f := mutable.Field(i)
		tag := rt.Field(i).Tag.Get("config")
		field := mutable.FieldByName(rt.Field(i).Name)

		key := ParaKey(tag)
		//fmt.Println("tag: ", tag," key: ",key," has para: ",newPara.Has(key),newPara.Data, ",", rt.Field(i).Name, ",", i, ":", f.Type(), ", kind:", f.Kind(), ",", f.String(), ",", field)

		//if global.Env().IsDebug {
		//	log.Trace("tag: ", tag, " key: ", key, " has para: ", newPara.Has(key), newPara.Data, ",", rt.Field(i).Name, ",", i, ":", f.Type(), ", kind:", f.Kind(), ",", f.String(), ",", field)
		//}

		if newPara.Has(key) {
			switch f.Kind() {
			case reflect.Bool:
				field.SetBool(newPara.GetBool(key, false))
				break
			case reflect.String:
				field.SetString(newPara.GetStringOrDefault(key, ""))
				break
			case reflect.Int64:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Int32:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Int16:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Int8:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Int:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Uint64:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Uint32:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Uint16:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Uint:
				field.SetInt(newPara.GetInt64OrDefault(key, 0))
				break
			case reflect.Float32:
				field.SetFloat(newPara.GetFloat64OrDefault(key, 0))
				break
			case reflect.Float64:
				field.SetFloat(newPara.GetFloat64OrDefault(key, 0))
				break
				//Complex64
				//Complex128
				//Array
				//Interface
			//case reflect.Array:
			//	break
			case reflect.Map:
				paraObj, ok := newPara.GetMap(key)
				if ok {
					field.Set(reflect.MakeMap(field.Type()))
					for k, v := range paraObj {
						field.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
					}
				}
				break
			case reflect.Struct:
				paraObj, ok := newPara.GetMap(key)
				if ok {
					v2 := reflect.New(field.Type()).Elem()
					newPara := Parameters{Data: paraObj}
					newPara.ConfigBinding(key, field.Type(), &v2)
					field.Set(v2)
				}
				break
			case reflect.Slice:
				arr, ok := newPara.GetArray(key)
				//fmt.Println("key:",key,"array,",arr,",ok:",ok)
				if ok {
					one := reflect.ValueOf(arr[0])
					targetItemsType := field.Type().Elem()
					slice := reflect.MakeSlice(reflect.SliceOf(targetItemsType), len(arr), len(arr))
					for i := 0; i < len(arr); i++ {
						one = reflect.ValueOf(arr[i])
						v := slice.Index(i)
						v.Set(one)
						v = slice.Index(i)
					}
					field.Set(slice)
				}
				break
			default:
				log.Errorf("type not handled: [%v]", f.Kind())
				break
			}
		}
	}

}

func (para *Parameters) GetBytes(key ParaKey) ([]byte, bool) {
	v := para.Get(key)
	if v == nil {
		return nil, false
	}

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
			x, ok := v.(string)
			if ok {
				result = append(result, x)
			}
		}
	}
	return result, ok
}

func (para *Parameters) GetInt64Array(key ParaKey) ([]int64, bool) {
	array, ok := para.GetArray(key)
	//fmt.Println(array,ok)

	var result []int64
	if ok {
		result = []int64{}
		for _, v := range array {
			x, ok := GetInt64OrDefault(v, 0)
			//fmt.Println(x,ok,reflect.TypeOf(v))
			if ok {
				result = append(result, x)
			}
		}
	}
	return result, ok
}

// GetArray will return a array which type of the items are interface {}
func (para *Parameters) GetArray(key ParaKey) ([]interface{}, bool) {

	//TODO cache

	v := para.Get(key)

	if v == nil {
		return []interface{}{}, false
	}

	s, ok := v.([]interface{})
	if ok {
		return s, ok
	}

	s1, ok := v.([]string)
	if ok {
		for _, v1 := range s1 {
			s = append(s, v1)
		}
		return s, ok
	}

	s2, ok := v.([]int)
	if ok {
		for _, v1 := range s2 {
			s = append(s, v1)
		}
		return s, ok
	}

	s3, ok := v.([]int32)
	if ok {
		for _, v1 := range s3 {
			s = append(s, v1)
		}
		return s, ok
	}

	s4, ok := v.([]int64)
	if ok {
		for _, v1 := range s4 {
			s = append(s, v1)
		}
		return s, ok
	}

	s5, ok := v.([]float32)
	if ok {
		for _, v1 := range s5 {
			s = append(s, v1)
		}
		return s, ok
	}

	s6, ok := v.([]float64)
	if ok {
		for _, v1 := range s6 {
			s = append(s, v1)
		}
		return s, ok
	}

	s7, ok := v.([]uint64)
	if ok {
		for _, v1 := range s7 {
			s = append(s, v1)
		}
		return s, ok
	}

	//TODO handle rest types
	//if global.Env().IsDebug {
		log.Warnf("parameters failed to GetArray, key: %v, type: %v", key, reflect.TypeOf(v))
	//}
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
	v, _ := para.GetValue(string(key))
	return v
}

func (para *Parameters) GetOrDefault(key ParaKey, val interface{}) interface{} {
	para.init()
	s := string(key)
	para.l.Lock()
	v := para.Data[s]
	para.l.Unlock()
	if v == nil {
		return val
	}
	return v
}

//TODO remove lock async, channel
func (para *Parameters) Set(key ParaKey, value interface{}) {
	para.init()
	para.PutValue(string(key), value)
}

func (para *Parameters) MustGetString(key ParaKey) string {
	s, ok := para.GetString(key)
	if !ok {
		panic(fmt.Errorf("%s not found in context, %v", key, para.Data))
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

const (
	FieldsKey         = "fields"
	TagsKey           = "tags"
	timestampFieldKey = "@timestamp"
	metadataFieldKey  = "@metadata"
)

var (
	errNoTimestamp = errors.New("value is no timestamp")
	errNoMapStr    = errors.New("value is no map[string]interface{} type")
	ErrKeyNotFound = errors.New("key not found")
)

func (e *Parameters) setTimestamp(v interface{}) error {
	switch ts := v.(type) {
	case time.Time:
		e.Timestamp = ts
	case util.Time:
		e.Timestamp = time.Time(ts)
	default:
		return errNoTimestamp
	}
	return nil
}

func (e *Parameters) GetValue(key string) (interface{}, error) {
	e.init()

	e.l.RLock()
	defer e.l.RUnlock()

	if key == timestampFieldKey {
		return e.Timestamp, nil
	} else if subKey, ok := metadataKey(key); ok {
		if subKey == "" || e.Meta == nil {
			return e.Meta, nil
		}
		return e.Meta.GetValue(subKey)
	}
	return e.Data.GetValue(key)
}

func (e *Parameters) PutValue(key string, v interface{}) (interface{}, error) {
	e.init()

	e.l.Lock()
	defer e.l.Unlock()

	if key == timestampFieldKey {
		err := e.setTimestamp(v)
		return nil, err
	} else if subKey, ok := metadataKey(key); ok {
		if subKey == "" {
			switch meta := v.(type) {
			case util.MapStr:
				e.Meta = meta
			case map[string]interface{}:
				e.Meta = meta
			default:
				return nil, errNoMapStr
			}
		} else if e.Meta == nil {
			e.Meta = util.MapStr{}
		}
		return e.Meta.Put(subKey, v)
	}

	return e.Data.Put(key, v)
}

func (e *Parameters) Delete(key string) error {
	e.init()

	if subKey, ok := metadataKey(key); ok {
		if subKey == "" {
			e.Meta = nil
			return nil
		}
		if e.Meta == nil {
			return nil
		}
		return e.Meta.Delete(subKey)
	}
	return e.Data.Delete(key)
}

func metadataKey(key string) (string, bool) {
	if !strings.HasPrefix(key, metadataFieldKey) {
		return "", false
	}

	subKey := key[len(metadataFieldKey):]
	if subKey == "" {
		return "", true
	}
	if subKey[0] == '.' {
		return subKey[1:], true
	}
	return "", false
}

// SetErrorWithOption sets jsonErr value in the event fields according to addErrKey value.
func (e *Parameters) SetErrorWithOption(jsonErr util.MapStr, addErrKey bool) {
	e.init()
	if addErrKey {
		e.Data["error"] = jsonErr
	}
}

// AddTags appends a tag to the tags field of ms. If the tags field does not
// exist then it will be created. If the tags field exists and is not a []string
// then an error will be returned. It does not deduplicate the list of tags.
func (para *Parameters) AddTags(tags []string) error {
	return para.AddTagsWithKey(TagsKey, tags)
}

func (para *Parameters) GetTags() (map[string]string, bool) {
	return para.GetStringMap(TagsKey)
}

func (para *Parameters) UpdateTags(addTags, removeTags []string) {
	temp, ok := para.GetStringMap(TagsKey)
	if !ok {
		temp = map[string]string{}
	}

	if addTags != nil {
		for _, v := range addTags {
			temp[v] = v
		}
	}

	if removeTags != nil {
		for _, v := range removeTags {
			delete(temp, v)
		}
	}

	para.Set(TagsKey, temp)
}

// AddTagsWithKey appends a tag to the key field of ms. If the field does not
// exist then it will be created. If the field exists and is not a []string
// then an error will be returned. It does not deduplicate the list.
func (para *Parameters) AddTagsWithKey(key string, tags []string) error {
	if para == nil || len(tags) == 0 {
		return nil
	}

	k, subMap, oldTags, present, err := mapFind(key, para.Data, true)
	if err != nil {
		return err
	}

	if !present {
		subMap[k] = tags
		return nil
	}

	switch arr := oldTags.(type) {
	case []string:
		subMap[k] = append(arr, tags...)
	case []interface{}:
		for _, tag := range tags {
			arr = append(arr, tag)
		}
		subMap[k] = arr
	default:
		return errors.Errorf("expected string array by type is %T", oldTags)

	}
	return nil
}

// mapFind iterates a MapStr based on a the given dotted key, finding the final
// subMap and subKey to operate on.
// An error is returned if some intermediate is no map or the key doesn't exist.
// If createMissing is set to true, intermediate maps are created.
// The final map and un-dotted key to run further operations on are returned in
// subKey and subMap. The subMap already contains a value for subKey, the
// present flag is set to true and the oldValue return will hold
// the original value.
func mapFind(
	key string,
	data util.MapStr,
	createMissing bool,
) (subKey string, subMap util.MapStr, oldValue interface{}, present bool, err error) {
	// XXX `safemapstr.mapFind` mimics this implementation, both should be updated to have similar behavior

	for {
		// Fast path, key is present as is.
		if v, exists := data[key]; exists {
			return key, data, v, true, nil
		}

		idx := strings.IndexRune(key, '.')
		if idx < 0 {
			return key, data, nil, false, nil
		}

		k := key[:idx]
		d, exists := data[k]
		if !exists {
			if createMissing {
				d = util.MapStr{}
				data[k] = d
			} else {
				return "", nil, nil, false, ErrKeyNotFound
			}
		}

		v, err := toMapStr(d)
		if err != nil {
			return "", nil, nil, false, err
		}

		// advance to sub-map
		key = key[idx+1:]
		data = v
	}
}

// toMapStr performs a type assertion on v and returns a MapStr. v can be either
// a MapStr or a map[string]interface{}. If it's any other type or nil then
// an error is returned.
func toMapStr(v interface{}) (util.MapStr, error) {
	m, ok := tryToMapStr(v)
	if !ok {
		return nil, errors.Errorf("expected map but type is %T", v)
	}
	return m, nil
}

func tryToMapStr(v interface{}) (util.MapStr, bool) {
	switch m := v.(type) {
	case util.MapStr:
		return m, true
	case map[string]interface{}:
		return util.MapStr(m), true
	default:
		return nil, false
	}
}

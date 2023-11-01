//copied from github.com/elastic/beats
//https://github.com/elastic/beats/blob/master/LICENSE
//Licensed under the Apache License, Version 2.0 (the "License");

package util

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// Event metadata constants. These keys are used to identify
// metadata stored in an event.
const (
	FieldsKey = "fields"
	TagsKey   = "tags"
)

var (
	// ErrKeyNotFound indicates that the specified key was not found.
	ErrKeyNotFound = errors.New("key not found")
)

// EventMetadata contains fields and tags that can be added to an event via
// configuration.
type EventMetadata struct {
	Fields          MapStr
	FieldsUnderRoot bool `config:"fields_under_root"`
	Tags            []string
}

// MapStr is a map[string]interface{} wrapper with utility methods for common
// map operations like converting to JSON.
type MapStr map[string]interface{}

// Update copies all the key-value pairs from d to this map. If the key
// already exists then it is overwritten. This method does not merge nested
// maps.
func (m MapStr) Update(d MapStr) {
	for k, v := range d {
		m[k] = v
	}
}

// Delete deletes the given key from the map.
func (m MapStr) Delete(key string) error {
	_, err := walkMap(key, m, opDelete)
	return err
}

// CopyFieldsTo copies the field specified by key to the given map. It will
// overwrite the key if it exists. An error is returned if the key does not
// exist in the source map.
func (m MapStr) CopyFieldsTo(to MapStr, key string) error {
	v, err := walkMap(key, m, opGet)
	if err != nil {
		return err
	}

	_, err = walkMap(key, to, mapStrOperation{putOperation{v}, true})
	return err
}

// Clone returns a copy of the MapStr. It recursively makes copies of inner
// maps.
func (m MapStr) Clone() MapStr {
	result := MapStr{}

	for k, v := range m {
		innerMap, err := toMapStr(v)
		if err == nil {
			result[k] = innerMap.Clone()
		} else {
			result[k] = v
		}
	}

	return result
}

// HasKey returns true if the key exist. If an error occurs then false is
// returned with a non-nil error.
func (m MapStr) SafetyHasKey(key string) (bool) {
	ok,err:=m.HasKey(key)
	if err!=nil{
		return false
	}
	return ok
}

func (m MapStr) HasKey(key string) (bool, error) {
	hasKey, err := walkMap(key, m, opHasKey)
	if err != nil {
		return false, err
	}

	return hasKey.(bool), nil
}

// GetValue gets a value from the map. If the key does not exist then an error
// is returned.
func (m MapStr) GetValue(key string) (interface{}, error) {
	return walkMap(key, m, opGet)
}

// Put associates the specified value with the specified key. If the map
// previously contained a mapping for the key, the old value is replaced and
// returned. The key can be expressed in dot-notation (e.g. x.y) to put a value
// into a nested map.
//
// If you need insert keys containing dots then you must use bracket notation
// to insert values (e.g. m[key] = value).
func (m MapStr) Put(key string, value interface{}) (interface{}, error) {
	return walkMap(key, m, mapStrOperation{putOperation{value}, true})
}

// StringToPrint returns the MapStr as pretty JSON.
func (m MapStr) StringToPrint() string {
	json, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Sprintf("Not valid json: %v", err)
	}
	return string(json)
}

// String returns the MapStr as JSON.
func (m MapStr) String() string {
	bytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("Not valid json: %v", err)
	}
	return string(bytes)
}

// MapStrUnion creates a new MapStr containing the union of the
// key-value pairs of the two maps. If the same key is present in
// both, the key-value pairs from dict2 overwrite the ones from dict1.
func MapStrUnion(dict1 MapStr, dict2 MapStr) MapStr {
	dict := MapStr{}

	for k, v := range dict1 {
		dict[k] = v
	}

	for k, v := range dict2 {
		dict[k] = v
	}
	return dict
}

// MergeFields merges the top-level keys and values in each source map (it does
// not perform a deep merge). If the same key exists in both, the value in
// fields takes precedence. If underRoot is true then the contents of the fields
// MapStr is merged with the value of the 'fields' key in ms.
//
// An error is returned if underRoot is true and the value of ms.fields is not a
// MapStr.
func MergeFields(ms, fields MapStr, underRoot bool) error {
	if ms == nil || len(fields) == 0 {
		return nil
	}

	fieldsMS := ms
	if !underRoot {
		f, ok := ms[FieldsKey]
		if !ok {
			fieldsMS = make(MapStr, len(fields))
			ms[FieldsKey] = fieldsMS
		} else {
			// Use existing 'fields' value.
			var err error
			fieldsMS, err = toMapStr(f)
			if err != nil {
				return err
			}
		}
	}

	// Add fields and override.
	for k, v := range fields {
		fieldsMS[k] = v
	}

	return nil
}

// AddTags appends a tag to the tags field of ms. If the tags field does not
// exist then it will be created. If the tags field exists and is not a []string
// then an error will be returned. It does not deduplicate the list of tags.
func AddTags(ms MapStr, tags []string) error {
	if ms == nil || len(tags) == 0 {
		return nil
	}

	tagsIfc, ok := ms[TagsKey]
	if !ok {
		ms[TagsKey] = tags
		return nil
	}

	existingTags, ok := tagsIfc.([]string)
	if !ok {
		return errors.Errorf("expected string array by type is %T", tagsIfc)
	}

	ms[TagsKey] = append(existingTags, tags...)
	return nil
}

// toMapStr performs a type assertion on v and returns a MapStr. v can be either
// a MapStr or a map[string]interface{}. If it's any other type or nil then
// an error is returned.
func toMapStr(v interface{}) (MapStr, error) {
	switch v.(type) {
	case MapStr:
		return v.(MapStr), nil
	case map[string]interface{}:
		m := v.(map[string]interface{})
		return MapStr(m), nil
	default:
		// Convert slices to maps for array indices support.
		if kind := reflect.TypeOf(v).Kind(); kind == reflect.Slice || kind == reflect.Array {
			m := map[string]interface{}{}
			s := reflect.ValueOf(v)
			for i := 0; i < s.Len(); i++ {
				m[strconv.Itoa(i)] = s.Index(i).Interface()
			}
			return MapStr(m), nil
		}
		return nil, errors.Errorf("expected map but type is %T", v)
	}
}

// walkMap walks the data MapStr to arrive at the value specified by the key.
// The key is expressed in dot-notation (eg. x.y.z). When the key is found then
// the given mapStrOperation is invoked.
func walkMap(key string, data MapStr, op mapStrOperation) (interface{}, error) {

	//try check map directly first
	if _, ok := data[key]; ok {
		if ok{
			return op.Do(key, data)
		}
	}

	var err error
	keyParts := strings.Split(key, ".")

	// Walk maps until reaching a leaf object.
	m := data
	for i, k := range keyParts[0 : len(keyParts)-1] {
		v, exists := m[k]
		if !exists {
			if op.CreateMissingKeys {
				newMap := MapStr{}
				m[k] = newMap
				m = newMap
				continue
			}
			return nil, errors.Wrapf(ErrKeyNotFound, "key=%v", strings.Join(keyParts[0:i+1], "."))
		}

		m, err = toMapStr(v)
		if err != nil {
			return nil, errors.Wrapf(err, "key=%v", strings.Join(keyParts[0:i+1], "."))
		}
	}

	// Execute the mapStrOperator on the leaf object.
	v, err := op.Do(keyParts[len(keyParts)-1], m)
	if err != nil {
		return nil, errors.Wrapf(err, "key=%v", key)
	}

	return v, nil
}

// mapStrOperation types

// These are static mapStrOperation types that store no state and are reusable.
var (
	opDelete = mapStrOperation{deleteOperation{}, false}
	opGet    = mapStrOperation{getOperation{}, false}
	opHasKey = mapStrOperation{hasKeyOperation{}, false}
)

// mapStrOperation represents an operation that can be applied to map.
type mapStrOperation struct {
	mapStrOperator
	CreateMissingKeys bool
}

// mapStrOperator is an interface with a single function that performs an
// operation on a MapStr.
type mapStrOperator interface {
	Do(key string, data MapStr) (value interface{}, err error)
}

type deleteOperation struct{}

func (op deleteOperation) Do(key string, data MapStr) (interface{}, error) {
	value, found := data[key]
	if !found {
		return nil, ErrKeyNotFound
	}
	delete(data, key)
	return value, nil
}

type getOperation struct{}

func (op getOperation) Do(key string, data MapStr) (interface{}, error) {
	value, found := data[key]
	if !found {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

type hasKeyOperation struct{}

func (op hasKeyOperation) Do(key string, data MapStr) (interface{}, error) {
	_, found := data[key]
	return found, nil
}

type putOperation struct {
	Value interface{}
}

func (op putOperation) Do(key string, data MapStr) (interface{}, error) {
	existingValue, _ := data[key]
	data[key] = op.Value
	return existingValue, nil
}


// Flatten flattens the given MapStr and returns a flat MapStr.
//
// Example:
//   "hello": MapStr{"world": "test" }
//
// This is converted to:
//   "hello.world": "test"
//
// This can be useful for testing or logging.
func (m MapStr) Flatten() MapStr {
	return flatten("", m, MapStr{})
}

// flatten is a helper for Flatten. See docs for Flatten. For convenience the
// out parameter is returned.
func flatten(prefix string, in, out MapStr) MapStr {
	for k, v := range in {
		var fullKey string
		if prefix == "" {
			fullKey = k
		} else {
			fullKey = prefix + "." + k
		}

		if m, ok := tryToMapStr(v); ok {
			flatten(fullKey, m, out)
		} else {
			out[fullKey] = v
		}
	}
	return out
}

func tryToMapStr(v interface{}) (MapStr, bool) {
	switch m := v.(type) {
	case MapStr:
		return m, true
	case map[string]interface{}:
		return MapStr(m), true
	default:
		return nil, false
	}
}

type KV struct {
	Key   string   `json:"key,omitempty"`
	Value string   `json:"value,omitempty"`
}

func GetIntMapKeys(m map[int]int)[]int  {
	keys:=[]int{}
	for k,_:=range m{
		keys=append(keys,k)
	}
	return keys
}

func GetStringIntMapKeys(m map[string]int)[]string  {
	keys:=[]string{}
	for k,_:=range m{
		keys=append(keys,k)
	}
	return keys
}

func GetMapKeys(m map[string]KV)[]string  {
	keys:=[]string{}
	for k,_:=range m{
		keys=append(keys,k)
	}
	return keys
}


func GetMapValueByKeys(keys []string,m map[string]interface{})(interface{},bool)  {

	for i,key:=range keys{
		v,ok:=m[key]
		if i==len(keys)-1{
			return v,ok
		}

		if ok{
			x,ok:=v.(map[string]interface{})
			if ok{
				m=x
			}else{
				return v,true
			}
		}
	}
	return nil,false
}

func GetValueByKeys(keys []string,m MapStr)(interface{},bool)  {

	for i,key:=range keys{
		v,ok:=m[key]
		if i==len(keys)-1{
			return v,ok
		}
		if ok{
			x,ok:=v.(MapStr)
			if ok{
				m=x
			}else{
				return v,true
			}
		}
	}
	return nil,false
}

func (m MapStr) Equals(dst MapStr) bool {
	a := m.Flatten()
	b := dst.Flatten()
	if len(a) != len(b){
		return false
	}
	for k, v := range a {
		if !reflect.DeepEqual(v, b[k]) {
			return false
		}
	}
	return true
}

// AddTagsWithKey appends a tag to the key field of ms. If the field does not
// exist then it will be created. If the field exists and is not a []string
// then an error will be returned. It does not deduplicate the list.
func AddTagsWithKey(ms MapStr, key string, tags []string) error {
	if ms == nil || len(tags) == 0 {
		return nil
	}

	k, subMap, oldTags, present, err := mapFind(key, ms, true)
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
		return fmt.Errorf("expected string array by type is %T", oldTags)

	}
	return nil
}

// mapFind iterates a M based on a the given dotted key, finding the final
// subMap and subKey to operate on.
// An error is returned if some intermediate is no map or the key doesn't exist.
// If createMissing is set to true, intermediate maps are created.
// The final map and un-dotted key to run further operations on are returned in
// subKey and subMap. The subMap already contains a value for subKey, the
// present flag is set to true and the oldValue return will hold
// the original value.
func mapFind(
	key string,
	data MapStr,
	createMissing bool,
) (subKey string, subMap MapStr, oldValue interface{}, present bool, err error) {
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
				d = MapStr{}
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

// DeepUpdate recursively copies the key-value pairs from d to this map.
// If the key is present and a map as well, the sub-map will be updated recursively
// via DeepUpdate.
// DeepUpdateNoOverwrite is a version of this function that does not
// overwrite existing values.
func (m MapStr) DeepUpdate(d MapStr) {
	m.deepUpdateMap(d, true)
}

func (m MapStr) deepUpdateMap(d MapStr, overwrite bool) {
	for k, v := range d {
		switch val := v.(type) {
		case map[string]interface{}:
			m[k] = deepUpdateValue(m[k], MapStr(val), overwrite)
		case MapStr:
			m[k] = deepUpdateValue(m[k], val, overwrite)
		default:
			if overwrite {
				m[k] = v
			} else if _, exists := m[k]; !exists {
				m[k] = v
			}
		}
	}
}

func (m *MapStr) Merge(vars map[string]interface{}) {
	for k, v := range vars {
		m.Put(k,v)
	}
}

func deepUpdateValue(old interface{}, val MapStr, overwrite bool) interface{} {
	switch sub := old.(type) {
	case MapStr:
		if sub == nil {
			return val
		}

		sub.deepUpdateMap(val, overwrite)
		return sub
	case map[string]interface{}:
		if sub == nil {
			return val
		}

		tmp := MapStr(sub)
		tmp.deepUpdateMap(val, overwrite)
		return tmp
	default:
		// We reach the default branch if old is no map or if old == nil.
		// In either case we return `val`, such that the old value is completely
		// replaced when merging.
		return val
	}
}



func GetSyncMapSize(m *sync.Map) int {
	var i int
	m.Range(func(k, v interface{}) bool {
		i++
		return true
	})
	return i
}

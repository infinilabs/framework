// Copyright © 2016 Charles Phillips <charles@doublerebel.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
// https://github.com/doublerebel/bellows/blob/master/LICENSE

package util

import (
	"encoding/json"
	"reflect"
	"strings"
)

func Expand(value map[string]interface{}) map[string]interface{} {
	return ExpandPrefixed(value, "")
}

func ExpandPrefixed(value map[string]interface{}, prefix string) map[string]interface{} {
	m := make(map[string]interface{})
	ExpandPrefixedToResult(value, prefix, m)
	return m
}

func ExpandPrefixedToResult(value map[string]interface{}, prefix string, result map[string]interface{}) {
	if prefix != "" {
		prefix += "."
	}
	for k, val := range value {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		key := k[len(prefix):]
		idx := strings.Index(key, ".")
		if idx != -1 {
			key = key[:idx]
		}
		if _, ok := result[key]; ok {
			continue
		}
		if idx == -1 {
			result[key] = val
			continue
		}

		// It contains a period, so it is a more complex structure
		result[key] = ExpandPrefixed(value, k[:len(prefix)+len(key)])
	}
}

func Flatten(value interface{}, ignoreNil bool) map[string]interface{} {
	return FlattenPrefixed(value, "", ignoreNil)
}

func FlattenJSON(value interface{}, ignoreNil bool) (m map[string]interface{}) {
	m = make(map[string]interface{})
	FlattenPrefixedToResultWithTagAsFieldName(value, "", m, "json", ignoreNil)
	return m
}

func FlattenPrefixed(value interface{}, prefix string, ignoreNil bool) map[string]interface{} {
	m := make(map[string]interface{})
	FlattenPrefixedToResult(value, prefix, m, ignoreNil)
	return m
}

func FlattenPrefixedToResult(value interface{}, prefix string, m map[string]interface{}, ignoreNil bool) {
	FlattenPrefixedToResultWithTagAsFieldName(value, prefix, m, "", ignoreNil)
}

func FlattenPrefixedToResultWithTagAsFieldName(value interface{}, prefix string, m map[string]interface{}, tag string, ignoreNil bool) {
	base := ""
	if prefix != "" {
		base = prefix + "."
	}

	original := reflect.ValueOf(value)
	kind := original.Kind()
	if kind == reflect.Ptr || kind == reflect.Interface {
		original = reflect.Indirect(original)
		kind = original.Kind()
	}
	t := original.Type()

	switch kind {
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			break
		}
		for _, childKey := range original.MapKeys() {
			childValue := original.MapIndex(childKey)

			if childValue.IsNil() && ignoreNil {
				break
			}

			FlattenPrefixedToResult(childValue.Interface(), base+childKey.String(), m, ignoreNil)
		}
	case reflect.Struct:
		for i := 0; i < original.NumField(); i += 1 {
			childValue := original.Field(i)

			childKey := t.Field(i).Name
			if tag != "" {
				v := TrimSpaces(t.Field(i).Tag.Get(tag))
				if v != "" {
					childKey = v
				}
			}

			if childValue.Type().Kind() == reflect.Ptr {
				break
			}

			FlattenPrefixedToResult(childValue.Interface(), base+childKey, m, ignoreNil)
		}
	default:

		if ignoreNil && kind == reflect.String && original.String() == "" {
			break
		}

		if ignoreNil && kind == reflect.Bool && original.Bool() == false {
			break
		}

		if ignoreNil && (kind == reflect.Int ||
			kind == reflect.Int8 ||
			kind == reflect.Int16 ||
			kind == reflect.Int32 ||
			kind == reflect.Int64 ||
			kind == reflect.Uint ||
			kind == reflect.Uint8 ||
			kind == reflect.Uint16 ||
			kind == reflect.Uint32 ||
			kind == reflect.Uint64) &&
			original.Int() == 0 {
			break
		}

		if ignoreNil && (kind == reflect.Float64 || kind == reflect.Float32) && original.Float() == 0 {
			break
		}

		if prefix != "" {
			m[prefix] = value
		}
	}
}

func FlattenJSONString(nestedstr, prefix string, ignoreNil bool) (string, error) {
	var nested map[string]interface{}
	err := json.Unmarshal([]byte(nestedstr), &nested)
	if err != nil {
		return "", err
	}

	flatmap := FlattenPrefixed(nested, prefix, ignoreNil)
	if err != nil {
		return "", err
	}

	flatb, err := json.Marshal(&flatmap)
	if err != nil {
		return "", err
	}

	return string(flatb), nil
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"encoding/json"
	"reflect"
	"strings"

	"infini.sh/framework/lib/go-ucfg"
)

var secretStringType = reflect.TypeOf(ucfg.SecretString(""))

// MustToJSONBytesWithSecrets marshals v to JSON with every embedded
// ucfg.SecretString replaced by its resolved plaintext value.
//
// It exists for STORAGE round-trips (e.g. the sqlite ORM persists whole
// objects as JSON blobs): SecretString.MarshalJSON intentionally emits
// the "******" mask for display, which would destroy plain secrets on
// persist and break every later read (test-connection uses the in-memory
// value and passes, but anything re-loaded from storage authenticates
// with the mask and gets a 401).
//
// Implementation note: the masked form is produced first (so all other
// json.Marshaler semantics, tags and omitempty behavior are preserved
// exactly), then the resolved secret values are patched into the decoded
// generic tree along the reflecting walk of v. The input is never
// mutated and no SecretString copy is re-marshaled (which would mask
// again).
func MustToJSONBytesWithSecrets(v interface{}) []byte {
	masked, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	var root interface{}
	if err := json.Unmarshal(masked, &root); err != nil {
		return masked // not an object/array graph (scalar, raw literal): as-is
	}
	patchSecrets(root, reflect.ValueOf(v))
	out, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	return out
}

// patchSecrets walks the marshaled node tree in lockstep with the source
// value and replaces masked secret entries with their resolved values.
func patchSecrets(node interface{}, v reflect.Value) {
	m, ok := node.(map[string]interface{})
	if !ok {
		return
	}
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported: not in JSON
		}
		name := jsonFieldName(f)
		if name == "-" {
			continue
		}
		fv := v.Field(i)

		// Embedded unnamed struct (or pointer to one): fields are inlined
		// into the parent object; patch against the same map.
		if name == "" && f.Anonymous {
			patchSecrets(m, fv)
			continue
		}
		if name == "" {
			name = f.Name
		}
		child, exists := m[name]
		if !exists {
			continue // omitted (omitempty / empty) or custom-marshaled away
		}

		if fv.Type() == secretStringType {
			m[name] = ucfg.SecretString(fv.String()).Get()
			continue
		}

		switch fv.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Struct:
			patchSecrets(child, fv)
		case reflect.Slice, reflect.Array:
			arr, ok := child.([]interface{})
			if !ok {
				continue
			}
			for idx := 0; idx < fv.Len() && idx < len(arr); idx++ {
				patchSecretElement(arr, idx, fv.Index(idx))
			}
		case reflect.Map:
			cm, ok := child.(map[string]interface{})
			if !ok {
				continue
			}
			patchSecretMap(cm, fv)
		}
	}
}

func patchSecretElement(arr []interface{}, idx int, ev reflect.Value) {
	if ev.Type() == secretStringType {
		arr[idx] = ucfg.SecretString(ev.String()).Get()
		return
	}
	switch ev.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Struct:
		patchSecrets(arr[idx], ev)
	}
}

func patchSecretMap(cm map[string]interface{}, mv reflect.Value) {
	elemType := mv.Type().Elem()
	iter := mv.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		val := iter.Value()
		cur, exists := cm[key]
		if !exists {
			continue
		}
		if elemType == secretStringType {
			cm[key] = ucfg.SecretString(val.String()).Get()
			continue
		}
		switch val.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Struct:
			patchSecrets(cur, val)
		}
	}
}

// jsonFieldName returns the effective JSON object key of a struct field
// (the json tag name, or "" when the tag is absent).
func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	return parts[0]
}

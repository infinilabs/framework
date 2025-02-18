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

package util

import (
	"encoding/json"
	"reflect"
	"strings"
)

// MaskFields masks fields with the `secured` tag
func MaskFields(obj interface{}) {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		// Check if the field has the `secured` tag
		if strings.Contains(jsonTag, "secured") {
			f := v.Field(i)
			if f.Kind() == reflect.String {
				originalValue := f.String()
				maskedValue := strings.Repeat("*", len(originalValue))
				f.SetString(maskedValue)
			}
		}
	}
}

// MaskAndMarshal applies masking during serialization
// MaskAndMarshal masks the secured fields and serializes the object to JSON.
func MaskAndMarshal(obj interface{}) ([]byte, error) {
	// Make a copy of the object to avoid mutating the original
	copyObj := reflect.New(reflect.TypeOf(obj).Elem()).Interface()
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, copyObj)
	if err != nil {
		return nil, err
	}

	// Mask fields in the copied object
	MaskFields(copyObj)

	// Marshal the masked object back to JSON
	return json.Marshal(copyObj)
}

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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"sort"
)

type KeyValue struct {
	Key     string      `json:"key,omitempty"`
	Value   int64       `json:"value,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

func SortMapStrIntToKV(data map[string]int) []KeyValue {
	var keyValuePairs []KeyValue

	for k, v := range data {
		keyValuePairs = append(keyValuePairs, KeyValue{k, int64(v), nil})
	}

	keyValuePairs = SortKeyValueArray(keyValuePairs, false)

	return keyValuePairs
}

// sort by value desc by default
func SortKeyValueArray(keyValuePairs []KeyValue, revers bool) []KeyValue {
	sort.Slice(keyValuePairs, func(i, j int) bool {
		if revers {
			return keyValuePairs[i].Value < keyValuePairs[j].Value
		} else {
			return keyValuePairs[i].Value > keyValuePairs[j].Value
		}
	})
	return keyValuePairs
}

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

// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package safemapstr


import (
	"infini.sh/framework/core/util"
	"strings"
	)

const alternativeKey = "value"

// Put This method implements a way to put dotted keys into a MapStr while
// ensuring they don't override each other. For example:
//
//  a := MapStr{}
//  safemapstr.Put(a, "com.docker.swarm.task", "x")
//  safemapstr.Put(a, "com.docker.swarm.task.id", 1)
//  safemapstr.Put(a, "com.docker.swarm.task.name", "foobar")
//
// Will result in `{"com":{"docker":{"swarm":{"task":{"id":1,"name":"foobar","value":"x"}}}}}`
//
// Put detects this scenario and renames the common base key, by appending
// `.value`
func Put(data util.MapStr, key string, value interface{}) error {
	// XXX This implementation mimics `MapStr.Put`, both should be updated to have similar behavior

	d, k := mapFind(data, key, alternativeKey)
	d[k] = value
	return nil
}

// mapFind walk the map based on the given dotted key and returns the final map
// and key to operate on. This function adds intermediate maps, if the key is
// missing from the original map.

// mapFind iterates a MapStr based on the given dotted key, finding the final
// subMap and subKey to operate on.
// If a key is already used, but the used value is no map, an intermediate map will be inserted and
// the old value will be stored using the 'alternativeKey' in a new map.
// If the old value found under key is already an dictionary, subMap will be
// the old value and subKey will be set to alternativeKey.
func mapFind(data util.MapStr, key, alternativeKey string) (subMap util.MapStr, subKey string) {
	// XXX This implementation mimics `common.mapFind`, both should be updated to have similar behavior

	for {
		if oldValue, exists := data[key]; exists {
			if oldMap, ok := tryToMapStr(oldValue); ok {
				return oldMap, alternativeKey
			}
			return data, key
		}

		idx := strings.IndexRune(key, '.')
		if idx < 0 {
			// if old value exists and is a dictionary, return the old dictionary and
			// make sure we store the new value using the 'alternativeKey'
			if oldValue, exists := data[key]; exists {
				if oldMap, ok := tryToMapStr(oldValue); ok {
					return oldMap, alternativeKey
				}
			}

			return data, key
		}

		// Check if first sub-key exists. Create an intermediate map if not.
		k := key[:idx]
		d, exists := data[k]
		if !exists {
			d = util.MapStr{}
			data[k] = d
		}

		// store old value under 'alternativeKey' if the old value is no map.
		// Do not overwrite old value.
		v, ok := tryToMapStr(d)
		if !ok {
			v = util.MapStr{alternativeKey: d}
			data[k] = v
		}

		// advance into sub-map
		key = key[idx+1:]
		data = v
	}
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

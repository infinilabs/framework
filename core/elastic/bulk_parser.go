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

package elastic

import (
	"github.com/buger/jsonparser"
	"github.com/rubyniu105/framework/core/errors"
)

var ActionIndex = "index"
var ActionDelete = "delete"
var ActionCreate = "create"
var ActionUpdate = "update"

var ActionStart = []byte("\"")
var ActionEnd = []byte("\"")

var Actions = []string{"index", "delete", "create", "update"}

func ParseActionMeta(data []byte) (action, index, typeName, id, routing string, err error) {

	match := false
	for _, v := range Actions {
		jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch string(key) {
			case "_index":
				index = string(value)
				break
			case "_type":
				typeName = string(value)
				break
			case "_id":
				id = string(value)
				break
			case "_routing":
				routing = string(value)
				break
			}
			match = true
			return nil
		}, v)
		action = v
		if match {
			return action, index, typeName, id, routing, nil
		}
	}

	return action, index, typeName, id, routing, errors.Errorf("invalid_meta_buffer: %v", string(data))
}

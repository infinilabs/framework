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
	"fmt"
	"src/github.com/buger/jsonparser"
)

var ActionIndex = "index"
var ActionDelete = "delete"
var ActionCreate = "create"
var ActionUpdate = "update"

var ActionStart = []byte("\"")
var ActionEnd = []byte("\"")

var Actions = []string{"index", "delete", "create", "update"}

func ParseActionMeta(data []byte) (action, index, typeName, id, routing string, err error) {
	// Extract the first key of the JSON object to determine the action
	err = jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		action = string(key) // The first key is the action

		// Parse the nested object for metadata fields
		parseErr := jsonparser.ObjectEach(value, func(metaKey []byte, metaValue []byte, dataType jsonparser.ValueType, offset int) error {
			switch string(metaKey) {
			case "_index":
				index = string(metaValue)
			case "_type":
				typeName = string(metaValue)
			case "_id":
				id = string(metaValue)
			case "_routing":
				routing = string(metaValue)
			}
			return nil
		})

		if parseErr != nil {
			err = fmt.Errorf("error parsing metadata: %w", parseErr)
		}

		// Stop further iteration after processing the first key
		return fmt.Errorf("break")
	})

	// If no action was found, or there was a parsing error
	if err != nil && err.Error() != "break" {
		return "", "", "", "", "", fmt.Errorf("invalid_meta_buffer: %v", string(data))
	}

	// Clear the error if "break" was used to stop iteration
	if err != nil && err.Error() == "break" {
		err = nil
	}

	return action, index, typeName, id, routing, err
}

//func ParseActionMeta(data []byte) (action, index, typeName, id, routing string, err error) {
//
//	match := false
//	for _, v := range Actions {
//		jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
//			switch string(key) {
//			case "_index":
//				index = string(value)
//				break
//			case "_type":
//				typeName = string(value)
//				break
//			case "_id":
//				id = string(value)
//				break
//			case "_routing":
//				routing = string(value)
//				break
//			}
//			match = true
//			return nil
//		}, v)
//		action = v
//		if match {
//			return action, index, typeName, id, routing, nil
//		}
//	}
//
//	if action==""{
//		return action, "", "", "", "", errors.Errorf("invalid_meta_buffer: %v", string(data))
//	}
//
//	return action, index, typeName, id, routing, nil
//}

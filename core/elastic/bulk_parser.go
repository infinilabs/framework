/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"github.com/buger/jsonparser"
	"infini.sh/framework/core/errors"
)

var ActionIndex = "index"
var ActionDelete = "delete"
var ActionCreate = "create"
var ActionUpdate = "update"

var ActionStart = []byte("\"")
var ActionEnd = []byte("\"")

var Actions = []string{"index", "delete", "create", "update"}

func ParseActionMeta(data []byte) (action, index, typeName, id,routing string) {

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
			return action, index, typeName, id,routing
		}
	}

	panic(errors.Errorf("invalid meta buffer: %v",string(data)))
}
/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/util"
)

var ActionIndex = "index"
var ActionDelete = "delete"
var ActionCreate = "create"
var ActionUpdate = "update"

var ActionStart = []byte("\"")
var ActionEnd = []byte("\"")

var Actions = []string{"index", "delete", "create", "update"}

func ParseActionMeta(data []byte) (action, index, typeName, id string) {

	match := false
	for _, v := range Actions {
		jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch util.UnsafeBytesToString(key) {
			case "_index":
				index = string(value)
				break
			case "_type":
				typeName = string(value)
				break
			case "_id":
				id = string(value)
				break
			}
			match = true
			return nil
		}, v)
		action = v
		if match {
			return action, index, typeName, id
		}
	}

	log.Warn("fallback to unsafe parse:", util.UnsafeBytesToString(data))

	action = string(util.ExtractFieldFromBytes(&data, ActionStart, ActionEnd, nil))
	index, _ = jsonparser.GetString(data, action, "_index")
	typeName, _ = jsonparser.GetString(data, action, "_type")
	id, _ = jsonparser.GetString(data, action, "_id")

	if index != "" {
		return action, index, typeName, id
	}

	log.Warn("fallback to safety parse:", util.UnsafeBytesToString(data))
	return safetyParseActionMeta(data)
}


//performance is poor
func safetyParseActionMeta(scannedByte []byte) (action, index, typeName, id string) {

	////{ "index" : { "_index" : "test", "_id" : "1" } }
	var meta = BulkActionMetadata{}
	meta.UnmarshalJSON(scannedByte)
	if meta.Index != nil {
		index = meta.Index.Index
		typeName = meta.Index.Type
		id = meta.Index.ID
		action = ActionIndex
	} else if meta.Create != nil {
		index = meta.Create.Index
		typeName = meta.Create.Type
		id = meta.Create.ID
		action = ActionCreate
	} else if meta.Update != nil {
		index = meta.Update.Index
		typeName = meta.Update.Type
		id = meta.Update.ID
		action = ActionUpdate
	} else if meta.Delete != nil {
		index = meta.Delete.Index
		typeName = meta.Delete.Type
		action = ActionDelete
		id = meta.Delete.ID
	}

	return action, index, typeName, id
}


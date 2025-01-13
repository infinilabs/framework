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

package elastic

import (
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/util"
	"math"
	"testing"
)

func TestBulkWalkLines(t *testing.T) {
	bulkRequests := "{ \"index\" : { \"_index\" : \"medcl-test\",\"_type\" : \"doc\", \"_id\" : \"id1\" } }\n{ \"id\" : \"123\",\"field1\" : \"user2\",\"ip\" : \"123\" }\n"
	bulkRequests += "{ \"index\" : { \"_index\" : \"medcl-test\",\"_type\" : \"doc\", \"_id\" : \"id2\" } }\n{ \"id\" : \"345\",\"field1\" : \"user1\",\"ip\" : \"456\" }\n"
	bulkRequests += "{ \"index\" : { \"_index\" : \"test\", \"_id\" : \"1\" } }\n { \"field1\" : \"value1\" }\n" +
		"{ \"delete\" : { \"_index\" : \"test\", \"_id\" : \"2\" } }\n" +
		"{ \"create\" : { \"_index\" : \"test\", \"_id\" : \"3\" } }\n{ \"field1\" : \"value3\" }\n" +
		"{ \"update\" : {\"_id\" : \"1\", \"_index\" : \"test\"} }\n{ \"doc\" : {\"field2\" : \"value2\"} }\n"

	WalkBulkRequests("", []byte(bulkRequests), func(eachLine []byte) (skipNextLine bool) {
		//fmt.Println(string(eachLine))
		return false
	}, func(metaBytes []byte, actionStr, index, typeName, id, routing string, offset int) (err error) {
		fmt.Println(string(metaBytes), "offset:", offset, ",id:", id)
		return nil
	}, func(payloadBytes []byte, actionStr, index, typeName, id, routing string) {
		fmt.Println(string(payloadBytes))
	}, nil)
}

func TestBulkStats(t *testing.T) {
	statsRet := make(map[int]int)
	statsRet[1] = statsRet[1] + 1

}

func TestBulkWalkLinesSimdJson(t *testing.T) {
	bulkRequests := "{ \"index\" : { \"_index\" : \"medcl-test\",\"_type\" : \"doc\", \"_id\" : \"id1\" } }\n{ \"id\" : \"123\",\"field1\" : \"user2\",\"ip\" : \"123\" }\n"
	bulkRequests += "{ \"index\" : { \"_index\" : \"medcl-test\",\"_type\" : \"doc\", \"_id\" : \"id2\" } }\n{ \"id\" : \"345\",\"field1\" : \"user1\",\"ip\" : \"456\" }\n"
	bulkRequests += "{ \"index\" : { \"_index\" : \"test\", \"_id\" : \"1\" } }\n { \"field1\" : \"value1\" }\n" +
		"{ \"delete\" : { \"_index\" : \"test\", \"_id\" : \"2\" } }\n" +
		"{ \"create\" : { \"_index\" : \"test\", \"_id\" : \"3\" } }\n{ \"field1\" : \"value3\" }\n" +
		"{ \"update\" : {\"_id\" : \"1\", \"_index\" : \"test\"} }\n{ \"doc\" : {\"field2\" : \"value2\"} }\n"

	response := "{\n  \"took\": 922,\n  \"errors\": true,\n  \"items\": [\n    " +
		"{\n      \"index\": {\n        \"_index\": \"medcl-test\",\n        \"_type\": \"doc\",\n        \"_id\": \"id1\",\n        \"status\": 400,\n       " +
		" \"error\": {\n          \"type\": \"illegal_argument_exception\",\n          \"reason\": \"Rejecting mapping update to [medcl-test] as the final mapping would have more than 1 type: [_doc, doc]\"\n        }\n      }\n    }," +
		"\n    {\n      \"index\": {\n        \"_index\": \"medcl-test\",\n        \"_type\": \"doc\",\n        \"_id\": \"id2\",\n        \"status\": 400,\n      " +
		"  \"error\": {\n          \"type\": \"illegal_argument_exception\",\n          \"reason\": \"Rejecting mapping update to [medcl-test] as the final mapping would have more than 1 type: [_doc, doc]\"\n        }\n      }\n    }," +
		"\n    {\n      \"index\": {\n        \"_index\": \"test\",\n        \"_type\": \"_doc\",\n        \"_id\": \"1\",\n        \"_version\": 1,\n        \"result\": \"created\",\n        \"_shards\": {\n          \"total\": 1,\n          \"successful\": 1,\n          \"failed\": 0\n        },\n        \"_seq_no\": 40107052,\n        \"_primary_term\": 13,\n        \"status\": 201\n      }\n    },\n " +
		"   {\n      \"delete\": {\n        \"_index\": \"test\",\n        \"_type\": \"_doc\",\n        \"_id\": \"2\",\n        \"_version\": 1,\n        \"result\": \"not_found\",\n        \"_shards\": {\n          \"total\": 1,\n          \"successful\": 1,\n          \"failed\": 0\n        },\n        \"_seq_no\": 41257489,\n        \"_primary_term\": 15,\n        \"status\": 404\n      }\n    },\n   " +
		" {\n      \"create\": {\n        \"_index\": \"test\",\n        \"_type\": \"_doc\",\n        \"_id\": \"3\",\n        \"_version\": 1,\n        \"result\": \"created\",\n        \"_shards\": {\n          \"total\": 1,\n          \"successful\": 1,\n          \"failed\": 0\n        },\n        \"_seq_no\": 41257490,\n        \"_primary_term\": 15,\n        \"status\": 201\n      }\n    },\n   " +
		" {\n      \"update\": {\n        \"_index\": \"test\",\n        \"_type\": \"_doc\",\n        \"_id\": \"1\",\n        \"_version\": 2,\n        \"result\": \"updated\",\n        \"_shards\": {\n          \"total\": 1,\n          \"successful\": 1,\n          \"failed\": 0\n        },\n        \"_seq_no\": 40107053,\n        \"_primary_term\": 13,\n        \"status\": 200\n      }\n    }\n  ]\n}"
		//fmt.Println(bulkRequests)

	items, _, _, err := jsonparser.Get(util.UnsafeStringToBytes(response), "items")
	if err != nil {
		panic(err)
	}

	jsonparser.ArrayEach(items, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		item, _, _, err := jsonparser.Get(value, "index")
		if err != nil {
			item, _, _, err = jsonparser.Get(value, "delete")
			if err != nil {
				item, _, _, err = jsonparser.Get(value, "create")
				if err != nil {
					item, _, _, err = jsonparser.Get(value, "update")
				}
			}
		}
		if err == nil {
			code, err := jsonparser.GetInt(item, "status")
			if err != nil {
				panic(err)
			}
			fmt.Println(code)
		}
	})

	//// Parse JSON:
	//pj, err := simdjson.Parse([]byte(`{"Image":{"URL":"http://example.com/example.gif"}}`), nil)
	//if err != nil {
	//	log.Fatal(err)
	//}

	//// Iterate each top level element.
	//_ = pj.ForEach(func(i simdjson.Iter) error {
	//	fmt.Println("Got iterator for type:", i.Type())
	//	element, err := i.FindElement(nil, "Image", "URL")
	//	if err == nil {
	//		value, _ := element.Iter.StringCvt()
	//		fmt.Println("Found element:", element.Name, "Type:", element.Type, "Value:", value)
	//	}
	//	return nil
	//})

}

func TestMod(t *testing.T) {
	a := 100
	b := 3
	fmt.Println(math.Mod(float64(a), float64(b)))
	fmt.Println(a % b)
}

func TestRetryRules_Retryable(t *testing.T) {
	rules := RetryRules{Retry429: false, Default: true, Denied: RetryRule{Keyword: []string{"\"message\":\"timeout\""}}}
	ok := rules.Retryable(429, "{\"error\":true,\"message\":\"timeout\"}")
	fmt.Println(ok)

	ok = rules.Retryable(429, "{\"error\":{\"root_cause\":[{\"type\":\"es_rejected_execution_exception\",\"reason\":\"rejected execution of coordinating operation [coordinating_and_primary_bytes=3325454943, replica_bytes=0, all_bytes=3325454943, coordinating_operation_bytes=9054099, max_coordinating_and_primary_bytes=3328599654]\"}],\"type\":\"es_rejected_execution_exception\",\"reason\":\"rejected execution of coordinating operation [coordinating_and_primary_bytes=3325454943, replica_bytes=0, all_bytes=3325454943, coordinating_operation_bytes=9054099, max_coordinating_and_primary_bytes=3328599654]\"},\"status\":429}")
	fmt.Println(ok)
}

func TestParseActionMeta3(t *testing.T) {

	data := []byte("{\"index\":{\"_index\":\"medcl1\",\"_type\":\"_doc\",\"_id\":\"GZq-bnYBC53QmW9Kk2ve\"}}")
	newData, err := UpdateBulkMetadata("index", data, "newIndex", "newType", "newId")
	fmt.Println(err, string(newData))
	assert.Equal(t, string(newData), "{\"index\":{\"_index\":\"newIndex\",\"_type\":\"newType\",\"_id\":\"newId\"}}")

	data = []byte("{\"index\":{\"_index\":\"medcl1\",\"_id\":\"GZq-bnYBC53QmW9Kk2ve\"}}")
	newData, err = UpdateBulkMetadata("index", data, "newIndex", "newType", "newId")
	fmt.Println(err, string(newData))
	assert.Equal(t, string(newData), "{\"index\":{\"_index\":\"newIndex\",\"_id\":\"newId\",\"_type\":\"newType\"}}")

	data = []byte("{\"index\":{\"_index\":\"medcl1\",\"_type\":\"doc1\"}}")
	newData, err = UpdateBulkMetadata("index", data, "newIndex", "newType", "newId")
	fmt.Println(err, string(newData))
	assert.Equal(t, string(newData), "{\"index\":{\"_index\":\"newIndex\",\"_type\":\"newType\",\"_id\":\"newId\"}}")

	data = []byte("{\"index\":{\"_index\":\"medcl1\",\"_type\":\"doc1\"}}")
	newData, err = UpdateBulkMetadata("index", data, "", "", "newId")
	fmt.Println(err, string(newData))
	assert.Equal(t, string(newData), "{\"index\":{\"_index\":\"medcl1\",\"_type\":\"doc1\",\"_id\":\"newId\"}}")
}

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elastic

import (
	"bytes"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/util"
	"strings"
	"unicode"
)

func getIndexName(any interface{}) string {
	return util.GetTypeName(any, true)
}

// getIndexID extract the field value and will be used as document ID
//elastic_meta:"_id"
func getIndexID(any interface{}) string {
	return util.GetFieldValueByTagName(any, "elastic_meta", "_id")
}

func getIndexMapping(any interface{}) []util.Annotation {
	return util.GetTagsByTagName(any, "elastic_mapping")
}

func parseAnnotation(mapping []util.Annotation) string {

	jsonFormat := ` properties:{ %s }`

	str := bytes.Buffer{}
	hasField := false
	for i := 0; i < len(mapping); i++ {
		field := mapping[i]

		tag := field.Tag
		//nested tag
		if len(field.Annotation) > 0 {
			tag = strings.Replace(tag, "}", fmt.Sprintf(", %s  }", parseAnnotation(field.Annotation)), -1)
		}
		if util.TrimSpaces(tag) != "" {
			if hasField {
				str.WriteString(",")
			}
			str.WriteString(tag)

			hasField = true
		}

	}
	json := fmt.Sprintf(jsonFormat, str.String())
	return json
}

//elastic_mapping:"content: { type: binary, doc_values:false }"
func (handler ElasticORM) RegisterSchema(t interface{}) error {

	indexName := getIndexName(t)

	log.Trace("indexName: ", indexName)
	exist, err := handler.Client.IndexExists(indexName)
	if err != nil {
		panic(err)
	}
	if !exist {
		err = handler.Client.CreateIndex(indexName, nil)
		if err != nil {
			panic(err)
		}

		jsonFormat := `{ %s }`
		mapping := getIndexMapping(t)

		js := parseAnnotation(mapping)

		json := fmt.Sprintf(jsonFormat, quoteJson(js))

		log.Trace("mapping: ", json)

		_, err = handler.Client.UpdateMapping(indexName, []byte(json))
		if err != nil {
			panic(err)
		}

		log.Debugf("schema %v successful initialized", indexName)

	}

	return nil
}

var quote int32 = 34     //"
var colon int32 = 58     //:
var comma int32 = 44     //,
var bracket1 int32 = 93  //]
var bracket2 int32 = 125 //}
func quoteJson(str string) string {

	var buffer bytes.Buffer
	white := false
	quoted := false

	for _, c := range str {

		if c != quote && (colon == c || comma == c || bracket2 == c || bracket1 == c || unicode.IsSpace(c)) && quoted {
			buffer.WriteString("\"")
			quoted = false
		}

		if c != quote && unicode.IsLetter(c) && !quoted {
			buffer.WriteString("\"")
			quoted = true
		}

		if unicode.IsSpace(c) {
			quoted = false
			if !white {
				buffer.WriteString(" ")
			}
			white = true
		} else {
			buffer.WriteRune(c)
			white = false
		}
	}
	return util.TrimSpaces(buffer.String())
}

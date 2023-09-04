/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"bytes"
	"fmt"
	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"strings"
	"sync"
	"unicode"
)

var indexNames =sync.Map{}

func getIndexName(any interface{}) string {
	pkg,t:=util.GetTypeAndPackageName(any, true)
	key:=fmt.Sprintf("%s-%s",pkg,t)
	//prefer to use registered index name
	v,ok:=indexNames.Load(key)
	if ok{
		return v.(string)
	}
	return t
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
func (handler *ElasticORM) RegisterSchema(t interface{}) error {
	return handler.RegisterSchemaWithIndexName(t,"")
}

func initIndexName(t interface{},indexName string)string  {
	pkg,ojbType:=util.GetTypeAndPackageName(t, true)
	key:=fmt.Sprintf("%s-%s",pkg,ojbType)
	if indexName!=""{
		v,ok:=indexNames.Load(indexName)
		if ok{
			if v==key{
				log.Warnf("duplicated schema %v, %s",indexName,key)
				return indexName
			}
			panic(errors.Errorf("index name [%s][%s] already registered!",indexName,key))
		}
	}else{
		indexName=ojbType
	}

	indexNames.Store(key,indexName)
	indexNames.Store(indexName,key)
	return indexName
}

func (handler *ElasticORM) RegisterSchemaWithIndexName(t interface{},indexName string) error {

	if !handler.Config.Enabled{
		log.Debugf("elastic ORM is disabled, skip schema registration")
		return nil
	}
	
	initIndexName(t,indexName)

	indexName=orm.GetIndexName(t)

	log.Trace("indexName: ", indexName)

	exist, err := handler.Client.IndexExists(indexName)
	if err != nil {
		return err
	}
	if !exist {
		err = handler.Client.CreateIndex(indexName, nil)
		if err != nil {
			return err
		}

		jsonFormat := `{ %s }`
		mapping := getIndexMapping(t)

		js := parseAnnotation(mapping)
		json := fmt.Sprintf(jsonFormat, quoteJson(js))

		log.Trace(indexName,", mapping: ", json)

		data, err := handler.Client.UpdateMapping(indexName, "", []byte(json))
		if err != nil {
			return err
		}
		x,_,_,_:= jsonparser.Get(data,"error")
		if x!=nil{
			log.Errorf("error on update mapping: %v, %v",indexName,string(x))
		}else{
			log.Debugf("schema %v successful initialized", indexName)
		}
	}
	return err
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

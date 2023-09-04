/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/util"
	"testing"
	"time"
)

type MyHostConfig struct {
	Created time.Time `json:"created,omitempty" elastic_mapping:"created: { type: date }"`
	Updated time.Time `json:"updated,omitempty" elastic_mapping:"updated: { type: date }"`
}

type MyHost struct {
	Host        string         `json:"host,omitempty" elastic_meta:"_id" elastic_mapping:"host: { type: text, fields: { keyword: { type: keyword, ignore_above: 256 } } }"`
	Favicon     string         `json:"favicon,omitempty" elastic_mapping:"favicon: { type: keyword ,copy_to : [all_field_values]}"`
	Enabled     bool           `json:"enabled" elastic_mapping:"enabled: { type: boolean }"`
	HostConfig  *MyHostConfig  `json:"host_configs,omitempty" elastic_mapping:"host_config:{type:object}"`
	HostConfigs []MyHostConfig `json:"host_configs,omitempty" elastic_mapping:"host_configs:{type:object}"`
}

var host = MyHost{
	Host: "www.google.com", Favicon: "http://1.com/favicon.ico", Enabled: false,
	HostConfig: &MyHostConfig{Created: time.Now()},
	HostConfigs: []MyHostConfig{
		{Created: time.Now()},
	},
}

func TestExtractIndexID(t *testing.T) {
	id := getIndexID(host)
	assert.Equal(t, "www.google.com", id)
}

func TestCheckPoint(t *testing.T) {
	mapping1 := getIndexMapping(host)

	mappingStr := util.TrimSpaces(parseAnnotation(mapping1))
	fmt.Println(mappingStr)

	assert.Equal(t, "properties:{ host: { type: text, fields: { keyword: { type: keyword, ignore_above: 256 } } },favicon: { type: keyword ,copy_to : [all_field_values]},enabled: { type: boolean },host_config:{type:object,  properties:{ created: { type: date },updated: { type: date } }  },host_configs:{type:object,  properties:{ created: { type: date },updated: { type: date } }  } }", mappingStr)

	// check with point
	mapping2 := getIndexMapping(&host)

	assert.Equal(t, mapping1, mapping2)
}

func TestExtractIndexMappingMetadata(t *testing.T) {
	mapping := getIndexMapping(host)
	fmt.Println(util.ToJson(mapping, true))
}

func TestSchemaRegister(t *testing.T) {
	o:=MyHost{}
	indexName:=initIndexName(o,"")
	fmt.Println(indexName)
	assert.Equal(t,"myhost",indexName)

	indexName=getIndexName(o)
	fmt.Println(indexName)
	assert.Equal(t,"myhost",indexName)

	indexName=initIndexName(o,"myindex")
	fmt.Println(indexName)
	assert.Equal(t,"myindex",indexName)

	indexName=getIndexName(o)
	fmt.Println(indexName)
	assert.Equal(t,"myindex",indexName)


	//indexName=initIndexName(MyHostConfig{},"myindex")
	//fmt.Println(indexName)

}
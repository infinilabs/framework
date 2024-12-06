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
	"github.com/rubyniu105/framework/core/util"
	"github.com/stretchr/testify/assert"
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
	o := MyHost{}
	indexName := initIndexName(o, "")
	fmt.Println(indexName)
	assert.Equal(t, "myhost", indexName)

	indexName = getIndexName(o)
	fmt.Println(indexName)
	assert.Equal(t, "myhost", indexName)

	indexName = initIndexName(o, "myindex")
	fmt.Println(indexName)
	assert.Equal(t, "myindex", indexName)

	indexName = getIndexName(o)
	fmt.Println(indexName)
	assert.Equal(t, "myindex", indexName)

	//indexName=initIndexName(MyHostConfig{},"myindex")
	//fmt.Println(indexName)

}

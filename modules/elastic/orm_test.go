/*
Copyright Medcl (m AT medcl.net)

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
	"fmt"
	"infini.sh/framework/core/util"
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

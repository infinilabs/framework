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

package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/elastic"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/util"
)

type ESAPIV7 struct {
	ESAPIV6
}

func (c *ESAPIV7) Init() {
	c.initTemplate(c.Config.IndexPrefix)
}

func (c *ESAPIV7) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"index_patterns": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.max_result_window":10000000
  },
  "mappings": {
      "dynamic_templates": [
        {
          "strings": {
            "match_mapping_type": "string",
            "mapping": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        }
      ]
  }
}
`
	return fmt.Sprintf(template, indexPrefix, 1)
}

func (c *ESAPIV7) initTemplate(indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	templateName := "infinitbyte"

	if c.Config.TemplateName != "" {
		templateName = c.Config.TemplateName
	}

	exist, err := c.TemplateExists(templateName)
	if err != nil {
		panic(err)
	}
	if !exist {
		template := c.getDefaultTemplate(indexPrefix)
		log.Trace("template: ", template)
		res, err := c.PutTemplate(templateName, []byte(template))
		if err != nil {
			panic(err)
		}
		if global.Env().IsDebug {
			log.Trace("put template response, %v", string(res))
		}
	}
	log.Debugf("elasticsearch template successful initialized")

}

const TypeName7 = "_doc"

// Delete used to delete document by id
func (c *ESAPIV7) Delete(indexName, id string) (*elastic.DeleteResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := c.Config.Endpoint + "/" + indexName + "/" + TypeName7 + "/" + id

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	esResp := &elastic.DeleteResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.DeleteResponse{}, err
	}
	if esResp.Result != "deleted" {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

// Get fetch document by id
func (c *ESAPIV7) Get(indexName, id string) (*elastic.GetResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := c.Config.Endpoint + "/" + indexName + "/" + TypeName7 + "/" + id

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	esResp := &elastic.GetResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.GetResponse{}, err
	}
	if !esResp.Found {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

// IndexDoc index a document into elasticsearch
func (c *ESAPIV7) Index(indexName string, id interface{}, data interface{}) (*elastic.InsertResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := fmt.Sprintf("%s/%s/%s/%s", c.Config.Endpoint, indexName, TypeName7, id)

	js, err := json.Marshal(data)

	if global.Env().IsDebug {
		log.Debug("indexing doc: ", url, ",", string(js))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.Request(util.Verb_POST, url, js)
	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	if global.Env().IsDebug {
		log.Trace("indexing response: ", string(resp.Body))
	}

	esResp := &elastic.InsertResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.InsertResponse{}, err
	}
	if !(esResp.Result == "created" || esResp.Result == "updated") {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

func (c *ESAPIV7) UpdateMapping(indexName string, mappings []byte) ([]byte, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	if global.Env().IsDebug {
		log.Debug("update mapping, ", indexName, ", ", string(mappings))
	}
	url := fmt.Sprintf("%s/%s/_mapping", c.Config.Endpoint, indexName)
	resp, err := c.Request(util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	return resp.Body, err
}

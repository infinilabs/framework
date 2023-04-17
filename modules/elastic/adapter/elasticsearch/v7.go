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

package elasticsearch

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net/http"
)

type ESAPIV7 struct {
	ESAPIV6_6
}

func (c *ESAPIV7) InitDefaultTemplate(templateName,indexPrefix string) {
	c.initTemplate(templateName,indexPrefix)
}

func (c *ESAPIV7) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"index_patterns": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.mapping.total_fields.limit": 20000,
    "index.max_result_window":10000000,
	 "analysis": {
		  "analyzer": {
			"suggest_text_search": {
			  "tokenizer": "classic",
			  "filter": [
				"word_delimiter"
			  ]
			}
		  }
		}
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

func (c *ESAPIV7) initTemplate(templateName,indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	if templateName==""{
		templateName = global.Env().GetAppLowercaseName()
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
func (c *ESAPIV7) Delete(indexName,docType, id string, refresh ...string) (*elastic.DeleteResponse, error) {
	indexName=util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/" + TypeName7 + "/" + id

	if len(refresh)>0 {
		url = url + "?refresh=" + refresh[0]
	}

	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	esResp := &elastic.DeleteResponse{}
	esResp.StatusCode=resp.StatusCode
	esResp.RawResult=resp
	err = json.Unmarshal(resp.Body, esResp)

	if err != nil {
		return &elastic.DeleteResponse{}, err
	}
	if esResp.Result != "deleted" && esResp.Result!="not_found" {
		return nil, errors.New(string(resp.Body))
	}
	return esResp, nil
}

// Get fetch document by id
func (c *ESAPIV7) Get(indexName, docType, id string) (*elastic.GetResponse, error) {
	if docType==""{
		docType=TypeName7
	}
	indexName=util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/" + docType + "/" + id

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode=resp.StatusCode
	esResp.RawResult=resp

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

// IndexDoc index a document into elasticsearch
func (c *ESAPIV7) Index(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType==""{
		docType=TypeName7
	}
	indexName=util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/%s/%s", c.GetEndpoint(), indexName, docType, id)

	if id==""{
		url = fmt.Sprintf("%s/%s/%s/", c.GetEndpoint(), indexName, docType)
	}
	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s", url, refresh)
	}
	var (
		js []byte
		err error
	)
	if dataBytes, ok := data.([]byte); ok {
		js = dataBytes
	}else{
		js, err = json.Marshal(data)
	}

	if global.Env().IsDebug {
		log.Debug("indexing doc: ", url, ",", string(js))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.Request(nil, util.Verb_POST, url, js)
	if err != nil {
		return nil, err
	}

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
	indexName=util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_mapping", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}

	return resp.Body, err
}

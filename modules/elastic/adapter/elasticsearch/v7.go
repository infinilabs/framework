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
	"bytes"
	"fmt"
	"infini.sh/framework/core/errors"
	"net/http"
	"net/url"

	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV7 struct {
	ESAPIV6_6
}

func (c *ESAPIV7) InitDefaultTemplate(templateName, indexPrefix string) {
	c.initTemplate(templateName, indexPrefix)
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

func (c *ESAPIV7) initTemplate(templateName, indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	if templateName == "" {
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

func (c *ESAPIV7) CreateIndex(indexName string, settings map[string]interface{}) (err error) {
	body := bytes.Buffer{}
	if len(settings) > 0 {
		enc := json.NewEncoder(&body)
		enc.Encode(settings)
	}

	var docType string

	if mappings, ok := settings["mappings"]; ok {
		if mappings, ok := mappings.(map[string]interface{}); ok && len(mappings) == 1 {
			for key, _ := range mappings {
				if key != "properties" {
					docType = key
				}
			}
		}
	}

	if global.Env().IsDebug {
		log.Trace("start create index: ", indexName, ",", settings, ",", string(body.Bytes()))
	}
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), indexName)
	if docType != "" {
		url = fmt.Sprintf("%s/%s?include_type_name=true", c.GetEndpoint(), indexName)
	}

	result, err := c.Request(nil, util.Verb_PUT, url, body.Bytes())

	if err != nil {
		return err
	}
	if result.StatusCode != http.StatusOK {
		return fmt.Errorf("code:%v,response:%v", result.StatusCode, string(result.Body))
	}

	return nil
}

const TypeName7 = "_doc"

// Delete used to delete document by id
func (c *ESAPIV7) Delete(indexName, docType, id string, refresh ...string) (*elastic.DeleteResponse, error) {
	indexName = util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/" + TypeName7 + "/" + id

	if len(refresh) > 0 {
		url = url + "?refresh=" + refresh[0]
	}

	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	esResp := &elastic.DeleteResponse{}
	esResp.StatusCode = resp.StatusCode
	esResp.RawResult = resp
	err = json.Unmarshal(resp.Body, esResp)

	if err != nil {
		return &elastic.DeleteResponse{}, err
	}
	if esResp.Result != "deleted" && esResp.Result != "not_found" {
		return nil, errors.New(string(resp.Body))
	}
	return esResp, nil
}

// Get fetch document by id
func (c *ESAPIV7) Get(indexName, docType, id string) (*elastic.GetResponse, error) {
	if docType == "" {
		docType = TypeName7
	}
	indexName = util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/" + docType + "/" + id

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode = resp.StatusCode
	esResp.RawResult = resp

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

func (c *ESAPIV7) Update(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName7
	}

	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_update/%s/", c.GetEndpoint(), indexName, id)

	if id == "" {
		panic(errors.New("id is required"))
	}

	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s&retry_on_conflict=3", url, refresh)
	} else {
		url = fmt.Sprintf("%s?retry_on_conflict=3", url)
	}

	js := util.MapStr{}
	js["doc"] = data
	js["detect_noop"] = true
	js["doc_as_upsert"] = true

	resp, err := c.Request(nil, util.Verb_POST, url, util.MustToJSONBytes(js))
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
	if !(esResp.Result == "created" || esResp.Result == "updated" || esResp.Shards.Successful > 0) {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

// IndexDoc index a document into elasticsearch
func (c *ESAPIV7) Index(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName7
	}
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/%s/%s", c.GetEndpoint(), indexName, docType, id)

	if id == "" {
		url = fmt.Sprintf("%s/%s/%s/", c.GetEndpoint(), indexName, docType)
	}
	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s", url, refresh)
	}
	var (
		js  []byte
		err error
	)
	if dataBytes, ok := data.([]byte); ok {
		js = dataBytes
	} else {
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

func (c *ESAPIV7) Create(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName7
	}
	indexName = util.UrlEncode(indexName)

	u, _ := url.Parse(fmt.Sprintf("%s/%s/%s", c.GetEndpoint(), indexName, docType))

	if id != "" {
		u.Path = fmt.Sprintf("%s/%s", u.Path, id)
		q := u.Query()
		q.Set("op_type", "create")
		u.RawQuery = q.Encode()
	}

	if refresh != "" {
		q := u.Query()
		q.Set("refresh", "true")
		u.RawQuery = q.Encode()
	}

	url := u.String()

	var (
		js  []byte
		err error
	)
	if dataBytes, ok := data.([]byte); ok {
		js = dataBytes
	} else {
		js, err = json.Marshal(data)
	}

	if global.Env().IsDebug {
		log.Debug("creating doc: ", url, ",", string(js))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.Request(nil, util.Verb_POST, url, js)
	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("creating response: ", string(resp.Body))
	}

	esResp := &elastic.InsertResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.InsertResponse{}, err
	}
	if !(esResp.Result == "created") {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

func (c *ESAPIV7) UpdateMapping(indexName string, docType string, mappings []byte) ([]byte, error) {
	indexName = util.UrlEncode(indexName)

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

func (c *ESAPIV7) ScriptExists(scriptName string) (bool, error) {
	if scriptName == "" {
		return false, errors.New("invalid script name")
	}

	url := fmt.Sprintf("/_scripts/%s", scriptName)
	url = c.GetEndpoint() + url
	resp, err := c.Request(nil, util.Verb_POST, url, nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf(string(resp.Body))
	}
	return true, nil
}

func (c *ESAPIV7) PutScript(scriptName string, script []byte) ([]byte, error) {
	if scriptName == "" {
		return nil, errors.New("invalid script name")
	}

	url := fmt.Sprintf("/_scripts/%s", scriptName)
	url = c.GetEndpoint() + url
	resp, err := c.Request(nil, util.Verb_POST, url, script)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}

func (c *ESAPIV7) SearchByTemplate(indexName, scriptName string, params map[string]interface{}) (*elastic.SearchResponse, error) {

	if indexName == "" {
		return nil, errors.New("invalid index name")
	}
	if scriptName == "" {
		return nil, errors.New("invalid script name")
	}

	url := fmt.Sprintf("/%s/_search/template", indexName)
	url = c.GetEndpoint() + url
	body := util.MapStr{
		"id":     scriptName,
		"params": params,
	}

	resp, err := c.Request(nil, util.Verb_GET, url, util.MustToJSONBytes(body))
	esResp := &elastic.SearchResponse{}

	if resp != nil {
		esResp.StatusCode = resp.StatusCode
		esResp.RawResult = resp
		esResp.ErrorObject = err
		internalError, _ := parseInternalError(resp)
		if internalError != nil {
			esResp.InternalError = *internalError
		}

		if esResp.Error != nil {
			return esResp, errors.Error(esResp.Error.RootCause)
		}
	}

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

func parseInternalError(resp *util.Result) (*elastic.InternalError, error) {
	//handle error
	if len(resp.Body) > 0 && (resp.StatusCode < 200 || resp.StatusCode >= 400) {
		internalError := elastic.InternalError{}
		err := util.FromJSONBytes(resp.Body, &internalError)
		if err != nil {
			return nil, err
		}
		return &internalError, nil
	}
	return nil, nil
}

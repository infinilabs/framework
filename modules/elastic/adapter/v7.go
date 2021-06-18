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
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
)

type ESAPIV7 struct {
	ESAPIV6
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
func (c *ESAPIV7) Delete(indexName,docType, id string) (*elastic.DeleteResponse, error) {
	url := c.Config.Endpoint + "/" + indexName + "/" + TypeName7 + "/" + id

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	esResp := &elastic.DeleteResponse{}
	esResp.StatusCode=resp.StatusCode
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

	url := c.Config.Endpoint + "/" + indexName + "/" + docType + "/" + id

	resp, err := c.Request(util.Verb_GET, url, nil)
	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode=resp.StatusCode
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}
	if !esResp.Found {
		return esResp, errors.New(string(resp.Body))
	}

	return esResp, nil
}

// IndexDoc index a document into elasticsearch
func (c *ESAPIV7) Index(indexName, docType string, id interface{}, data interface{}) (*elastic.InsertResponse, error) {

	if docType==""{
		docType=TypeName7
	}

	url := fmt.Sprintf("%s/%s/%s/%s", c.Config.Endpoint, indexName, docType, id)

	if id==""{
		url = fmt.Sprintf("%s/%s/%s/", c.Config.Endpoint, indexName, docType)
	}

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
	url := fmt.Sprintf("%s/%s/_mapping", c.Config.Endpoint, indexName)
	resp, err := c.Request(util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	return resp.Body, err
}

func (c *ESAPIV7) NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, sourceFields string,sortField,sortType string) (scroll interface{}, err error) {
	url := fmt.Sprintf("%s/%s/_search?scroll=%s&size=%d", c.Config.Endpoint, indexNames, scrollTime, docBufferCount)
	var jsonBody []byte
	if len(query) > 0 || maxSlicedCount > 0 || len(sourceFields) > 0||true {
		queryBody := map[string]interface{}{}

		if len(sourceFields) > 0 {
			if !strings.Contains(sourceFields, ",") {
				log.Error("The source fields shoud be seraprated by ,")
				return nil, errors.New("")
			} else {
				queryBody["_source"] = strings.Split(sourceFields, ",")
			}
		}

		if len(sortField) > 0 {
			if len(sortType)==0{
				sortType="asc"
			}
			sort:= []map[string]interface{}{}
			sort=append(sort,util.MapStr{
				sortField:util.MapStr{
					"order":sortType,
				},
			})
			queryBody["sort"] =sort
		}

		if len(query) > 0 {
			queryBody["query"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"].(map[string]interface{})["query"] = query
		}

		if maxSlicedCount > 1 {
			log.Tracef("sliced scroll, %d of %d", slicedId, maxSlicedCount)
			queryBody["slice"] = map[string]interface{}{}
			queryBody["slice"].(map[string]interface{})["id"] = slicedId
			queryBody["slice"].(map[string]interface{})["max"] = maxSlicedCount
		}

		jsonArray, err := json.Marshal(queryBody)
		if err != nil {
			log.Error(err)

		} else {
			jsonBody = jsonArray
		}
	}

	if jsonBody == nil {
		panic("scroll request is nil")
	}

	resp, err := c.Request(util.Verb_GET, url, jsonBody)

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	if err != nil {
		log.Error(err)
		return nil, err
	}

	scroll = &elastic.ScrollResponseV7{}
	err = json.Unmarshal(resp.Body, scroll)
	if err != nil {
		log.Error(string(resp.Body))
		log.Error(err)
		return nil, err
	}

	return scroll, err
}

func BasicAuth(req *fasthttp.Request, user, pass string) {
	msg := fmt.Sprintf("%s:%s", user, pass)
	encoded := base64.StdEncoding.EncodeToString([]byte(msg))
	req.Header.Add("Authorization", "Basic "+encoded)
}

func (c *ESAPIV7) NextScroll(scrollTime string, scrollId string) (interface{}, error) {
	id := bytes.NewBufferString(scrollId)
	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", c.Config.Endpoint, scrollTime, id)

	client := &fasthttp.Client{
		MaxConnsPerHost: 60000,
		TLSConfig:       &tls.Config{InsecureSkipVerify: true},
	}

	req := fasthttp.AcquireRequest()
	req.Reset()
	req.ResetBody()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	if c.Config.BasicAuth!=nil{
		BasicAuth(req, c.Config.BasicAuth.Username, c.Config.BasicAuth.Password)
	}

	req.SetRequestURI(url)

	err := client.Do(req, res)
	if err != nil {
		panic(err)
	}

	scroll := &elastic.ScrollResponseV7{}
	err = json.Unmarshal(res.Body(), &scroll)
	if err != nil {
		panic(err)
		return nil, err
	}
	if err != nil {
		//log.Error(body)
		log.Error(err)
		return nil, err
	}

	return scroll, nil
}

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
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"regexp"
	"strings"
)

type ESAPIV0 struct {
	Version string
	Config  elastic.ElasticsearchConfig
}

func (c ESAPIV0) GetMajorVersion() int {
	vs := strings.Split(c.Version, ".")
	n, err := util.ToInt(vs[0])
	if err != nil {
		panic(err)
	}
	return n
}

const TypeName6 = "doc"

func (c *ESAPIV0) Request(method, url string, body []byte) (result *util.Result, err error) {

	if global.Env().IsDebug {
		log.Trace(method, ",", url, ",", string(body))
	}

	var req *util.Request

	switch method {
	case util.Verb_GET:
		req = util.NewGetRequest(url, body)
		break
	case util.Verb_PUT:
		req = util.NewPutRequest(url, body)
		break
	case util.Verb_POST:
		req = util.NewPostRequest(url, body)
		break
	case util.Verb_DELETE:
		req = util.NewDeleteRequest(url, body)
		break
	}

	req.SetContentType(util.ContentTypeJson)

	if c.Config.BasicAuth != nil {
		req.SetBasicAuth(c.Config.BasicAuth.Username, c.Config.BasicAuth.Password)
	}

	if c.Config.HttpProxy != "" {
		req.SetProxy(c.Config.HttpProxy)
	}

	resp, err := util.ExecuteRequest(req)

	if err != nil {
		panic(err)
		return nil, err
	}

	responseHandle(resp)

	return resp, err
}

func (c *ESAPIV0) Init() {
	c.initTemplate(c.Config.IndexPrefix)
}

func (c *ESAPIV0) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"template": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.max_result_window":10000000
  },
  "mappings": {
    "%s": {
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
}
`
	return fmt.Sprintf(template, indexPrefix, 1, TypeName6)
}

func (c *ESAPIV0) initTemplate(indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}

	templateName := global.Env().GetAppLowercaseName()

	if c.Config.TemplateName != "" {
		templateName = c.Config.TemplateName
	}

	exist, err := c.TemplateExists(templateName)
	if err != nil {
		panic(err)
	}

	if !exist {
		template := c.getDefaultTemplate(indexPrefix)
		if global.Env().IsDebug {
			log.Trace("template: ", template)
		}
		res, err := c.PutTemplate(templateName, []byte(template))
		if err != nil {
			panic(err)
		}

		if strings.Contains(string(res), "error") {
			panic(errors.New(string(res)))
		}
		if global.Env().IsDebug {
			log.Trace("put template response, ", string(res))
		}

		log.Debugf("elasticsearch template successful initialized")

	}

}

// Index index a document into elasticsearch
func (c *ESAPIV0) Index(indexName string, id interface{}, data interface{}) (*elastic.InsertResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := fmt.Sprintf("%s/%s/%s/%s", c.Config.Endpoint, indexName, TypeName6, id)

	js, err := json.Marshal(data)

	if global.Env().IsDebug {
		log.Trace("indexing doc: ", url, ",", string(js))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.Request(util.Verb_POST, url, js)

	if err != nil {
		panic(err)
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

// Get fetch document by id
func (c *ESAPIV0) Get(indexName, id string) (*elastic.GetResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := c.Config.Endpoint + "/" + indexName + "/" + TypeName6 + "/" + id

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	responseHandle(resp)

	if global.Env().IsDebug {
		log.Trace("get response: ", string(resp.Body))
	}

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

// Delete used to delete document by id
func (c *ESAPIV0) Delete(indexName, id string) (*elastic.DeleteResponse, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}
	url := c.Config.Endpoint + "/" + indexName + "/" + TypeName6 + "/" + id

	if global.Env().IsDebug {
		log.Debug("delete doc: ", url)
	}

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	responseHandle(resp)

	if global.Env().IsDebug {
		log.Trace("delete response: ", string(resp.Body))
	}

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

// Count used to count how many docs in one index
func (c *ESAPIV0) Count(indexName string) (*elastic.CountResponse, error) {

	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}

	url := c.Config.Endpoint + "/" + indexName + "/_count"

	if global.Env().IsDebug {
		log.Debug("doc count: ", url)
	}

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	if global.Env().IsDebug {
		log.Trace("count response: ", string(resp.Body))
	}

	esResp := &elastic.CountResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.CountResponse{}, err
	}

	return esResp, nil
}

// Search used to execute a search query
func (c *ESAPIV0) Search(indexName string, query *elastic.SearchRequest) (*elastic.SearchResponse, error) {

	if query.From < 0 {
		query.From = 0
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	js, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	return c.SearchWithRawQueryDSL(indexName, js)
}

func (c *ESAPIV0) SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*elastic.SearchResponse, error) {

	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}

	url := c.Config.Endpoint + "/" + indexName + "/_search"

	if global.Env().IsDebug {
		log.Trace("search: ", url, ",", string(queryDSL))
	}

	resp, err := c.Request(util.Verb_GET, url, queryDSL)
	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	if global.Env().IsDebug {
		log.Trace("search response: ", string(queryDSL), ",", string(resp.Body))
	}

	esResp := &elastic.SearchResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.SearchResponse{}, err
	}

	return esResp, nil
}

func (c *ESAPIV0) IndexExists(indexName string) (bool, error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}

	url := fmt.Sprintf("%s/%s", c.Config.Endpoint, indexName)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		panic(err)
	}

	if resp.StatusCode == 404 {
		return false, nil
	}

	if resp.StatusCode == 200 {
		return true, nil
	}

	return false, nil
}

func (c *ESAPIV0) ClusterVersion() string {
	return c.Version
}

func (c *ESAPIV0) ClusterHealth() *elastic.ClusterHealth {

	url := fmt.Sprintf("%s/_cluster/health", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		log.Error(err)
		return &elastic.ClusterHealth{Name: c.Config.Endpoint, Status: "unreachable"}
	}

	responseHandle(resp)

	health := &elastic.ClusterHealth{}
	err = json.Unmarshal(resp.Body, health)

	if err != nil {
		log.Error(err)
		return &elastic.ClusterHealth{Name: c.Config.Endpoint, Status: "unreachable"}
	}
	return health
}

func (c *ESAPIV0) GetNodes() (*elastic.NodesResponse, error){
	nodes:=&elastic.NodesResponse{}

	url := fmt.Sprintf("%s/_nodes", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		panic(err)
		return nil, err
	}
	responseHandle(resp)
	err = json.Unmarshal(resp.Body, nodes)
	if err != nil {
		panic(err)
		return nil, err
	}
	return nodes, nil
}

func (c *ESAPIV0) Bulk(data *bytes.Buffer) {
	if data == nil || data.Len() == 0 {
		return
	}

	data.WriteRune('\n')

	url := fmt.Sprintf("%s/_bulk", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_POST, url, data.Bytes())

	if err != nil {
		panic(err)
		return
	}

	responseHandle(resp)

	data.Reset()
}

func (c *ESAPIV0) GetIndexSettings(indexNames string) (*elastic.Indexes, error) {

	// get all settings
	allSettings := &elastic.Indexes{}

	url := fmt.Sprintf("%s/%s/_settings", c.Config.Endpoint, indexNames)

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}
	responseHandle(resp)

	err = json.Unmarshal(resp.Body, allSettings)
	if err != nil {
		panic(err)
		return nil, err
	}

	return allSettings, nil
}

func (c *ESAPIV0) GetMapping(copyAllIndexes bool, indexNames string) (string, int, *elastic.Indexes, error) {
	url := fmt.Sprintf("%s/%s/_mapping", c.Config.Endpoint, indexNames)

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		panic(err)
		return "", 0, nil, err
	}
	responseHandle(resp)

	idxs := elastic.Indexes{}
	er := json.Unmarshal(resp.Body, &idxs)

	if er != nil {
		panic(err)
		return "", 0, nil, er
	}

	// remove indexes that start with . if user asked for it
	//if copyAllIndexes == false {
	//      for name := range idxs {
	//              switch name[0] {
	//              case '.':
	//                      delete(idxs, name)
	//              case '_':
	//                      delete(idxs, name)
	//
	//
	//                      }
	//              }
	//      }

	// if _all indexes limit the list of indexes to only these that we kept
	// after looking at mappings
	if indexNames == "_all" {

		var newIndexes []string
		for name := range idxs {
			newIndexes = append(newIndexes, name)
		}
		indexNames = strings.Join(newIndexes, ",")

	} else if strings.Contains(indexNames, "*") || strings.Contains(indexNames, "?") {

		r, _ := regexp.Compile(indexNames)

		//check index patterns
		var newIndexes []string
		for name := range idxs {
			matched := r.MatchString(name)
			if matched {
				newIndexes = append(newIndexes, name)
			}
		}
		indexNames = strings.Join(newIndexes, ",")

	}

	i := 0
	// wrap in mappings if moving from super old es
	for name, idx := range idxs {
		i++
		if _, ok := idx.(map[string]interface{})["mappings"]; !ok {
			(idxs)[name] = map[string]interface{}{
				"mappings": idx,
			}
		}
	}

	return indexNames, i, &idxs, nil
}

func getEmptyIndexSettings() map[string]interface{} {
	tempIndexSettings := map[string]interface{}{}
	tempIndexSettings["settings"] = map[string]interface{}{}
	tempIndexSettings["settings"].(map[string]interface{})["index"] = map[string]interface{}{}
	return tempIndexSettings
}

func cleanSettings(settings map[string]interface{}) {

	if settings == nil {
		return
	}
	//clean up settings
	delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "creation_date")
	delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "uuid")
	delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "version")
	delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "provided_name")
}

func (s *ESAPIV0) UpdateIndexSettings(name string, settings map[string]interface{}) error {
	if s.Config.IndexPrefix != "" {
		name = s.Config.IndexPrefix + name
	}

	if global.Env().IsDebug {
		log.Trace("update index: ", name, ", ", settings)
	}
	cleanSettings(settings)
	url := fmt.Sprintf("%s/%s/_settings", s.Config.Endpoint, name)

	if _, ok := settings["settings"].(map[string]interface{})["index"]; ok {
		if set, ok := settings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"]; ok {
			staticIndexSettings := getEmptyIndexSettings()
			staticIndexSettings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"] = set

			resp, err := s.Request("POST", fmt.Sprintf("%s/%s/_close", s.Config.Endpoint, name), nil)
			responseHandle(resp)

			//TODO error handle

			body := bytes.Buffer{}
			enc := json.NewEncoder(&body)
			enc.Encode(staticIndexSettings)
			resp, err = s.Request("PUT", url, body.Bytes())
			if err != nil {
				panic(err)
			}

			responseHandle(resp)

			delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "analysis")

			resp, err = s.Request("POST", fmt.Sprintf("%s/%s/_open", s.Config.Endpoint, name), nil)
			responseHandle(resp)

			//TODO error handle
		}
	}

	body := bytes.Buffer{}
	enc := json.NewEncoder(&body)
	enc.Encode(settings)
	resp, err := s.Request(util.Verb_PUT, url, body.Bytes())
	responseHandle(resp)

	return err
}

func (s *ESAPIV0) UpdateMapping(indexName string, mappings []byte) ([]byte, error) {
	if s.Config.IndexPrefix != "" {
		indexName = s.Config.IndexPrefix + indexName
	}

	url := fmt.Sprintf("%s/%s/%s/_mapping", s.Config.Endpoint, indexName, TypeName6)

	resp, err := s.Request(util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	responseHandle(resp)

	return resp.Body, nil
}

func (c *ESAPIV0) DeleteIndex(indexName string) (err error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}

	if global.Env().IsDebug {
		log.Trace("start delete index: ", indexName)
	}

	url := fmt.Sprintf("%s/%s", c.Config.Endpoint, indexName)

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	responseHandle(resp)

	return nil
}

func (c *ESAPIV0) CreateIndex(indexName string, settings map[string]interface{}) (err error) {
	if c.Config.IndexPrefix != "" {
		indexName = c.Config.IndexPrefix + indexName
	}

	cleanSettings(settings)

	body := bytes.Buffer{}
	if len(settings) > 0 {
		enc := json.NewEncoder(&body)
		enc.Encode(settings)
	}

	if global.Env().IsDebug {
		log.Trace("start create index: ", indexName, ",", settings, ",", string(body.Bytes()))
	}

	url := fmt.Sprintf("%s/%s", c.Config.Endpoint, indexName)

	resp, err := c.Request(util.Verb_PUT, url, body.Bytes())

	if err != nil {
		panic(err)
	}

	responseHandle(resp)

	return err
}

func (s *ESAPIV0) Refresh(name string) (err error) {
	if s.Config.IndexPrefix != "" {
		name = s.Config.IndexPrefix + name
	}

	if global.Env().IsDebug {
		log.Trace("refresh index: ", name)
	}

	url := fmt.Sprintf("%s/%s/_refresh", s.Config.Endpoint, name)

	resp, err := s.Request(util.Verb_POST, url, nil)

	responseHandle(resp)

	return err
}

func (s *ESAPIV0) NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, fields string) (scroll interface{}, err error) {

	// curl -XGET 'http://es-0.9:9200/_search?search_type=scan&scroll=10m&size=50'
	url := fmt.Sprintf("%s/%s/_search?search_type=scan&scroll=%s&size=%d", s.Config.Endpoint, indexNames, scrollTime, docBufferCount)

	var jsonBody []byte
	if len(query) > 0 || len(fields) > 0 {
		queryBody := map[string]interface{}{}
		if len(fields) > 0 {
			if !strings.Contains(fields, ",") {
				panic(errors.New("The fields shoud be seraprated by ,"))
				return
			} else {
				queryBody["_source"] = strings.Split(fields, ",")
			}
		}

		if len(query) > 0 {
			queryBody["query"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"].(map[string]interface{})["query"] = query

			jsonArray, err := json.Marshal(queryBody)
			if err != nil {
				panic(err)

			} else {
				jsonBody = jsonArray
			}
		}

	}
	resp, err := s.Request(util.Verb_POST, url, jsonBody)

	if err != nil {
		panic(err)
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("new scroll,", url, ",", string(jsonBody))
	}

	if err != nil {
		panic(err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	scroll = &elastic.ScrollResponse{}
	err = json.Unmarshal(resp.Body, scroll)
	if err != nil {
		panic(err)
		return nil, err
	}

	responseHandle(resp)

	return scroll, err
}

func (s *ESAPIV0) NextScroll(scrollTime string, scrollId string) (interface{}, error) {
	//  curl -XGET 'http://es-0.9:9200/_search/scroll?scroll=5m'
	id := bytes.NewBufferString(scrollId)
	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", s.Config.Endpoint, scrollTime, id)
	resp, err := s.Request(util.Verb_GET, url, nil)

	if err != nil {
		panic(err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	if global.Env().IsDebug {
		log.Trace("next scroll,", url, "m,", string(resp.Body))
	}
	// decode elasticsearch scroll response
	scroll := &elastic.ScrollResponse{}
	err = json.Unmarshal(resp.Body, &scroll)
	if err != nil {
		panic(err)
		return nil, err
	}

	responseHandle(resp)

	return scroll, nil
}

func (c *ESAPIV0) TemplateExists(templateName string) (bool, error) {
	url := fmt.Sprintf("%s/_template/%s", c.Config.Endpoint, templateName)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil || resp.StatusCode == 404 {
		return false, err
	} else {
		return true, nil
	}

	responseHandle(resp)

	return false, nil
}

func (c *ESAPIV0) PutTemplate(templateName string, template []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/_template/%s", c.Config.Endpoint, templateName)
	resp, err := c.Request(util.Verb_PUT, url, template)

	if err != nil {
		return nil, err
	}

	responseHandle(resp)

	return resp.Body, nil
}

func responseHandle(resp *util.Result) {
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 404 {
		panic(errors.New(string(resp.Body)))
	}

	if global.Env().IsDebug {
		log.Trace(util.ToJson(resp, true))
	}

}

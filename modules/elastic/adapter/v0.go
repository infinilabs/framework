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
	"regexp"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	errors2 "infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV0 struct {
	Version string
	Config  elastic.ElasticsearchConfig
	majorVersion int
}

func (c ESAPIV0) GetMajorVersion() int {
	if c.majorVersion>0{
		return c.majorVersion
	}
	vs := strings.Split(c.Version, ".")
	n, err := util.ToInt(vs[0])
	if err != nil {
		panic(err)
	}
	c.majorVersion=n
	return n
}

const TypeName6 = "doc"

func (c *ESAPIV0) Request(method, url string, body []byte) (result *util.Result, err error) {

	if global.Env().IsDebug {
		log.Trace(method, ",", url, ",", util.SubString(string(body), 0, 3000))
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

	if !global.Env().IsDebug {
		defer func(data *util.Request) (result *util.Result, err error) {
			var resp *util.Result
			if err := recover(); err != nil {
				var count = 0
			RETRY:
				if count > 10 {
					log.Errorf("still have error in request, after retry [%v] times\n", err)
					return resp, errors2.Errorf("still have error in request, after retry [%v] times\n", err)
				}
				count++
				log.Errorf("error in request, sleep 10s and retry [%v]: %s\n", count, err)
				time.Sleep(10 * time.Second)
				resp, err = util.ExecuteRequestWithCatchFlag(req, true)
				if err != nil {
					log.Errorf("retry still have error in request, sleep 10s and retry [%v]: %s\n", count, err)
					goto RETRY
				}
			}
			return resp, err
		}(req)
	}

	resp, err := util.ExecuteRequest(req)
	if err != nil {
		return resp, err
	}

	return resp, err
}

func (c *ESAPIV0) InitDefaultTemplate(templateName, indexPrefix string) {
	c.initTemplate(templateName, indexPrefix)
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

func (c *ESAPIV0) initTemplate(templateName, indexPrefix string) {
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
func (c *ESAPIV0) Index(indexName, docType string, id interface{}, data interface{}) (*elastic.InsertResponse, error) {

	if docType==""{
		docType=TypeName6
	}

	url := fmt.Sprintf("%s/%s/%s/%s", c.Config.Endpoint, indexName, docType, id)

	if id==""{
		url = fmt.Sprintf("%s/%s/%s/", c.Config.Endpoint, indexName, docType)
	}

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

// Get fetch document by id
func (c *ESAPIV0) Get(indexName, docType, id string) (*elastic.GetResponse, error) {

	if docType==""{
		docType=TypeName6
	}

	url := c.Config.Endpoint + "/" + indexName + "/" + docType + "/" + id

	resp, err := c.Request(util.Verb_GET, url, nil)
	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode=resp.StatusCode

	if global.Env().IsDebug {
		log.Trace("get response: ", string(resp.Body))
	}

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	if !esResp.Found {
		return esResp, errors.New(string(resp.Body))
	}

	return esResp, nil
}

// Delete used to delete document by id
func (c *ESAPIV0) Delete(indexName, docType, id string) (*elastic.DeleteResponse, error) {
	url := c.Config.Endpoint + "/" + indexName + "/" + docType + "/" + id

	if global.Env().IsDebug {
		log.Debug("delete doc: ", url)
	}

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("delete response: ", string(resp.Body))
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

// Count used to count how many docs in one index
func (c *ESAPIV0) Count(indexName string) (*elastic.CountResponse, error) {
	url := c.Config.Endpoint + "/" + indexName + "/_count"

	if global.Env().IsDebug {
		log.Debug("doc count: ", url)
	}

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

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
	url := c.Config.Endpoint + "/" + indexName + "/_search"
	esResp := &elastic.SearchResponse{}

	if global.Env().IsDebug {
		log.Trace("search: ", url, ",", string(queryDSL))
	}

	resp, err := c.Request(util.Verb_GET, url, queryDSL)
	if resp!=nil{
		esResp.StatusCode=resp.StatusCode
		esResp.ErrorObject=err
	}

	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("search response: ", string(queryDSL), ",", string(resp.Body))
	}

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

func (c *ESAPIV0) IndexExists(indexName string) (bool, error) {
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

func (c *ESAPIV0) GetNodesStats() *elastic.NodesStats {
	// /_nodes/_local/stats
	// /_nodes/_all/stats`

	url := fmt.Sprintf("%s/_nodes/_all/stats", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	obj:= &elastic.NodesStats{}
	if err != nil {
		if resp!=nil{
			obj.StatusCode=resp.StatusCode
		}else{
			obj.StatusCode=500
		}
		obj.ErrorObject=err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode=resp.StatusCode
		obj.ErrorObject=err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetIndicesStats() *elastic.IndicesStats {
	// /_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices
	url := fmt.Sprintf("%s/_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	obj:= &elastic.IndicesStats{}
	if err != nil {
		if resp!=nil{
			obj.StatusCode=resp.StatusCode
		}else{
			obj.StatusCode=500
		}
		obj.ErrorObject=err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode=resp.StatusCode
		obj.ErrorObject=err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetClusterStats() *elastic.ClusterStats {
	//_cluster/stats
	url := fmt.Sprintf("%s/_cluster/stats", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	obj:= &elastic.ClusterStats{}
	if err != nil {
		if resp!=nil{
			obj.StatusCode=resp.StatusCode
		}else{
			obj.StatusCode=500
		}
		obj.ErrorObject=err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode=resp.StatusCode
		obj.ErrorObject=err
		return obj
	}

	return obj
}

func (c *ESAPIV0) ClusterHealth() *elastic.ClusterHealth {

	url := fmt.Sprintf("%s/_cluster/health", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)
	obj:= &elastic.ClusterHealth{}
	if err != nil {
		if resp!=nil{
			obj.StatusCode=resp.StatusCode
		}else{
			obj.StatusCode=500
		}
		obj.ErrorObject=err
		return obj
	}

	health := &elastic.ClusterHealth{}
	err = json.Unmarshal(resp.Body, health)

	if err != nil {
		obj.StatusCode=resp.StatusCode
		obj.ErrorObject=err
		return obj
	}
	return health
}

func (c *ESAPIV0) GetNodes() (*map[string]elastic.NodesInfo, error) {
	nodes := &elastic.NodesResponse{}

	url := fmt.Sprintf("%s/_nodes", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp.Body, nodes)
	if err != nil {
		return nil, err
	}
	return &nodes.Nodes, nil
}

//{
//"health" : "green",
//"status" : "open",
//"index" : ".monitoring-kibana-7-2021.01.01",
//"uuid" : "Kdkyc5QNS1ekTXTQ-Q-Row",
//"pri" : "1",
//"rep" : "0",
//"docs.count" : "17278",
//"docs.deleted" : "0",
//"store.size" : "2.9mb",
//"pri.store.size" : "2.9mb"
//}
type CatIndexResponse struct {
	Index        string `json:"index,omitempty"`
	Uuid         string `json:"uuid,omitempty"`
	Status       string `json:"status,omitempty"`
	Health       string `json:"health,omitempty"`
	Pri          string `json:"pri,omitempty"`
	Rep          string `json:"rep,omitempty"`
	DocsCount    string `json:"docs.count,omitempty"`
	DocsDeleted  string `json:"docs.deleted,omitempty"`
	StoreSize    string `json:"store.size,omitempty"`
	PriStoreSize string `json:"pri.store.size,omitempty"`
}

func (c *ESAPIV0) GetIndices() (*map[string]elastic.IndexInfo, error) {

	url := fmt.Sprintf("%s/_cat/indices?v&format=json", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}
	data := []CatIndexResponse{}
	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	indexInfo := map[string]elastic.IndexInfo{}
	for _, v := range data {
		info := elastic.IndexInfo{}
		info.ID = v.Uuid
		info.Index = v.Index
		info.Status = v.Status
		info.Health = v.Health

		info.Shards, _ = util.ToInt(v.Pri)
		info.Replicas, _ = util.ToInt(v.Rep)
		info.DocsCount, _ = util.ToInt64(v.DocsCount)
		info.DocsDeleted, _ = util.ToInt64(v.DocsDeleted)

		indexInfo[v.Index] = info
	}

	return &indexInfo, nil
}

//{
//"index" : ".monitoring-es-7-2020.12.29",
//"shard" : "0",
//"prirep" : "p",
//"state" : "STARTED",
//"unassigned.reason" : null,
//"docs" : "227608",
//"store" : "132.5mb",
//"id" : "qIgTsxtuQ8mzAGiBATkqHw",
//"node" : "dev",
//"ip" : "192.168.3.98"
//}
type CatShardResponse struct {
	Index            string `json:"index,omitempty"`
	ShardID          string `json:"shard,omitempty"`
	ShardType        string `json:"prirep,omitempty"`
	State            string `json:"state,omitempty"`
	UnassignedReason string `json:"unassigned,omitempty"`
	Docs             string `json:"docs,omitempty"`
	Store            string `json:"store,omitempty"`
	NodeID           string `json:"id,omitempty"`
	NodeName         string `json:"node,omitempty"`
	NodeIP           string `json:"ip,omitempty"`
}

//index:shardID -> nodesInfo
func (c *ESAPIV0) GetPrimaryShards() (*map[string]elastic.ShardInfo, error) {
	data := []CatShardResponse{}

	url := fmt.Sprintf("%s/_cat/shards?v&h=index,shard,prirep,state,unassigned.reason,docs,store,id,node,ip&format=json", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	infos := map[string]elastic.ShardInfo{}
	for _, v := range data {
		if v.ShardType != "p" {
			continue
		}

		info := elastic.ShardInfo{}
		info.Index = v.Index
		info.ShardID = v.ShardID
		info.Primary = v.ShardType == "p"

		info.State = v.State
		info.Docs, err = util.ToInt64(v.Docs)
		if err != nil {
			info.Docs = 0
		}
		info.Store = v.Store
		info.NodeID = v.NodeID
		info.NodeName = v.NodeName
		info.NodeIP = v.NodeIP

		infos[fmt.Sprintf("%v:%v", info.Index, info.ShardID)] = info
	}
	return &infos, nil
}

func (c *ESAPIV0) Bulk(data *bytes.Buffer) {
	if data == nil || data.Len() == 0 {
		return
	}
	defer data.Reset()
	data.WriteRune('\n')

	url := fmt.Sprintf("%s/_bulk", c.Config.Endpoint)
	_, err := c.Request(util.Verb_POST, url, data.Bytes())

	if err != nil {
		panic(err)
		return
	}

}

func (c *ESAPIV0) GetIndexSettings(indexNames string) (*elastic.Indexes, error) {

	// get all settings
	allSettings := &elastic.Indexes{}

	url := fmt.Sprintf("%s/%s/_settings?include_defaults", c.Config.Endpoint, indexNames)

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resp.Body, allSettings)
	if err != nil {
		return nil, err
	}

	return allSettings, nil
}

func (c *ESAPIV0) GetMapping(copyAllIndexes bool, indexNames string) (string, int, *elastic.Indexes, error) {
	url := fmt.Sprintf("%s/%s/_mapping", c.Config.Endpoint, indexNames)

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return "", 0, nil, err
	}

	idxs := elastic.Indexes{}
	er := json.Unmarshal(resp.Body, &idxs)

	if er != nil {
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
	if global.Env().IsDebug {
		log.Trace("update index: ", name, ", ", settings)
	}
	cleanSettings(settings)
	url := fmt.Sprintf("%s/%s/_settings", s.Config.Endpoint, name)

	if _, ok := settings["settings"].(map[string]interface{})["index"]; ok {
		if set, ok := settings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"]; ok {
			staticIndexSettings := getEmptyIndexSettings()
			staticIndexSettings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"] = set

			_, err := s.Request("POST", fmt.Sprintf("%s/%s/_close", s.Config.Endpoint, name), nil)

			//TODO error handle

			body := bytes.Buffer{}
			enc := json.NewEncoder(&body)
			enc.Encode(staticIndexSettings)
			_, err = s.Request("PUT", url, body.Bytes())
			if err != nil {
				panic(err)
			}

			delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "analysis")

			_, err = s.Request("POST", fmt.Sprintf("%s/%s/_open", s.Config.Endpoint, name), nil)

			//TODO error handle
		}
	}

	body := bytes.Buffer{}
	enc := json.NewEncoder(&body)
	enc.Encode(settings)
	_, err := s.Request(util.Verb_PUT, url, body.Bytes())

	return err
}

func (s *ESAPIV0) UpdateMapping(indexName string, mappings []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s/_mapping", s.Config.Endpoint, indexName, TypeName6)

	resp, err := s.Request(util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	return resp.Body, nil
}

func (c *ESAPIV0) DeleteIndex(indexName string) (err error) {
	if global.Env().IsDebug {
		log.Trace("start delete index: ", indexName)
	}

	url := fmt.Sprintf("%s/%s", c.Config.Endpoint, indexName)

	c.Request(util.Verb_DELETE, url, nil)

	return nil
}

func (c *ESAPIV0) CreateIndex(indexName string, settings map[string]interface{}) (err error) {

	//cleanSettings(settings)

	body := bytes.Buffer{}
	if len(settings) > 0 {
		enc := json.NewEncoder(&body)
		enc.Encode(settings)
	}

	if global.Env().IsDebug {
		log.Trace("start create index: ", indexName, ",", settings, ",", string(body.Bytes()))
	}

	url := fmt.Sprintf("%s/%s", c.Config.Endpoint, indexName)

	_, err = c.Request(util.Verb_PUT, url, body.Bytes())

	if err != nil {
		panic(err)
	}

	return err
}

func (s *ESAPIV0) Refresh(name string) (err error) {
	url := fmt.Sprintf("%s/%s/_refresh", s.Config.Endpoint, name)

	_, err = s.Request(util.Verb_POST, url, nil)

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

	return scroll, err
}

func (s *ESAPIV0) NextScroll(scrollTime string, scrollId string) (interface{}, error) {
	//  curl -XGET 'http://es-0.9:9200/_search/scroll?scroll=5m'
	id := bytes.NewBufferString(scrollId)
	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", s.Config.Endpoint, scrollTime, id)
	resp, err := s.Request(util.Verb_GET, url, nil)

	if err != nil {
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
		return nil, err
	}

	return scroll, nil
}

func (c *ESAPIV0) TemplateExists(templateName string) (bool, error) {
	url := fmt.Sprintf("%s/_template/%s", c.Config.Endpoint, templateName)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil || resp != nil && resp.StatusCode == 404 {
		return false, err
	} else {
		return true, nil
	}

	return false, nil
}

func (c *ESAPIV0) PutTemplate(templateName string, template []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/_template/%s", c.Config.Endpoint, templateName)
	resp, err := c.Request(util.Verb_PUT, url, template)

	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *ESAPIV0) SearchTasksByIds(ids []string) (*elastic.SearchResponse, error) {
	if len(ids) == 0 {
		return nil, errors.New("param ids can not be empty")
	}
	esBody := `{
  "query":{
    "terms": {
      "_id": [
      %s
      ]
    }
  }
}`
	strTerms := ""
	for _, term := range ids {
		strTerms += fmt.Sprintf(`"%s",`, term)
	}
	esBody = fmt.Sprintf(esBody, strTerms[0:len(strTerms)-1])
	return c.SearchWithRawQueryDSL(".tasks", []byte(esBody))
}

func (c *ESAPIV0) Reindex(body []byte) (*elastic.ReindexResponse, error) {
	url := fmt.Sprintf("%s/_reindex?wait_for_completion=false", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_POST, url, body)
	if err != nil {
		return nil, err
	}
	var reindexResponse = &elastic.ReindexResponse{}
	err = json.Unmarshal(resp.Body, reindexResponse)
	if err != nil {
		return nil, err
	}
	return reindexResponse, nil
}

func (c *ESAPIV0) GetIndexStats(indexName string) (*elastic.IndexStats, error) {
	url := fmt.Sprintf("%s/%s/_stats", c.Config.Endpoint, indexName)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	var response = &elastic.IndexStats{}
	err = json.Unmarshal(resp.Body, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

//"dict" : {
	//"aliases" : {
	//"dictalias1" : {
	//"is_write_index" : true
	//},
	//"dictalias2" : {
	//"is_write_index" : true
	//}
	//}
//},
type AliasesResponse struct {
		Aliases map[string]struct{
			IsWriteIndex  bool   `json:"is_write_index,omitempty"`
			IsHiddenIndex  bool   `json:"is_hidden,omitempty"`
			IndexRouting  string `json:"index_routing,omitempty"`
			SearchRouting string `json:"search_routing,omitempty"`
			Filter        interface{} `json:"filter,omitempty"`
		}`json:"aliases,omitempty"`
}

func (c *ESAPIV0) GetAliases() (*map[string]elastic.AliasInfo, error) {

	url := fmt.Sprintf("%s/_alias", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}
	data := map[string]AliasesResponse{}
	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	aliasInfo := map[string]elastic.AliasInfo{}
	for index, v := range data {
		for alias,v1 :=range v.Aliases{
			info,ok:=aliasInfo[alias]
			if !ok{
				info = elastic.AliasInfo{}
				info.Alias = alias
			}

			info.Index = append(info.Index,index)
			if v1.IsWriteIndex{
				info.WriteIndex = index
			}
			aliasInfo[alias] = info
		}
	}

	if global.Env().IsDebug{
		log.Trace("get alias:",util.ToJson(aliasInfo,false))
	}

	return &aliasInfo, nil
}

func (c *ESAPIV0) Forcemerge(indexName string, maxCount int) error {
	url := fmt.Sprintf("%s/%s/_forcemerge?max_num_segments=%v", c.Config.Endpoint, indexName, maxCount)
	_, err := c.Request(util.Verb_POST, url, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *ESAPIV0) DeleteByQuery(indexName string, body []byte) (*elastic.DeleteByQueryResponse, error) {
	url := fmt.Sprintf("%s/%s/_delete_by_query", c.Config.Endpoint, indexName)
	resp, err := c.Request(util.Verb_POST, url, body)
	if err != nil {
		return nil, err
	}
	var delResponse = &elastic.DeleteByQueryResponse{}
	err = json.Unmarshal(resp.Body, delResponse)
	if err != nil {
		return nil, err
	}
	return delResponse, nil
}

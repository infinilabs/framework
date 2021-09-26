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
	"errors"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/segmentio/encoding/json"
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
	Version      string
	Config       elastic.ElasticsearchConfig
	majorVersion int
}

func (c ESAPIV0) GetMajorVersion() int {
	if c.majorVersion > 0 {
		return c.majorVersion
	}

	if c.Version == "" && c.Config.Endpoint != "" {
		c.Version, _ = GetMajorVersion(c.Config)
	}

	if c.Version != "" {
		vs := strings.Split(c.Version, ".")
		n, err := util.ToInt(vs[0])
		if err != nil {
			panic(err)
		}
		c.majorVersion = n
		return n
	}

	panic("invalid major version")
}

const TypeName0 = "doc"

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
    "number_of_shards": %v
  },
  "mappings": {
    "%s": {
      "dynamic_templates": [
        {
          "strings": {
            "match_mapping_type": "string",
            "mapping": {
              "type": "string",
               "index": "not_analyzed"
            }
          }
        }
      ]
    }
  }
}
`
	return fmt.Sprintf(template, indexPrefix, 1, TypeName0)
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

	if docType == "" {
		docType = TypeName0
	}

	url := fmt.Sprintf("%s/%s/%s/%s?refresh=wait_for", c.Config.Endpoint, indexName, docType, id)

	if id == "" {
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

	if docType == "" {
		docType = TypeName0
	}

	url := c.Config.Endpoint + "/" + indexName + "/" + docType + "/" + id

	resp, err := c.Request(util.Verb_GET, url, nil)
	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode = resp.StatusCode

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
func (c *ESAPIV0) Delete(indexName, docType, id string, refresh ...string) (*elastic.DeleteResponse, error) {
	url := c.Config.Endpoint + "/" + indexName + "/" + docType + "/" + id

	if global.Env().IsDebug {
		log.Debug("delete doc: ", url)
	}
	if len(refresh)>0 {
		url = url + "?refresh=" + refresh[0]
	}

	resp, err := c.Request(util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("delete response: ", string(resp.Body))
	}

	esResp := &elastic.DeleteResponse{}
	esResp.StatusCode = resp.StatusCode

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.DeleteResponse{}, err
	}
	if esResp.Result != "deleted" && esResp.Result != "not_found" {
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
	if resp != nil {
		esResp.StatusCode = resp.StatusCode
		esResp.ErrorObject = err
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

	obj := &elastic.NodesStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.ErrorObject = err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.ErrorObject = err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetIndicesStats() *elastic.IndicesStats {
	// /_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices
	url := fmt.Sprintf("%s/_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	obj := &elastic.IndicesStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.ErrorObject = err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.ErrorObject = err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetClusterStats() *elastic.ClusterStats {
	//_cluster/stats
	url := fmt.Sprintf("%s/_cluster/stats", c.Config.Endpoint)

	resp, err := c.Request(util.Verb_GET, url, nil)

	obj := &elastic.ClusterStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.ErrorObject = err
		return obj
	}

	//dirty fix for es 7.0.0
	if c.GetMajorVersion()==7{
		v,err:=jsonparser.GetInt(resp.Body,"indices","segments","max_unsafe_auto_id_timestamp")
		if err!=nil||v< -1{
			d,err:=jsonparser.Set(resp.Body,[]byte("-1"),"indices","segments","max_unsafe_auto_id_timestamp")
			if err==nil{
				resp.Body=d
			}
		}
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.ErrorObject = err
		return obj
	}

	return obj
}

func (c *ESAPIV0) ClusterHealth() *elastic.ClusterHealth {

	url := fmt.Sprintf("%s/_cluster/health", c.Config.Endpoint)
	health := &elastic.ClusterHealth{}

	resp, err := c.Request(util.Verb_GET, url, nil)

	if resp != nil {
		health.StatusCode = resp.StatusCode
	} else {
		health.StatusCode = 500
	}

	if err != nil {
		health.ErrorObject = err
		return health
	}

	err = json.Unmarshal(resp.Body, health)

	if err != nil {
		health.StatusCode = resp.StatusCode
		health.ErrorObject = err
		return health
	}
	return health
}

func (c *ESAPIV0) GetNodes() (*map[string]elastic.NodesInfo, error) {

	url := fmt.Sprintf("%s/_nodes", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	node := elastic.NodesResponse{}

	err=node.UnmarshalJSON(resp.Body)
	if err != nil {
		return nil, err
	}
	return &node.Nodes, nil
}

func (c *ESAPIV0) GetIndices(pattern string) (*map[string]elastic.IndexInfo, error) {

	url := fmt.Sprintf("%s/_cat/indices?v&h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size,segments.count&format=json", c.Config.Endpoint)
	if pattern != "" {
		url = fmt.Sprintf("%s/_cat/indices/%s?v&h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size,segments.count&format=json", c.Config.Endpoint, pattern)
	}

	resp, err := c.Request(util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := []elastic.CatIndexResponse{}
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
		info.SegmentsCount, _ = util.ToInt64(v.SegmentCount)

		info.StoreSize = v.StoreSize
		info.PriStoreSize = v.PriStoreSize

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
func (c *ESAPIV0) GetPrimaryShards() (*map[string]map[int]elastic.ShardInfo, error) {
	data := []CatShardResponse{}

	url := fmt.Sprintf("%s/_cat/shards?v&h=index,shard,prirep,state,unassigned.reason,docs,store,id,node,ip&format=json", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	infos := map[string]map[int]elastic.ShardInfo{}
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

		indexMap, ok := infos[v.Index]
		if !ok {
			indexMap = map[int]elastic.ShardInfo{}
		}
		id,err:=util.ToInt(v.ShardID)
		if err!=nil{
			log.Error("invalid shard id, it should be number,",string(resp.Body))
			return nil, err
		}
		indexMap[id] = info
		infos[v.Index] = indexMap

		// infos[fmt.Sprintf("%v:%v", info.Index, info.ShardID)] = info
	}
	return &infos, nil
}

func (c *ESAPIV0) Bulk(data []byte) {
	if data == nil || len(data) == 0 {
		return
	}

	url := fmt.Sprintf("%s/_bulk", c.Config.Endpoint)
	result, err := c.Request(util.Verb_POST, url, data)

	if global.Env().IsDebug{
		log.Trace(string(result.Body),err)
	}

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

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
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
	url := fmt.Sprintf("%s/%s/%s/_mapping", s.Config.Endpoint, indexName, TypeName0)

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

func (s *ESAPIV0) NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, sourceFields string, sortField, sortType string) ( elastic.ScrollResponseAPI,  error) {

	// curl -XGET 'http://es-0.9:9200/_search?search_type=scan&scroll=10m&size=50'
	url := fmt.Sprintf("%s/%s/_search?search_type=scan&scroll=%s&size=%d", s.Config.Endpoint, indexNames, scrollTime, docBufferCount)

	var jsonBody []byte
	if len(query) > 0 || len(sourceFields) > 0 || true {
		queryBody := map[string]interface{}{}
		if len(sourceFields) > 0 {
			if !strings.Contains(sourceFields, ",") {
				panic(errors.New("The source fields shoud be seraprated by ,"))
			} else {
				queryBody["_source"] = strings.Split(sourceFields, ",")
			}
		}

		if len(sortField) > 0 {
			if len(sortType) == 0 {
				sortType = "asc"
			}
			sort := []map[string]interface{}{}
			sort = append(sort, util.MapStr{
				sortField: util.MapStr{
					"order": sortType,
				},
			})
			queryBody["sort"] = sort
		}

		if len(query) > 0 {
			queryBody["query"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"].(map[string]interface{})["query"] = query
		}

		jsonArray, err := json.Marshal(queryBody)
		if err != nil {
			panic(err)

		} else {
			jsonBody = jsonArray
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

	scroll := &elastic.ScrollResponse{}
	err = scroll.UnmarshalJSON(resp.Body)
	if err != nil {
		panic(err)
		return nil, err
	}

	return scroll, err
}

func (s *ESAPIV0) NextScroll(scrollTime string, scrollId string) ([]byte, error) {
	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", s.Config.Endpoint, scrollTime, scrollId)
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

	return resp.Body, nil
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

func (c *ESAPIV0) GetStats() (*elastic.Stats, error) {
	url := fmt.Sprintf("%s/_stats", c.Config.Endpoint)
	resp, err := c.Request(util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	var response = &elastic.Stats{}
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
	Aliases map[string]struct {
		IsWriteIndex  bool        `json:"is_write_index,omitempty"`
		IsHiddenIndex bool        `json:"is_hidden,omitempty"`
		IndexRouting  string      `json:"index_routing,omitempty"`
		SearchRouting string      `json:"search_routing,omitempty"`
		Filter        interface{} `json:"filter,omitempty"`
	} `json:"aliases,omitempty"`
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
		for alias, v1 := range v.Aliases {
			info, ok := aliasInfo[alias]
			if !ok {
				info = elastic.AliasInfo{}
				info.Alias = alias
			}

			info.Index = append(info.Index, index)
			if v1.IsWriteIndex {
				info.WriteIndex = index
			}
			aliasInfo[alias] = info
		}
	}

	if global.Env().IsDebug {
		log.Trace("get alias:", util.ToJson(aliasInfo, false))
	}

	return &aliasInfo, nil
}


func (c *ESAPIV0) GetAliasesDetail() (*map[string]elastic.AliasDetailInfo, error) {

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

	aliasInfo := map[string]elastic.AliasDetailInfo{}
	for index, v := range data {
		for alias, v1 := range v.Aliases {
			info, ok := aliasInfo[alias]
			if !ok {
				info = elastic.AliasDetailInfo{}
				info.Alias = alias
			}

			info.Indexes = append(info.Indexes, elastic.AliasIndex{
				Index:         index,
				Filter:        v1.Filter,
				SearchRouting: v1.SearchRouting,
				IndexRouting:  v1.IndexRouting,
				IsHidden:      v1.IsHiddenIndex,
				IsWriteIndex:  v1.IsWriteIndex,
			})
			if v1.IsWriteIndex {
				info.WriteIndex = index
			}
			aliasInfo[alias] = info
		}
	}

	return &aliasInfo, nil
}

func (c *ESAPIV0) GetAliasesAndIndices() (*elastic.AliasAndIndicesResponse, error) {

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

	resInfo := elastic.AliasAndIndicesResponse{}
	aliasInfo := map[string]elastic.AAIR_Alias{}
	for index, v := range data {
		idxItem :=  elastic.AAIR_Indices{
			Name:       index,
			Attributes: []string{"open"},
		}
		for alias,_ :=range v.Aliases{
			idxItem.Aliases = append(idxItem.Aliases, alias)
			info,ok:=aliasInfo[alias]
			if !ok{
				info = elastic.AAIR_Alias{
					Name: alias,
				}
			}
			info.Indices = append(info.Indices, index)
			aliasInfo[alias] = info
		}
		resInfo.Indices = append(resInfo.Indices, idxItem)
	}
	for _, alias := range aliasInfo {
		resInfo.Aliases = append(resInfo.Aliases, alias)
	}

	return &resInfo, nil
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

func (c *ESAPIV0) SetSearchTemplate(templateID string, body []byte) error {
	url := fmt.Sprintf("%s/_search/template/%s", c.Config.Endpoint, templateID)
	_, err := c.Request(util.Verb_POST, url, body)
	return err
}

func (c *ESAPIV0) DeleteSearchTemplate(templateID string) error {
	url := fmt.Sprintf("%s/_search/template/%s", c.Config.Endpoint, templateID)
	_, err := c.Request(util.Verb_DELETE, url, nil)
	return err
}

func (c *ESAPIV0) RenderTemplate(body map[string]interface{}) ([]byte, error) {
	if c.Version < "5.6" {
		if source, ok := body["source"]; ok {
			body["inline"] = source
			delete(body, "source")
		}
	}
	url := fmt.Sprintf("%s/_render/template", c.Config.Endpoint)
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(util.Verb_GET, url, bytesBody)
	return res.Body, err
}

func (c *ESAPIV0) SearchTemplate(body map[string]interface{}) ([]byte, error) {
	if c.Version < "5.6" {
		if source, ok := body["source"]; ok {
			body["inline"] = source
			delete(body, "source")
		}
	}
	url := fmt.Sprintf("%s/_search/template", c.Config.Endpoint)
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(util.Verb_GET, url, bytesBody)
	return res.Body, err
}

func (c *ESAPIV0) Alias(body []byte) error {
	url := fmt.Sprintf("%s/_aliases", c.Config.Endpoint)
	_, err := c.Request(util.Verb_POST, url, body)
	return err
}

func (c *ESAPIV0) FieldCaps(target string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/_mappings", c.Config.Endpoint, target)
	res, err := c.Request(util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	mappingsRes := map[string] interface{}{}
	err = json.Unmarshal(res.Body, &mappingsRes)

	if err != nil {
		return nil, err
	}

	var (
		indices []string
		fields = map[string]interface{}{}
	)

	for indexName, mappingsInterface := range mappingsRes {
		indices = append(indices, indexName)
		if mappings, ok := mappingsInterface.(map[string]interface{}); ok {
			if mappingsValue, ok := mappings["mappings"].(map[string]interface{}); ok {
				for _, docInterface := range mappingsValue {
					if docTypeValue, ok := docInterface.(map[string]interface{}); ok {
						if propertiesInterface, ok := docTypeValue["properties"]; ok {
							if properties, ok := propertiesInterface.(map[string]interface{}); ok {
								walkProperties(fields, properties, "")
							}
						}
					}
				}
			}
		}
	}
	result := map[string]interface{}{
		"indices": indices,
		"fields": fields,
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return resultBytes, nil
}

 func walkProperties(fields, properties map[string]interface{}, prefix string){
	var dotFieldName string
	for fieldName, fieldInterface := range properties {
		if prefix != ""{
			dotFieldName = fmt.Sprintf("%s.%s", prefix, fieldName)
		}else{
			dotFieldName = fieldName
		}
		if field, ok := fieldInterface.(map[string]interface{}); ok {
			if esType, ok := field["type"].(string); ok {
				fields[dotFieldName] = map[string]interface{}{
					esType: map[string]interface{}{
						"type": field["type"],
						"searchable":true,
						"aggregatable": true,
					},
				}
			}else if propertiesInterface, ok := field["properties"]; ok {
				if subProperties, ok := propertiesInterface.(map[string]interface{}); ok {
					walkProperties(fields, subProperties, dotFieldName)
				}
			}
		}
	}
}

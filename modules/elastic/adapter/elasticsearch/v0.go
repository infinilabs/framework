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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"infini.sh/framework/modules/elastic/adapter"

	"github.com/buger/jsonparser"
	"github.com/segmentio/encoding/json"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV0 struct {
	Elasticsearch string
	Version       elastic.Version
	metadata      *elastic.ElasticsearchMetadata
	metaLocker    sync.Mutex
}

func (c *ESAPIV0) GetActivePreferredEndpoint(host string) string {
	return c.GetMetadata().GetActivePreferredEndpoint(host)
}

func (c *ESAPIV0) GetEndpoint() string {
	return c.GetMetadata().GetActiveEndpoint()
}
func (c *ESAPIV0) GetMetadata() *elastic.ElasticsearchMetadata {
	c.metaLocker.Lock()
	defer c.metaLocker.Unlock()

	if c.metadata != nil {
		return c.metadata
	}
	if meta := elastic.GetMetadata(c.Elasticsearch); meta !=nil {
		c.metadata = meta
		return c.metadata
	}

	cfg := elastic.GetConfig(c.Elasticsearch)
	if cfg != nil {
		meta := elastic.GetOrInitMetadata(cfg)
		if meta != nil {
			c.metadata = meta
		}
	}
	if c.metadata == nil {
		panic(errors.Errorf("metadata not found for [%v]", c.Elasticsearch))
	}
	return c.metadata
}

func (c *ESAPIV0) GetVersion() elastic.Version {
	if c.Version.Number == "" && c.GetEndpoint() != "" {
		c.Version, _ = adapter.GetMajorVersion(c.GetMetadata())
	}
	return c.Version
}

func (c *ESAPIV0) GetMajorVersion() int {
	if c.Version.Major > 0 {
		return c.Version.Major
	}

	c.GetVersion()
	if c.Version.Major > 0 {
		return c.Version.Major
	}

	log.Debugf("failed to get the major version of elasticsearch [%v], fallback to v0", c.GetMetadata().Config.Name)
	return 0
}

const TypeName0 = "doc"

func (c *ESAPIV0) Request(ctx context.Context, method, url string, body []byte) (result *util.Result, err error) {

	if global.Env().IsDebug {
		log.Trace(method, ",", url, ",", util.SubString(util.UnsafeBytesToString(body), 0, 3000))
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
	case util.Verb_HEAD:
		req = util.NewRequest(util.Verb_HEAD, url)
		break
	}

	if req == nil {
		panic(errors.Errorf("invalid request [%v] %v", method, url))
	}

	req.Context = ctx

	req.SetContentType(util.ContentTypeJson)
	if c.GetMetadata().Config.BasicAuth != nil {
		req.SetBasicAuth(c.GetMetadata().Config.BasicAuth.Username, c.GetMetadata().Config.BasicAuth.Password.Get())
	}

	if c.GetMetadata().Config.HttpProxy != "" {
		req.SetProxy(c.GetMetadata().Config.HttpProxy)
	}

	if !global.Env().IsDebug {
		defer func(data *util.Request) (result *util.Result, err error) {
			var resp *util.Result
			if err := recover(); err != nil {
				var count = 0
			RETRY:
				if count > 3 {
					log.Errorf("still have error in request, after retry [%v] times\n", err)
					return resp, errors.Errorf("still have error in request, after retry [%v] times\n", err)
				}
				count++
				log.Errorf("error in request, sleep 1s and retry [%v]: %s\n", count, err)
				time.Sleep(1 * time.Second)
				var err1 error
				resp, err1 = util.ExecuteRequest(req)
				if err1 != nil {
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
func (c *ESAPIV0) Index(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName0
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
		log.Trace("indexing doc: ", url, ",", string(js))
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
	if !(esResp.Result == "created" || esResp.Result == "updated" || esResp.Shards.Successful > 0) {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

func (c *ESAPIV0) Update(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName0
	}

	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/%s/%s/_update", c.GetEndpoint(), indexName, docType, id)

	if id == "" {
		panic(errors.New("id is required"))
	}
	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s", url, refresh)
	}

	js:=util.MapStr{}
	js["doc"]=data
	js["detect_noop"]=false
	js["doc_as_upsert"]=true

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

// Get fetch document by id
func (c *ESAPIV0) Get(indexName, docType, id string) (*elastic.GetResponse, error) {

	if docType == "" {
		docType = TypeName0
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

	if global.Env().IsDebug {
		log.Trace("get response: ", string(resp.Body))
	}

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

// Delete used to delete document by id
func (c *ESAPIV0) Delete(indexName, docType, id string, refresh ...string) (*elastic.DeleteResponse, error) {
	indexName = util.UrlEncode(indexName)
	if docType == "" {
		docType = TypeName0
	}

	url := c.GetEndpoint() + "/" + indexName + "/" + docType + "/" + id

	if global.Env().IsDebug {
		log.Debug("delete doc: ", url)
	}
	if len(refresh) > 0 {
		url = url + "?refresh=" + refresh[0]
	}

	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("delete response: ", string(resp.Body))
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

// Count used to count how many docs in one index
func (c *ESAPIV0) Count(ctx context.Context, indexName string, body []byte) (*elastic.CountResponse, error) {
	indexName = util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/_count"

	if global.Env().IsDebug {
		log.Debug("doc count: ", url)
	}

	resp, err := c.Request(ctx, util.Verb_GET, url, body)

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

	js := query.ToJSONString()
	return c.SearchWithRawQueryDSL(indexName, util.UnsafeStringToBytes(js))
}

func (c *ESAPIV0) QueryDSL(ctx context.Context, indexName string, queryArgs *[]util.KV, queryDSL []byte) (*elastic.SearchResponse, error) {
	indexName = util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/_search"

	if queryArgs != nil && len(*queryArgs) > 0 {
		str := strings.Builder{}
		str.WriteString(url)
		str.WriteString("?")
		for _, v := range *queryArgs {
			str.WriteString(v.Key)
			str.WriteString("=")
			str.WriteString(v.Value)
		}
		url = str.String()
	}

	esResp := &elastic.SearchResponse{}

	if global.Env().IsDebug {
		log.Trace("search: ", url, ",", string(queryDSL))
	}

	resp, err := c.Request(ctx, util.Verb_POST, url, queryDSL)
	if resp != nil {
		esResp.StatusCode = resp.StatusCode
		esResp.RawResult = resp
		esResp.ErrorObject = err
	}

	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		log.Error("invalid response: ", url, ",", string(queryDSL), ",", string(resp.Body))
	}

	if global.Env().IsDebug {
		log.Trace("search response: ", url, ",", string(queryDSL), ",", string(resp.Body))
	}

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

func (c *ESAPIV0) SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*elastic.SearchResponse, error) {
	return c.QueryDSL(nil, indexName, nil, queryDSL)
}

func (c *ESAPIV0) IndexExists(indexName string) (bool, error) {
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

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

func (c *ESAPIV0) ClusterVersion() elastic.Version {
	return c.GetVersion()
}

func (c *ESAPIV0) GetNodesStats(nodeID, host string, level string) *elastic.NodesStats {

	log.Tracef("get stats for node: %v-%v", nodeID, host)

	suffix := ""
	if level != "" {
		suffix = fmt.Sprintf("?level=%v&include_segment_file_sizes=true", level)
	}

	url := fmt.Sprintf("%s/_nodes/_all/stats%v", c.GetEndpoint(), suffix)
	if nodeID != "" {
		//fetch shard level metrics
		url = fmt.Sprintf("%s/_nodes/%v/stats%v", c.GetActivePreferredEndpoint(host), nodeID, suffix)
	}

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	obj := &elastic.NodesStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.RawResult = resp
		obj.StatusCode = resp.StatusCode
		obj.ErrorObject = err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetIndicesStats() *elastic.IndicesStats {
	// /_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices
	url := fmt.Sprintf("%s/_stats/docs,fielddata,indexing,merge,search,segments,store,refresh,query_cache,request_cache?filter_path=indices", c.GetEndpoint())

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	obj := &elastic.IndicesStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.RawResult = resp
		obj.StatusCode = resp.StatusCode
		obj.ErrorObject = err
		return obj
	}

	return obj
}

func (c *ESAPIV0) GetClusterState() (*elastic.ClusterState, error) {

	//GET /_cluster/state/version,nodes,master_node,routing_table
	//url := fmt.Sprintf("%s/_cluster/state/version,nodes,master_node,routing_table", c.GetEndpoint())
	format := "%s/_cluster/state/version,master_node,routing_table,metadata/*"
	url := fmt.Sprintf(format, c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	obj := &elastic.ClusterState{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}
	return obj, nil
}

func (c *ESAPIV0) GetClusterStatsSpecEndpoint(ctx context.Context, node string, endPoint string) (*elastic.ClusterStats, error) {

	var url string
	if endPoint == "" {
		url = fmt.Sprintf("%s/_cluster/stats", c.GetEndpoint())
	} else {
		url = fmt.Sprintf("%s/_cluster/stats", endPoint)
	}

	//_cluster/stats
	if node != "" {
		url = fmt.Sprintf("%s/_cluster/stats/nodes/%v", c.GetEndpoint(), node)
	}

	resp, err := c.Request(ctx, util.Verb_GET, url, nil)

	obj := &elastic.ClusterStats{}
	if err != nil {
		if resp != nil {
			obj.StatusCode = resp.StatusCode
		} else {
			obj.StatusCode = 500
		}
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}

	//dirty fix for es 7.0.0
	//if c.ParseMajorVersion()==7{
	v, err := jsonparser.GetInt(resp.Body, "indices", "segments", "max_unsafe_auto_id_timestamp")
	if err != nil || v < -1 {
		d, err := jsonparser.Set(resp.Body, []byte("-1"), "indices", "segments", "max_unsafe_auto_id_timestamp")
		if err == nil {
			resp.Body = d
		}
	}
	//}

	err = json.Unmarshal(resp.Body, obj)
	if err != nil {
		obj.StatusCode = resp.StatusCode
		obj.RawResult = resp
		obj.ErrorObject = err
		return obj, err
	}

	return obj, nil
}

func (c *ESAPIV0) GetClusterStats(ctx context.Context, node string) (*elastic.ClusterStats, error) {
	return c.GetClusterStatsSpecEndpoint(ctx, node, "")
}

// ClusterHealthSpecEndpoint
// @param url eg: http://192.168.3.22:9200
func (c *ESAPIV0) ClusterHealthSpecEndpoint(ctx context.Context, endPoint string, level string) (*elastic.ClusterHealth, error) {

	var url string
	if endPoint == "" {
		url = fmt.Sprintf("%s/_cluster/health?timeout=10s", c.GetEndpoint())
	} else {
		url = fmt.Sprintf("%s/_cluster/health?timeout=10s", endPoint)
	}
	if level != "" {
		url += "&level=" + level
	}
	log.Tracef("get cluster health, url: %s", url)
	health := &elastic.ClusterHealth{}
	resp, err := c.Request(ctx, util.Verb_GET, url, nil)

	if resp != nil {
		health.StatusCode = resp.StatusCode
		health.RawResult = resp
	} else {
		health.StatusCode = 500
	}

	if err != nil {
		log.Error(err, string(resp.Body))
		health.ErrorObject = err
		return health, err
	}

	if resp.StatusCode == 200 {
		err = json.Unmarshal(resp.Body, health)
		if err != nil {
			health.ErrorObject = err
			health.RawResult = resp
			health.StatusCode = resp.StatusCode
			return health, err
		}
	}

	return health, err
}

func (c *ESAPIV0) ClusterHealth(ctx context.Context) (*elastic.ClusterHealth, error) {
	return c.ClusterHealthSpecEndpoint(ctx, "", "")
}

func (c *ESAPIV0) GetNodes() (*map[string]elastic.NodesInfo, error) {

	url := fmt.Sprintf("%s/_nodes", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	node := elastic.NodesResponse{}

	err = util.FromJSONBytes(resp.Body, &node)

	if err != nil {
		return nil, err
	}
	return &node.Nodes, nil
}

func (c *ESAPIV0) GetNodeInfo(nodeID string) (*elastic.NodesInfo, error) {

	url := fmt.Sprintf("%s/_nodes/%v", c.GetEndpoint(), nodeID)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}
	node := elastic.NodesResponse{}
	err = util.FromJSONBytes(resp.Body, &node)
	if err != nil {
		return nil, err
	}
	nodeInfo, ok := node.Nodes[nodeID]
	if ok{
		return &nodeInfo, nil
	}
	return nil, errors.Errorf("node info[%v] not found",nodeID)
}

func (c *ESAPIV0) GetIndices(pattern string) (*map[string]elastic.IndexInfo, error) {
	format := "%s/_cat/indices%s?h=health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size,segments.count&format=json"
	if pattern != "" {
		pattern = "/" + pattern
	}
	url := fmt.Sprintf(format, c.GetEndpoint(), pattern)

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

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

//index:shardID -> nodesInfo
func (c *ESAPIV0) GetPrimaryShards() (*map[string]map[int]elastic.ShardInfo, error) {
	data := []elastic.CatShardResponse{}

	url := fmt.Sprintf("%s/_cat/shards?v&h=index,shard,prirep,state,unassigned.reason,docs,store,id,node,ip&format=json", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
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
		id, err := util.ToInt(v.ShardID)
		if err != nil {
			log.Error("invalid shard id, it should be number,", string(resp.Body))
			return nil, err
		}
		indexMap[id] = info
		infos[v.Index] = indexMap

		// infos[fmt.Sprintf("%v:%v", info.Index, info.ShardID)] = info
	}
	return &infos, nil
}

func (c *ESAPIV0) CatShardsSpecEndpoint(endPoint string) ([]elastic.CatShardResponse, error) {
	data := []elastic.CatShardResponse{}
	var url string
	if endPoint == "" {
		url = fmt.Sprintf("%s/_cat/shards?v&h=index,shard,prirep,state,unassigned.reason,docs,store,id,node,ip&format=json&bytes=b", c.GetEndpoint())
	} else {
		url = fmt.Sprintf("%s/_cat/shards?v&h=index,shard,prirep,state,unassigned.reason,docs,store,id,node,ip&format=json&bytes=b", endPoint)
	}
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
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
	for i, catRes := range data {
		data[i].StoreInBytes, _ = strconv.ParseInt(catRes.Store, 10, 64)
		data[i].Store = util.FormatBytes(float64(data[i].StoreInBytes), 2)
	}
	return data, nil
}

func (c *ESAPIV0) CatShards() ([]elastic.CatShardResponse, error) {
	return c.CatShardsSpecEndpoint("")
}

func (c *ESAPIV0) Bulk(data []byte) (*util.Result, error) {
	if data == nil || len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	url := fmt.Sprintf("%s/_bulk?filter_path=items.*.error", c.GetEndpoint())
	result, err := c.Request(nil, util.Verb_POST, url, data)

	if global.Env().IsDebug {
		log.Trace(string(result.Body), err)
	}

	if err != nil {
		return result, err
	}
	if v := string(result.Body); v != "{}" {
		log.Warn(v)
	}

	containError := util.LimitedBytesSearch(result.Body, []byte("\"errors\":true"), 64)
	if containError {
		return result, errors.New("bulk partial failure")
	}

	return result, nil
}

func (c *ESAPIV0) GetIndexSettings(indexNames string) (*util.MapStr, error) {
	indexNames = util.UrlEncode(indexNames)

	// get all settings
	allSettings := &util.MapStr{}

	url := fmt.Sprintf("%s/%s/_settings?include_defaults", c.GetEndpoint(), indexNames)

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

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

func (c *ESAPIV0) GetMapping(copyAllIndexes bool, indexNames string) (string, int, *util.MapStr, error) {
	indexNames = util.UrlEncode(indexNames)

	url := fmt.Sprintf("%s/%s/_mapping", c.GetEndpoint(), indexNames)

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil {
		return "", 0, nil, err
	}

	idxs := util.MapStr{}
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
	name = util.UrlEncode(name)

	//cleanSettings(settings)
	url := fmt.Sprintf("%s/%s/_settings", s.GetEndpoint(), name)

	//if _, ok := settings["settings"].(map[string]interface{})["index"]; ok {
	//	if set, ok := settings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"]; ok {
	//		staticIndexSettings := getEmptyIndexSettings()
	//		staticIndexSettings["settings"].(map[string]interface{})["index"].(map[string]interface{})["analysis"] = set
	//
	//		_, err := s.Request("POST", fmt.Sprintf("%s/%s/_close", s.GetEndpoint(), name), nil)
	//
	//		//TODO error handle
	//
	//		body := bytes.Buffer{}
	//		enc := json.NewEncoder(&body)
	//		enc.Encode(staticIndexSettings)
	//		_, err = s.Request("PUT", url, body.Bytes())
	//		if err != nil {
	//			panic(err)
	//		}
	//
	//		delete(settings["settings"].(map[string]interface{})["index"].(map[string]interface{}), "analysis")
	//
	//		_, err = s.Request("POST", fmt.Sprintf("%s/%s/_open", s.GetEndpoint(), name), nil)
	//
	//		//TODO error handle
	//	}
	//}

	body := bytes.Buffer{}
	enc := json.NewEncoder(&body)
	enc.Encode(settings)
	result, err := s.Request(nil, util.Verb_PUT, url, body.Bytes())
	errReason, _ := jsonparser.GetString(result.Body, "error", "reason")
	if errReason != "" {
		return fmt.Errorf(errReason)
	}

	return err
}

func (s *ESAPIV0) UpdateMapping(indexName string, docType string, mappings []byte) ([]byte, error) {
	//es6.6 start support index_prefixes, index_phrases
	mappings = bytes.Replace(mappings, []byte(`"index_prefixes":{},"index_phrases":"true",`), nil, -1)

	indexName = util.UrlEncode(indexName)
	if docType == "" {
		docType = TypeName0
	}

	url := fmt.Sprintf("%s/%s/%s/_mapping", s.GetEndpoint(), indexName, docType)

	resp, err := s.Request(nil, util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}

	return resp.Body, nil
}

func (c *ESAPIV0) DeleteIndex(indexName string) (err error) {
	if global.Env().IsDebug {
		log.Trace("start delete index: ", indexName)
	}
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), indexName)

	c.Request(nil, util.Verb_DELETE, url, nil)

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
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), indexName)

	result, err := c.Request(nil, util.Verb_PUT, url, body.Bytes())

	if err != nil {
		return err
	}
	if result.StatusCode != http.StatusOK {
		return fmt.Errorf("code:%v,response:%v", result.StatusCode, string(result.Body))
	}

	return nil
}

func (s *ESAPIV0) Refresh(name string) (err error) {
	name = util.UrlEncode(name)

	url := fmt.Sprintf("%s/%s/_refresh", s.GetEndpoint(), name)

	_, err = s.Request(nil, util.Verb_POST, url, nil)

	return err
}

func (s *ESAPIV0) NewScroll(indexNames string, scrollTime string, docBufferCount int, query *elastic.SearchRequest, slicedId, maxSlicedCount int) ([]byte, error) {
	indexNames = util.UrlEncode(indexNames)

	// curl -XGET 'http://es-0.9:9200/_search?search_type=scan&scroll=10m&size=50'
	url := fmt.Sprintf("%s/%s/_search?search_type=scan&scroll=%s&size=%d", s.GetEndpoint(), indexNames, scrollTime, docBufferCount)

	var jsonBody string
	if query != nil {
		jsonBody = query.ToJSONString()
	}

	resp, err := s.Request(nil, util.Verb_POST, url, util.UnsafeStringToBytes(jsonBody))
	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("new scroll,", url, ",", jsonBody)
	}

	if resp.StatusCode != 200 {
		log.Error("response status:", resp.StatusCode)
		return nil, errors.New(string(resp.Body))
	}

	return resp.Body, err
}

func (s *ESAPIV0) NextScroll(ctx *elastic.APIContext, scrollTime string, scrollId string) ([]byte, error) {

	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", s.GetEndpoint(), scrollTime, scrollId)

	resp, err := adapter.RequestTimeout(ctx, util.Verb_GET, url, nil, s.metadata, time.Duration(s.metadata.Config.RequestTimeout)*time.Second)
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

func (s *ESAPIV0) ClearScroll(scrollId string) error {
	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body := []byte(scrollId)

	resp, err := s.Request(context.Background(), util.Verb_DELETE, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(string(resp.Body))
	}
	return nil
}

func (c *ESAPIV0) TemplateExists(templateName string) (bool, error) {
	url := fmt.Sprintf("%s/_template/%s", c.GetEndpoint(), templateName)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil || resp != nil && resp.StatusCode == 404 {
		return false, err
	} else {
		return true, nil
	}

	return false, nil
}

func (c *ESAPIV0) PutTemplate(templateName string, template []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/_template/%s", c.GetEndpoint(), templateName)
	resp, err := c.Request(nil, util.Verb_PUT, url, template)

	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *ESAPIV0) GetTemplate(templateName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/_template", c.GetEndpoint())
	if templateName != "" {
		url = fmt.Sprintf("%s/%s", url, templateName)
	}
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(string(resp.Body))
	}

	data := map[string]interface{}{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
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
	url := fmt.Sprintf("%s/_reindex?wait_for_completion=false", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_POST, url, body)
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

func (c *ESAPIV0) GetIndexStats(indexName string) (*util.MapStr, error) {
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_stats", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	var response = &util.MapStr{}
	err = json.Unmarshal(resp.Body, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ESAPIV0) GetStats() (*elastic.Stats, error) {
	format := "%s/_stats?ignore_unavailable=true"
	url := fmt.Sprintf(format, c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
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

// "dict" : {
// "aliases" : {
// "dictalias1" : {
// "is_write_index" : true
// },
// "dictalias2" : {
// "is_write_index" : true
// }
// }
// },
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

	url := fmt.Sprintf("%s/_alias", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil || resp.StatusCode != 200 {
		return nil, err
	}

	data := map[string]AliasesResponse{}
	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		log.Error(url, string(resp.Body), err)
		c.GetMetadata().ReportFailure(err)
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

	url := fmt.Sprintf("%s/_alias", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

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

	url := fmt.Sprintf("%s/_alias", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	if err != nil || resp.StatusCode != 200 {
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
		idxItem := elastic.AAIR_Indices{
			Name:       index,
			Attributes: []string{"open"},
		}
		for alias, _ := range v.Aliases {
			idxItem.Aliases = append(idxItem.Aliases, alias)
			info, ok := aliasInfo[alias]
			if !ok {
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
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_forcemerge?max_num_segments=%v", c.GetEndpoint(), indexName, maxCount)
	_, err := c.Request(nil, util.Verb_POST, url, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *ESAPIV0) DeleteByQuery(indexName string, body []byte) (*elastic.DeleteByQueryResponse, error) {
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_delete_by_query", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_POST, url, body)
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

func (c *ESAPIV0) UpdateByQuery(indexName string, body []byte) (*elastic.UpdateByQueryResponse, error) {
	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_update_by_query", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return nil, err
	}
	var upResponse = &elastic.UpdateByQueryResponse{}
	err = json.Unmarshal(resp.Body, upResponse)
	if err != nil {
		return nil, err
	}
	return upResponse, nil
}

func (c *ESAPIV0) SetSearchTemplate(templateID string, body []byte) error {
	url := fmt.Sprintf("%s/_search/template/%s", c.GetEndpoint(), templateID)
	_, err := c.Request(nil, util.Verb_POST, url, body)
	return err
}

func (c *ESAPIV0) DeleteSearchTemplate(templateID string) error {
	url := fmt.Sprintf("%s/_search/template/%s", c.GetEndpoint(), templateID)
	_, err := c.Request(nil, util.Verb_DELETE, url, nil)
	return err
}

func (c *ESAPIV0) RenderTemplate(body map[string]interface{}) ([]byte, error) {
	if source, ok := body["source"]; ok { //es 5.6
		body["inline"] = source
		delete(body, "source")
	}
	url := fmt.Sprintf("%s/_render/template", c.GetEndpoint())
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(nil, util.Verb_POST, url, bytesBody)
	return res.Body, err
}

func (c *ESAPIV0) SearchTemplate(body map[string]interface{}) ([]byte, error) {
	if source, ok := body["source"]; ok { //es 5.6
		body["inline"] = source
		delete(body, "source")
	}

	url := fmt.Sprintf("%s/_search/template", c.GetEndpoint())
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(nil, util.Verb_POST, url, bytesBody)
	return res.Body, err
}

func (c *ESAPIV0) Alias(body []byte) error {
	url := fmt.Sprintf("%s/_aliases", c.GetEndpoint())
	res, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf(string(res.Body))
	}
	return nil
}

func (c *ESAPIV0) FieldCaps(target string) ([]byte, error) {
	target = util.UrlEncode(target)

	url := fmt.Sprintf("%s/%s/_mappings", c.GetEndpoint(), target)
	res, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	mappingsRes := map[string]interface{}{}
	err = json.Unmarshal(res.Body, &mappingsRes)

	if err != nil {
		return nil, err
	}

	var (
		indices []string
		fields  = map[string]interface{}{}
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
		"fields":  fields,
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return resultBytes, nil
}

func walkProperties(fields, properties map[string]interface{}, prefix string) {
	var dotFieldName string
	for fieldName, fieldInterface := range properties {
		if prefix != "" {
			dotFieldName = fmt.Sprintf("%s.%s", prefix, fieldName)
		} else {
			dotFieldName = fieldName
		}
		if field, ok := fieldInterface.(map[string]interface{}); ok {
			if esType, ok := field["type"].(string); ok {
				fields[dotFieldName] = map[string]interface{}{
					esType: map[string]interface{}{
						"type":         field["type"],
						"searchable":   true,
						"aggregatable": true,
					},
				}
			} else if propertiesInterface, ok := field["properties"]; ok {
				if subProperties, ok := propertiesInterface.(map[string]interface{}); ok {
					walkProperties(fields, subProperties, dotFieldName)
				}
			}
		}
	}
}

func (c *ESAPIV0) Close(name string) ([]byte, error) {
	name = util.UrlEncode(name)

	url := fmt.Sprintf("%s/%s/_close", c.GetEndpoint(), name)
	closeRes, err := c.Request(nil, util.Verb_POST, url, nil)
	return closeRes.Body, err
}

func (c *ESAPIV0) Open(name string) ([]byte, error) {
	name = util.UrlEncode(name)

	url := fmt.Sprintf("%s/%s/_open", c.GetEndpoint(), name)
	openRes, err := c.Request(nil, util.Verb_POST, url, nil)
	return openRes.Body, err
}

func (c *ESAPIV0) GetIndexRoutingTable(index string) (map[string][]elastic.IndexShardRouting, error) {
	//fetch routing table in realtime
	url := fmt.Sprintf("%s/_cluster/state/routing_table/%s", c.GetEndpoint(), index)
	res, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	resObj := &elastic.ClusterState{}
	util.MustFromJSONBytes(res.Body, resObj)
	if v, ok := resObj.RoutingTable.Indices[index]; ok {
		return v.Shards, nil
	}
	return nil, errors.Errorf("routing table for index [%v] was not found", index)
}

func (c *ESAPIV0) CatNodes(colStr string) ([]elastic.CatNodeResponse, error) {
	url := fmt.Sprintf("%s/_cat/nodes?format=json&full_id", c.GetEndpoint())
	if colStr != "" {
		url = fmt.Sprintf("%s/_cat/nodes?format=json&h=%s&full_id", c.GetEndpoint(), colStr)
	}
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := []elastic.CatNodeResponse{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
}

func (c *ESAPIV0) GetClusterSettings(values url.Values) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/_cluster/settings", c.GetEndpoint())
	if len(values) > 0 {
		url += "?"+values.Encode()
	}
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := map[string]interface{}{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
}


func (c *ESAPIV0) UpdateClusterSettings(body []byte) error {
	url := fmt.Sprintf("%s/_cluster/settings", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_PUT, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}

	return nil
}

func (c *ESAPIV0) GetRemoteInfo()([]byte, error){
	url := fmt.Sprintf("%s/_remote/info", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}

	return resp.Body, nil
}

func (c *ESAPIV0) GetIndex(indexName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := map[string]interface{}{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
}

func (c *ESAPIV0) Exists(target string) (bool, error) {
	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), target)
	resp, err := c.Request(nil, util.Verb_HEAD, url, nil)
	if err != nil {
		return false, err
	}
	return resp.StatusCode == 200, nil
}

func (c *ESAPIV0) GetILMPolicy(target string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unsupport feature")
}

func (c *ESAPIV0) PutILMPolicy(target string, policyConfig []byte) error {
	return fmt.Errorf("unsupport feature")
}

func (c *ESAPIV0) DeleteILMPolicy(target string) error {
	return fmt.Errorf("unsupport feature")
}

func (c *ESAPIV0) StartReplication(followIndex string, body []byte) error{
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) StopReplication(indexName string, body []byte) error{
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) PauseReplication(followIndex string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) ResumeReplication(followIndex string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetReplicationStatus(followIndex string) ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetReplicationFollowerStats(followIndex string) ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) CreateAutoFollowReplication(autoFollowPatternName string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) DeleteAutoFollowReplication(autoFollowPatternName string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetAutoFollowStats(autoFollowPatternName string)([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}

func (c *ESAPIV0) GetUser(username string) ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetUsers() ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) DeleteUser(username string) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) PutUser(username string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetRole(roleName string) ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetRoles() ([]byte, error){
	return nil, fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) DeleteRole(roleName string) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) PutRole(roleName string, body []byte) error {
	return fmt.Errorf("unsupport feature")
}
func (c *ESAPIV0) GetPrivileges() ([]byte, error) {
	return nil, fmt.Errorf("unsupport feature")
}

func (c *ESAPIV0) Flush(indexName string) ([]byte, error) {
	url := "/_flush/synced"
	if indexName != "" {
		url = fmt.Sprintf("/%s/_flush/synced", indexName)
	}
	url = c.GetEndpoint() + url
	resp, err := c.Request(nil, util.Verb_POST, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}


func (c *ESAPIV0)  	ScriptExists(scriptName string)(bool,error){
	panic("not implemented")
}

func (c *ESAPIV0) PutScript(scriptName string, script []byte)([]byte,error){
	panic("not implemented")
}

func (c *ESAPIV0)SearchByTemplate(indexName,scriptName string,params map[string]interface{}) (*elastic.SearchResponse, error)  {
	panic("not implemented")
}

func (c *ESAPIV0) ClusterAllocationExplain(ctx context.Context, body []byte, params url.Values)([]byte,error){
	url := fmt.Sprintf("%s/_cluster/allocation/explain", c.GetEndpoint())
	if len(params) > 0 {
		url = fmt.Sprintf("%s?%s", url, params.Encode())
	}
	method := util.Verb_GET
	if len(body) > 0 {
		method = util.Verb_POST
	}
	resp, err := c.Request(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
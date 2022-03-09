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
	"context"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
)

type API interface {
	ScrollAPI
	MappingAPI
	TemplateAPI

	InitDefaultTemplate(templateName, indexPrefix string)

	GetMajorVersion() int

	ClusterHealth() (*ClusterHealth,error)

	GetClusterState() (*ClusterState,error)

	GetClusterStats(node string) (*ClusterStats,error)

	GetNodesStats(nodeID,host string) *NodesStats

	GetIndicesStats() *IndicesStats

	ClusterVersion() string

	CreateIndex(name string, settings map[string]interface{}) error

	Index(indexName, docType string, id interface{}, data interface{}) (*InsertResponse, error)

	Bulk(data []byte)

	Get(indexName, docType, id string) (*GetResponse, error)
	Delete(indexName, docType, id string, refresh ...string) (*DeleteResponse, error)
	Count(indexName string) (*CountResponse, error)
	Search(indexName string, query *SearchRequest) (*SearchResponse, error)

	QueryDSL(indexName string,queryArgs *[]util.KV, queryDSL []byte) (*SearchResponse, error)

	SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*SearchResponse, error)

	GetIndexSettings(indexNames string) (*Indexes, error)
	UpdateIndexSettings(indexName string, settings map[string]interface{}) error

	IndexExists(indexName string) (bool, error)

	DeleteIndex(name string) error

	Refresh(name string) (err error)

	GetNodes() (*map[string]NodesInfo, error)

	GetNodeInfo(nodeID string) (*NodesInfo, error)


	GetIndices(pattern string) (*map[string]IndexInfo, error)

	GetPrimaryShards() (*map[string]map[int]ShardInfo, error)
	GetAliases() (*map[string]AliasInfo, error)
	GetAliasesDetail() (*map[string]AliasDetailInfo, error)
	GetAliasesAndIndices() (*AliasAndIndicesResponse, error)

	SearchTasksByIds(ids []string) (*SearchResponse, error)
	Reindex(body []byte) (*ReindexResponse, error)
	DeleteByQuery(indexName string, body []byte) (*DeleteByQueryResponse, error)

	GetIndexStats(indexName string) (*IndexStats, error)
	GetStats() (*Stats, error)
	Forcemerge(indexName string, maxCount int) error
	SetSearchTemplate(templateID string, body []byte) error
	DeleteSearchTemplate(templateID string) error
	RenderTemplate(body map[string]interface{}) ([]byte, error)
	SearchTemplate(body map[string]interface{}) ([]byte, error)
	Alias(body []byte) error
	FieldCaps(target string) ([]byte, error)
	CatShards() ([]CatShardResponse, error)


	GetIndexRoutingTable(index string) (map[string][]IndexShardRouting,error)

}

type TemplateAPI interface {
	TemplateExists(templateName string) (bool, error)
	PutTemplate(templateName string, template []byte) ([]byte, error)
}

type MappingAPI interface {
	GetMapping(copyAllIndexes bool, indexNames string) (string, int, *Indexes, error)
	UpdateMapping(indexName string, mappings []byte) ([]byte, error)
}

type ScrollAPI interface {
	NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, fields string, sortField, sortType string) ([]byte, error)
	NextScroll(ctx *APIContext,scrollTime string, scrollId string) ([]byte, error)
}

type APIContext struct {
	context.Context `json:"-"`
	Client *fasthttp.Client
	Request *fasthttp.Request
	Response *fasthttp.Response
}


type ScrollResponseAPI interface {
	GetScrollId() string
	SetScrollId(id string)
	GetHitsTotal() int64
	GetShardResponse() ShardResponse
	GetDocs() []IndexDocument
}

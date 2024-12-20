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

package elastic

import (
	"context"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"net/url"
)

type API interface {
	ScrollAPI
	MappingAPI
	TemplateAPI
	ReplicationAPI
	SecurityAPI
	ScriptAPI

	InitDefaultTemplate(templateName, indexPrefix string)

	GetMajorVersion() int

	ClusterHealth(ctx context.Context) (*ClusterHealth, error)
	ClusterHealthSpecEndpoint(ctx context.Context, endPoint string, level string) (*ClusterHealth, error)

	GetClusterState() (*ClusterState, error)

	GetClusterStats(ctx context.Context, node string) (*ClusterStats, error)
	GetClusterStatsSpecEndpoint(ctx context.Context, node string, endPoint string) (*ClusterStats, error)

	GetNodesStats(nodeID, host string, level string) *NodesStats

	GetIndicesStats() *IndicesStats

	GetVersion() Version

	CreateIndex(name string, settings map[string]interface{}) error

	Index(indexName, docType string, id interface{}, data interface{}, refresh string) (*InsertResponse, error)

	Update(indexName, docType string, id interface{}, data interface{}, refresh string) (*InsertResponse, error)

	Bulk(data []byte) (*util.Result, error)

	Get(indexName, docType, id string) (*GetResponse, error)
	Delete(indexName, docType, id string, refresh ...string) (*DeleteResponse, error)
	Count(ctx context.Context, indexName string, body []byte) (*CountResponse, error)
	Search(indexName string, query *SearchRequest) (*SearchResponse, error)

	QueryDSL(ctx context.Context, indexName string, queryArgs *[]util.KV, queryDSL []byte) (*SearchResponse, error)

	SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*SearchResponse, error)

	GetIndexSettings(indexNames string) (*util.MapStr, error)
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
	UpdateByQuery(indexName string, body []byte) (*UpdateByQueryResponse, error)

	GetIndexStats(indexName string) (*util.MapStr, error)
	GetStats() (*Stats, error)
	Forcemerge(indexName string, maxCount int) error
	SetSearchTemplate(templateID string, body []byte) error
	DeleteSearchTemplate(templateID string) error
	RenderTemplate(body map[string]interface{}) ([]byte, error)
	SearchTemplate(body map[string]interface{}) ([]byte, error)
	Alias(body []byte) error
	FieldCaps(target string) ([]byte, error)
	CatShards() ([]CatShardResponse, error)
	CatShardsSpecEndpoint(endPoint string) ([]CatShardResponse, error)
	CatNodes(colStr string) ([]CatNodeResponse, error)

	GetIndexRoutingTable(index string) (map[string][]IndexShardRouting, error)
	GetClusterSettings(values url.Values) (map[string]interface{}, error)
	UpdateClusterSettings(body []byte) error
	GetIndex(indexName string) (map[string]interface{}, error)
	Exists(target string) (bool, error)
	GetILMPolicy(target string) (map[string]interface{}, error)
	PutILMPolicy(target string, policyConfig []byte) error
	DeleteILMPolicy(target string) error
	GetRemoteInfo()([]byte, error)
	Flush(indexName string) ([]byte, error)
	ClusterAllocationExplain(ctx context.Context, body []byte, params url.Values)([]byte,error)
}

type TemplateAPI interface {
	TemplateExists(scriptName string) (bool, error)
	PutTemplate(scriptName string, template []byte) ([]byte, error)
	GetTemplate(scriptName string) (map[string]interface{}, error)
	SearchByTemplate(indexName,scriptName string,params map[string]interface{}) (*SearchResponse, error)
}

type MappingAPI interface {
	GetMapping(copyAllIndexes bool, indexNames string) (string, int, *util.MapStr, error)
	UpdateMapping(indexName string, docType string, mappings []byte) ([]byte, error)
}

type ScrollAPI interface {
	NewScroll(indexNames string, scrollTime string, docBufferCount int, query *SearchRequest, slicedId, maxSlicedCount int) ([]byte, error)
	NextScroll(ctx *APIContext, scrollTime string, scrollId string) ([]byte, error)
	ClearScroll(scrollId string) error
}

type ScriptAPI interface {
	ScriptExists(scriptName string)(bool,error)
	PutScript(scriptName string, script []byte)([]byte,error)
	SearchByTemplate(indexName,scriptName string,params map[string]interface{}) (*SearchResponse, error)
	//GetScript(scriptName string)([]byte,error)
	//DeleteScript(scriptName string)([]byte,error)
}

type ReplicationAPI interface {
	StartReplication(followIndex string, body []byte) error
	StopReplication(indexName string, body []byte) error
	PauseReplication(followIndex string, body []byte) error
	ResumeReplication(followIndex string, body []byte) error
	GetReplicationStatus(followIndex string) ([]byte, error)
	GetReplicationFollowerStats(followIndex string) ([]byte, error)
	CreateAutoFollowReplication(autoFollowPatternName string, body []byte) error
	GetAutoFollowStats(autoFollowPatternName string)([]byte, error)
	DeleteAutoFollowReplication(autoFollowPatternName string, body []byte) error
}

type SecurityAPI interface {
	GetUser(username string) ([]byte, error)
	GetUsers() ([]byte, error)
	DeleteUser(username string) error
	PutUser(username string, body []byte) error
	GetRole(roleName string) ([]byte, error)
	GetRoles() ([]byte, error)
	DeleteRole(roleName string) error
	PutRole(roleName string, body []byte) error
	GetPrivileges() ([]byte, error)
}

type APIContext struct {
	context.Context `json:"-"`
	Client          *fasthttp.Client
	Request         *fasthttp.Request
	Response        *fasthttp.Response
}

type ScrollResponseAPI interface {
	GetScrollId() string
	SetScrollId(id string)
	GetHitsTotal() int64
	GetShardResponse() ShardResponse
	GetDocs() []IndexDocument
}

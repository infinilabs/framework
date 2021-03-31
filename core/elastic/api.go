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
	"bytes"
)

type API interface {
	ScrollAPI
	MappingAPI
	TemplateAPI

	InitDefaultTemplate(templateName,indexPrefix string)

	GetMajorVersion() int

	ClusterHealth() *ClusterHealth

	GetClusterStats() *ClusterStats
	GetNodesStats() *NodesStats
	GetIndicesStats() *IndicesStats

	ClusterVersion() string

	CreateIndex(name string, settings map[string]interface{}) error

	Index(indexName, docType string, id interface{}, data interface{}) (*InsertResponse, error)

	Bulk(data *bytes.Buffer)

	Get(indexName, docType, id string) (*GetResponse, error)
	Delete(indexName, docType, id string) (*DeleteResponse, error)
	Count(indexName string) (*CountResponse, error)
	Search(indexName string, query *SearchRequest) (*SearchResponse, error)
	SearchWithRawQueryDSL(indexName string, queryDSL []byte) (*SearchResponse, error)

	GetIndexSettings(indexNames string) (*Indexes, error)
	UpdateIndexSettings(indexName string, settings map[string]interface{}) error

	IndexExists(indexName string) (bool, error)
	DeleteIndex(name string) error

	Refresh(name string) (err error)

	GetNodes() (*map[string]NodesInfo, error)

	GetIndices(pattern string) (*map[string]IndexInfo, error)

	GetPrimaryShards() (*map[string]ShardInfo, error)
	GetAliases() (*map[string]AliasInfo,error)


	SearchTasksByIds(ids []string) (*SearchResponse, error)
	Reindex(body []byte) (*ReindexResponse, error)
	DeleteByQuery(indexName string, body []byte) (*DeleteByQueryResponse, error)


	GetIndexStats(indexName string)(*IndexStats,error)
	GetStats()(*Stats,error)
	Forcemerge(indexName string,maxCount int)(error)

}

type Stats struct {
	All struct{
		Primary map[string]interface{}`json:"primaries"`
		Total map[string]interface{}`json:"total"`

	}`json:"_all"`

	Indices map[string]interface{}`json:"indices"`
}

type IndexStats struct {
	All struct{
		Primary struct{
			Store struct{
				SizeInBytes int `json:"size_in_bytes"`
			}`json:"store"`

			Segments struct{
				Count int `json:"count"`
			}`json:"segments"`

			Merges struct{
				Current int `json:"current"`
			}`json:"merges"`

		}`json:"primaries"`

	}`json:"_all"`
}

type NodesInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Http    struct {
		PublishAddress          string `json:"publish_address,omitempty"`
		MaxContentLengthInBytes int    `json:"max_content_length_in_bytes,omitempty"`
	} `json:"http,omitempty"`

	TotalIndexingBuffer int                    `json:"total_indexing_buffer,omitempty"`
	Attributes          map[string]interface{} `json:"attributes,omitempty"`
	Roles               []string               `json:"roles,omitempty"`
	//TODO return more nodes level settings, for later check and usage
}

type IndexInfo struct {
	ID        string `json:"id,omitempty"`
	Index        string `json:"index,omitempty"`
	Status       string `json:"status,omitempty"`
	Health       string `json:"health,omitempty"`
	Shards       int    `json:"shards,omitempty"`
	Replicas     int    `json:"replicas,omitempty"`
	DocsCount    int64  `json:"docs_count,omitempty"`
	DocsDeleted  int64  `json:"docs_deleted,omitempty"`
	SegmentsCount  int64  `json:"segments_count,omitempty"`
	StoreSize    string `json:"store_size,omitempty"`
	PriStoreSize string `json:"pri_store_size,omitempty"`
}

type ShardInfo struct {
	Index            string `json:"index,omitempty"`
	ShardID          string `json:"shard_id,omitempty"`
	Primary          bool   `json:"primary,omitempty"`
	State            string `json:"state,omitempty"`
	UnassignedReason string `json:"unassigned_reason,omitempty"`
	Docs             int64  `json:"docs_count,omitempty"`
	Store            string `json:"store_size,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
	NodeName         string `json:"node_name,omitempty"`
	NodeIP           string `json:"node_ip,omitempty"`
}
type AliasInfo struct {
	Alias            string `json:"alias,omitempty"`
	Index          	[]string `json:"index,omitempty"`
	WriteIndex        string   `json:"write_index,omitempty"`
}

type NodesResponse struct {
	ClusterName string `json:"cluster_name,omitempty"`
	Nodes       map[string]NodesInfo
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
	NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, fields string) (interface{}, error)
	NextScroll(scrollTime string, scrollId string) (interface{}, error)
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
	Health       string `json:"health,omitempty"`
	Status       string `json:"status,omitempty"`
	Index        string `json:"index,omitempty"`
	Uuid         string `json:"uuid,omitempty"`
	Pri          string `json:"pri,omitempty"`
	Rep          string `json:"rep,omitempty"`
	DocsCount    string `json:"docs.count,omitempty"`
	DocsDeleted  string `json:"docs.deleted,omitempty"`
	StoreSize    string `json:"store.size,omitempty"`
	PriStoreSize string `json:"pri.store.size,omitempty"`
	SegmentCount string `json:"segments.count,omitempty"`

	//TotalMemory string `json:"memory.total,omitempty"`
	//FieldDataMemory string `json:"fielddata.memory_size,omitempty"`
	//FieldDataEvictions string `json:"fielddata.evictions,omitempty"`
	//QueryCacheMemory string `json:"query_cache.memory_size,omitempty"`
	//QueryCacheEvictions string `json:"query_cache.evictions,omitempty"`
	//RequestCacheMemory string `json:"request_cache.memory_size,omitempty"`
	//RequestCacheEvictions string `json:"request_cache.evictions,omitempty"`
	//RequestCacheHitCount string `json:"request_cache.hit_count,omitempty"`
	//RequestCacheMissCount string `json:"request_cache.miss_count,omitempty"`
	//SegmentMemory string `json:"segments.memory,omitempty"`
	//SegmentWriterMemory string `json:"segments.index_writer_memory,omitempty"`
	//SegmentVersionMapMemory string `json:"segments.version_map_memory,omitempty"`
	//SegmentFixedBitsetMemory string `json:"segments.fixed_bitset_memory,omitempty"`
}

type ReindexResponse struct {
	Task string `json:"task"`
}

type DeleteByQueryResponse struct {
	Deleted int `json:"deleted"`
	Total   int `json:"total"`
}

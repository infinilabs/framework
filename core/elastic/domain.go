package elastic

import (
	"sync"
	"time"
)

type Stats struct {
	All struct {
		Primary map[string]interface{} `json:"primaries"`
		Total   map[string]interface{} `json:"total"`
	} `json:"_all"`

	Indices map[string]interface{} `json:"indices"`
}

type IndexStats struct {
	All struct {
		Primary struct {
			Store struct {
				SizeInBytes int64 `json:"size_in_bytes"`
			} `json:"store"`

			Segments struct {
				Count int `json:"count"`
			} `json:"segments"`

			Merges struct {
				Current int `json:"current"`
			} `json:"merges"`
		} `json:"primaries"`
	} `json:"_all"`
}

type NodesInfo struct {
	Name    string `json:"name,omitempty,nocopy"`
	Version string `json:"version,omitempty,nocopy"`
	Http    struct {
		PublishAddress          string `json:"publish_address,omitempty"`
		MaxContentLengthInBytes int64  `json:"max_content_length_in_bytes,omitempty"`
	} `json:"http,omitempty"`

	TotalIndexingBuffer int64                  `json:"total_indexing_buffer,omitempty"`
	Attributes          map[string]interface{} `json:"attributes,omitempty"`
	Roles               []string               `json:"roles,omitempty"`
	//TODO return more nodes level settings, for later check and usage
}

type IndexInfo struct {
	ID            string `json:"id,omitempty"`
	Index         string `json:"index,omitempty"`
	Status        string `json:"status,omitempty"`
	Health        string `json:"health,omitempty"`
	Shards        int    `json:"shards,omitempty"`
	Replicas      int    `json:"replicas,omitempty"`
	DocsCount     int64  `json:"docs_count,omitempty"`
	DocsDeleted   int64  `json:"docs_deleted,omitempty"`
	SegmentsCount int64  `json:"segments_count,omitempty"`
	StoreSize     string `json:"store_size,omitempty"`
	PriStoreSize  string `json:"pri_store_size,omitempty"`
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
	Alias      string   `json:"alias,omitempty"`
	Index      []string `json:"index,omitempty"`
	WriteIndex string   `json:"write_index,omitempty"`
}

type AliasDetailInfo struct {
	Alias      string       `json:"alias,omitempty"`
	Indexes    []AliasIndex `json:"indexes,omitempty"`
	WriteIndex string       `json:"write_index,omitempty"`
}

type AliasIndex struct {
	Index         string      `json:"index"`
	Filter        interface{} `json:"filter"`
	IndexRouting  string      `json:"index_routing"`
	SearchRouting string      `json:"search_routing"`
	IsHidden      bool        `json:"is_hidden"`
	IsWriteIndex  bool        `json:"is_write_index"`
}

type NodesResponse struct {
	NodesCount  map[string]interface{} `json:"_nodes"`
	ClusterName string                 `json:"cluster_name"`
	Nodes       map[string]NodesInfo   `json:"nodes"`
}

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
	Deleted int64 `json:"deleted"`
	Total   int64 `json:"total"`
}

type BulkActionMetadata struct {
	Index  *BulkIndexMetadata `json:"index,omitempty"`
	Delete *BulkIndexMetadata `json:"delete,omitempty"`
	Create *BulkIndexMetadata `json:"create,omitempty"`
	Update *BulkIndexMetadata `json:"update,omitempty"`
}

type BulkIndexMetadata struct {
	Index           string      `json:"_index,omitempty"`
	Type            string      `json:"_type,omitempty"`
	ID              string      `json:"_id,omitempty"`
	Pipeline        string      `json:"pipeline,omitempty"`
	Refresh         string      `json:"refresh,omitempty"`
	Source          interface{} `json:"_source,omitempty"`
	SourceExcludes  interface{} `json:"_source_excludes,omitempty"`
	SourceIncludes  interface{} `json:"_source_includes,omitempty"`
	RetryOnConflict interface{} `json:"retry_on_conflict,omitempty"`
	RequireAlias    interface{} `json:"require_alias,omitempty"`
	Parent1         interface{} `json:"_parent,omitempty"`
	Parent2         interface{} `json:"parent,omitempty"`
	Routing1        interface{} `json:"routing,omitempty"`
	Routing2        interface{} `json:"_routing,omitempty"`
	Version1        interface{} `json:"_version,omitempty"`
	Version2        interface{} `json:"version,omitempty"`

	//for bulk response
	Result      string    `json:"result,omitempty"`
	SeqNo       int64     `json:"_seq_no,omitempty"`
	PrimaryTerm int64     `json:"_primary_term,omitempty"`
	Shards      *struct{} `json:"_shards,omitempty"`

	ErrorResponse
}

type BulkResponse struct {
	Took   int                  `json:"took"`
	Errors bool                 `json:"errors"`
	Items  []BulkActionMetadata `json:"items"`
}

type ErrorResponse struct {
	Status int                 `json:"status,omitempty"`
	Error       *struct {
		Type   string `json:"type,omitempty"`
		Reason string `json:"reason,omitempty"`
	} `json:"error,omitempty"`
}

type ElasticsearchMetadata struct {
	Config *ElasticsearchConfig

	NodesTopologyVersion int
	IndicesChanged       bool
	Nodes                map[string]NodesInfo
	Indices              map[string]IndexInfo
	PrimaryShards        map[string]map[int]ShardInfo
	Aliases              map[string]AliasInfo
	HealthStatus string

	HostAvailableInfo map[string]HostAvailableInfo

	clusterFailureTicket int
	clusterOnFailure     bool
	clusterAvailable     bool
	lastSuccess time.Time
	configLock sync.RWMutex
}

type HostAvailableInfo struct {
	Available bool
	Host string
	Port int
	LastActive time.Time
	LastFailure time.Time
}

// ElasticsearchConfig contains common settings for elasticsearch
type ElasticsearchConfig struct {
	Source      string `json:"-"`
	ID          string `json:"-" index:"id"`
	Name        string `json:"name,omitempty" config:"name" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Description string `json:"description,omitempty" elastic_mapping:"description:{type:text}"`
	Enabled     bool   `json:"enabled,omitempty" config:"enabled" elastic_mapping:"enabled:{type:boolean}"`
	Monitored   bool   `json:"monitored,omitempty" config:"monitored" elastic_mapping:"monitored:{type:boolean}"`
	HttpProxy   string `json:"http_proxy,omitempty" config:"http_proxy"`
	Endpoint    string `json:"endpoint,omitempty" config:"endpoint" elastic_mapping:"endpoint:{type:keyword}"`
	Version string `json:"version,omitempty" config:"version"`
	ClientMode string `json:"client_mode,omitempty" config:"client_mode"`

	BasicAuth *struct {
		Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
		Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
	} `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`

	TrafficControl *struct {
		MaxBytesPerNode int `json:"max_bytes_per_node,omitempty" config:"max_bytes_per_node" elastic_mapping:"max_bytes_per_node:{type:keyword}"`
		MaxQpsPerNode   int `json:"max_qps_per_node,omitempty" config:"max_qps_per_node" elastic_mapping:"max_qps_per_node:{type:keyword}"`
	} `config:"traffic_control" json:"traffic_control,omitempty" elastic_mapping:"traffic_control:{type:object}"`

	Discovery struct {
		Enabled bool     `json:"enabled,omitempty" config:"enabled"`
		Modules []string `json:"module,omitempty" config:"module"`
		Refresh struct {
			Enabled  bool   `json:"enabled,omitempty" config:"enabled"`
			Interval string `json:"interval,omitempty" config:"interval"`
		} `json:"refresh,omitempty" config:"refresh"`
	} `json:"discovery,omitempty" config:"discovery"`

	Order   int       `json:"order,omitempty" elastic_mapping:"order:{type:integer}"`
	Created time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
	Updated time.Time `json:"updated,omitempty" elastic_mapping:"updated:{type:date}"`

	Schema string `json:"schema,omitempty" elastic_mapping:"schema:{type:keyword}"`

	Host string `json:"host,omitempty" elastic_mapping:"host:{type:keyword}"`
}

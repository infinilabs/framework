package elastic

import (
	"github.com/dgraph-io/ristretto"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type Stats struct {
	All struct {
		Primaries IndexLevelStats `json:"primaries"`
		Total     IndexLevelStats `json:"total"`
	} `json:"_all"`

	Indices map[string]struct {
		Uuid      string          `json:"uuid"` //v7.0+
		Primaries IndexLevelStats `json:"primaries"`
		Total     IndexLevelStats `json:"total"`
	} `json:"indices"`
}

type IndexLevelStats struct {
	Docs struct {
		Count   int `json:"count"`
		Deleted int `json:"deleted"`
	} `json:"docs"`
	Store struct {
		SizeInBytes             int `json:"size_in_bytes"`
		TotalDataSetSizeInBytes int `json:"total_data_set_size_in_bytes"`
		ReservedInBytes         int `json:"reserved_in_bytes"`
	} `json:"store"`
	Indexing struct {
		IndexTotal           int  `json:"index_total"`
		IndexTimeInMillis    int  `json:"index_time_in_millis"`
		IndexCurrent         int  `json:"index_current"`
		IndexFailed          int  `json:"index_failed"`
		DeleteTotal          int  `json:"delete_total"`
		DeleteTimeInMillis   int  `json:"delete_time_in_millis"`
		DeleteCurrent        int  `json:"delete_current"`
		NoopUpdateTotal      int  `json:"noop_update_total"`
		IsThrottled          bool `json:"is_throttled"`
		ThrottleTimeInMillis int  `json:"throttle_time_in_millis"`
	} `json:"indexing"`
	Get struct {
		Total               int `json:"total"`
		TimeInMillis        int `json:"time_in_millis"`
		ExistsTotal         int `json:"exists_total"`
		ExistsTimeInMillis  int `json:"exists_time_in_millis"`
		MissingTotal        int `json:"missing_total"`
		MissingTimeInMillis int `json:"missing_time_in_millis"`
		Current             int `json:"current"`
	} `json:"get"`
	Search struct {
		OpenContexts        int `json:"open_contexts"`
		QueryTotal          int `json:"query_total"`
		QueryTimeInMillis   int `json:"query_time_in_millis"`
		QueryCurrent        int `json:"query_current"`
		FetchTotal          int `json:"fetch_total"`
		FetchTimeInMillis   int `json:"fetch_time_in_millis"`
		FetchCurrent        int `json:"fetch_current"`
		ScrollTotal         int `json:"scroll_total"`
		ScrollTimeInMillis  int `json:"scroll_time_in_millis"`
		ScrollCurrent       int `json:"scroll_current"`
		SuggestTotal        int `json:"suggest_total"`
		SuggestTimeInMillis int `json:"suggest_time_in_millis"`
		SuggestCurrent      int `json:"suggest_current"`
	} `json:"search"`
	Merges struct {
		Current                    int `json:"current"`
		CurrentDocs                int `json:"current_docs"`
		CurrentSizeInBytes         int `json:"current_size_in_bytes"`
		Total                      int `json:"total"`
		TotalTimeInMillis          int `json:"total_time_in_millis"`
		TotalDocs                  int `json:"total_docs"`
		TotalSizeInBytes           int `json:"total_size_in_bytes"`
		TotalStoppedTimeInMillis   int `json:"total_stopped_time_in_millis"`
		TotalThrottledTimeInMillis int `json:"total_throttled_time_in_millis"`
		TotalAutoThrottleInBytes   int `json:"total_auto_throttle_in_bytes"`
	} `json:"merges"`
	Refresh struct {
		Total                     int `json:"total"`
		TotalTimeInMillis         int `json:"total_time_in_millis"`
		ExternalTotal             int `json:"external_total"`
		ExternalTotalTimeInMillis int `json:"external_total_time_in_millis"`
		Listeners                 int `json:"listeners"`
	} `json:"refresh"`
	Flush struct {
		Total             int `json:"total"`
		Periodic          int `json:"periodic"`
		TotalTimeInMillis int `json:"total_time_in_millis"`
	} `json:"flush"`
	Warmer struct {
		Current           int `json:"current"`
		Total             int `json:"total"`
		TotalTimeInMillis int `json:"total_time_in_millis"`
	} `json:"warmer"`
	QueryCache struct {
		MemorySizeInBytes int `json:"memory_size_in_bytes"`
		TotalCount        int `json:"total_count"`
		HitCount          int `json:"hit_count"`
		MissCount         int `json:"miss_count"`
		CacheSize         int `json:"cache_size"`
		CacheCount        int `json:"cache_count"`
		Evictions         int `json:"evictions"`
	} `json:"query_cache"`
	Fielddata struct {
		MemorySizeInBytes int `json:"memory_size_in_bytes"`
		Evictions         int `json:"evictions"`
	} `json:"fielddata"`
	Completion struct {
		SizeInBytes int `json:"size_in_bytes"`
	} `json:"completion"`
	Segments struct {
		Count                     int   `json:"count"`
		MemoryInBytes             int   `json:"memory_in_bytes"`
		TermsMemoryInBytes        int   `json:"terms_memory_in_bytes"`
		StoredFieldsMemoryInBytes int   `json:"stored_fields_memory_in_bytes"`
		TermVectorsMemoryInBytes  int   `json:"term_vectors_memory_in_bytes"`
		NormsMemoryInBytes        int   `json:"norms_memory_in_bytes"`
		PointsMemoryInBytes       int   `json:"points_memory_in_bytes"`
		DocValuesMemoryInBytes    int   `json:"doc_values_memory_in_bytes"`
		IndexWriterMemoryInBytes  int   `json:"index_writer_memory_in_bytes"`
		VersionMapMemoryInBytes   int   `json:"version_map_memory_in_bytes"`
		FixedBitSetMemoryInBytes  int   `json:"fixed_bit_set_memory_in_bytes"`
		MaxUnsafeAutoIdTimestamp  int64 `json:"max_unsafe_auto_id_timestamp"`
		FileSizes                 struct {
		} `json:"file_sizes"`
	} `json:"segments"`
	Translog struct {
		Operations              int `json:"operations"`
		SizeInBytes             int `json:"size_in_bytes"`
		UncommittedOperations   int `json:"uncommitted_operations"`
		UncommittedSizeInBytes  int `json:"uncommitted_size_in_bytes"`
		EarliestLastModifiedAge int `json:"earliest_last_modified_age"`
	} `json:"translog"`
	RequestCache struct {
		MemorySizeInBytes int `json:"memory_size_in_bytes"`
		Evictions         int `json:"evictions"`
		HitCount          int `json:"hit_count"`
		MissCount         int `json:"miss_count"`
	} `json:"request_cache"`
	Recovery struct {
		CurrentAsSource      int `json:"current_as_source"`
		CurrentAsTarget      int `json:"current_as_target"`
		ThrottleTimeInMillis int `json:"throttle_time_in_millis"`
	} `json:"recovery"`
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

	Http struct {
		BoundAddress            []string `json:"bound_address"`
		PublishAddress          string   `json:"publish_address,omitempty"`
		MaxContentLengthInBytes int64    `json:"max_content_length_in_bytes,omitempty"`
	} `json:"http,omitempty"`

	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Roles      []string               `json:"roles,omitempty"`

	TransportAddress string `json:"transport_address"`
	Host             string `json:"host"`
	Ip               string `json:"ip"`

	BuildFlavor string `json:"build_flavor"`
	BuildType   string `json:"build_type"`
	BuildHash   string `json:"build_hash"`

	TotalIndexingBuffer int64 `json:"total_indexing_buffer,omitempty"`

	Settings   map[string]interface{} `json:"settings"`
	Os         map[string]interface{} `json:"os"`
	Process    map[string]interface{} `json:"process"`
	Jvm        map[string]interface{} `json:"jvm"`
	ThreadPool map[string]interface{} `json:"thread_pool"`
	Transport  struct {
		BoundAddress   []string `json:"bound_address"`
		PublishAddress string   `json:"publish_address"`
		Profiles       struct {
		} `json:"profiles"`
	} `json:"transport"`

	Plugins      []interface{}            `json:"plugins"`
	Modules      []map[string]interface{} `json:"modules"`
	Ingest       map[string]interface{}   `json:"ingest"`
	Aggregations map[string]interface{}   `json:"aggregations"`
}

type NodeAvailable struct {
	ClusterID string
	Host        string
	ticket      int
	available   bool
	isDead bool
	lastCheck time.Time
	lastSuccess time.Time
	configLock  sync.RWMutex
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

type CatNodeResponse struct {
	Id string `json:"id"`
	Ip          string      `json:"ip"`
	Port string `json:"port"`
	HeapPercent string      `json:"heap.percent"`
	RamPercent  string      `json:"ram.percent"`
	Cpu         string      `json:"cpu"`
	Load1M      string      `json:"load_1m"`
	Load5M      interface{} `json:"load_5m"`
	Load15M     interface{} `json:"load_15m"`
	NodeRole    string      `json:"node.role"`
	Master      string      `json:"master"`
	Name        string      `json:"name"`
	DiskAvail string `json:"disk.avail"`
	Shards int `json:"shards,omitempty"`
	Uptime string `json:"uptime"`
}

type ReindexResponse struct {
	Task string `json:"task"`
}

type DeleteByQueryResponse struct {
	Deleted int64 `json:"deleted"`
	Total   int64 `json:"total"`
}
type UpdateByQueryResponse struct {
	Updated int64 `json:"updated"`
	Total   int64 `json:"total"`
	Failures []interface{} `json:"failures"`
}

type BulkActionMetadata struct {
	Index  *BulkIndexMetadata `json:"index,omitempty"`
	Delete *BulkIndexMetadata `json:"delete,omitempty"`
	Create *BulkIndexMetadata `json:"create,omitempty"`
	Update *BulkIndexMetadata `json:"update,omitempty"`
	Stats int `json:"-,omitempty"`
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
	Status    int         `json:"status,omitempty"`
	RootCause interface{} `json:"root_cause,omitempty"`
	Error     *struct {
		Type   string `json:"type,omitempty"`
		Reason string `json:"reason,omitempty"`
	} `json:"error,omitempty"`
}

type ElasticsearchMetadata struct {
	Config *ElasticsearchConfig

	ClusterState *ClusterState

	NodesTopologyVersion int

	Nodes *map[string]NodesInfo

	IndexSettings map[string]*util.MapStr

	//PrimaryShards *map[string]map[int]ShardInfo
	Aliases *map[string]AliasInfo
	Health  *ClusterHealth

	clusterFailureTicket int
	clusterAvailable     bool
	lastSuccess          time.Time
	configLock           sync.RWMutex
	seedHosts            []string
	activeHost           *NodeAvailable

	cache *ristretto.Cache
}
type TaskConfig struct {
	Enabled bool `json:"enabled"`
	Interval string `json:"interval"`
}
type MonitorConfig struct{
	ClusterHealth TaskConfig `json:"cluster_health"`
	ClusterStats  TaskConfig `json:"cluster_stats"`
	NodeStats     TaskConfig `json:"node_stats"`
	IndexStats    TaskConfig `json:"index_stats"`
}
type MetadataConfig struct {
	HealthCheck           TaskConfig `json:"health_check"`
	ClusterSettingsCheck  TaskConfig `json:"cluster_settings_check"`
	MetadataRefresh       TaskConfig `json:"metadata_refresh"`
	NodeAvailabilityCheck TaskConfig `json:"node_availability_check"`
}

// ElasticsearchConfig contains common settings for elasticsearch
type ElasticsearchConfig struct {
	orm.ORMObjectBase

	Source      string   `json:"source,omitempty"`
	Name        string   `json:"name,omitempty" config:"name" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Description string   `json:"description,omitempty" elastic_mapping:"description:{type:text}"`
	Enabled     bool     `json:"enabled,omitempty" config:"enabled" elastic_mapping:"enabled:{type:boolean}"`
	Monitored   bool              `json:"monitored,omitempty" config:"monitored" elastic_mapping:"monitored:{type:boolean}"`
	MonitorConfigs *MonitorConfig `config:"monitor_configs" json:"monitor_configs,omitempty" elastic_mapping:"monitor_configs:{type:object}"`
	HttpProxy   string            `json:"http_proxy,omitempty" config:"http_proxy"`
	Endpoint    string   `json:"endpoint,omitempty" config:"endpoint" elastic_mapping:"endpoint:{type:keyword}"`
	Endpoints   []string `json:"endpoints,omitempty" config:"endpoints" elastic_mapping:"endpoints:{type:keyword}"`
	Version     string   `json:"version,omitempty" config:"version" elastic_mapping:"version:{type:keyword,copy_to:search_text}"`
	ClientMode  string   `json:"client_mode,omitempty" config:"client_mode"`

	RequestTimeout  int  `json:"request_timeout,omitempty" config:"request_timeout"`
	RequestCompress bool `json:"request_compress,omitempty" config:"request_compress"`

	AllowAccessWhenMasterNotFound bool `json:"allow_access_when_master_not_found,omitempty" config:"allow_access_when_master_not_found"`

	BasicAuth *struct {
		Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
		Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
	} `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`

	TrafficControl *struct {
		MaxConnectionPerNode int `json:"max_connection_per_node,omitempty" config:"max_connection_per_node" elastic_mapping:"max_connection_per_node:{type:keyword}"`
		MaxWaitTimeInMs      int `json:"max_wait_time_in_ms,omitempty" config:"max_wait_time_in_ms" elastic_mapping:"max_wait_time_in_ms:{type:keyword}"`
		MaxBytesPerNode      int `json:"max_bytes_per_node,omitempty" config:"max_bytes_per_node" elastic_mapping:"max_bytes_per_node:{type:keyword}"`
		MaxQpsPerNode        int `json:"max_qps_per_node,omitempty" config:"max_qps_per_node" elastic_mapping:"max_qps_per_node:{type:keyword}"`
	} `config:"traffic_control" json:"traffic_control,omitempty" elastic_mapping:"traffic_control:{type:object}"`

	Discovery struct {
		Enabled bool     `json:"enabled,omitempty" config:"enabled"`
		Modules []string `json:"module,omitempty" config:"module"`
		Refresh struct {
			Enabled  bool   `json:"enabled,omitempty" config:"enabled"`
			Interval string `json:"interval,omitempty" config:"interval"`
		} `json:"refresh,omitempty" config:"refresh"`
	} `json:"discovery,omitempty" config:"discovery"`

	Order int `json:"order,omitempty" elastic_mapping:"order:{type:integer}"`

	Schema string `json:"schema,omitempty" elastic_mapping:"schema:{type:keyword}"`

	Host  string   `json:"host,omitempty" elastic_mapping:"host:{type:keyword,copy_to:search_text}"`
	Hosts []string `json:"hosts,omitempty" elastic_mapping:"hosts:{type:keyword}"`

	GeoLocation *GeoLocation `json:"location,omitempty" elastic_mapping:"location:{type:object}"`

	Project string `json:"project,omitempty" elastic_mapping:"project:{type:keyword,copy_to:search_text}"`

	Owner []struct {
		Department string `json:"department,omitempty" elastic_mapping:"department:{type:keyword,copy_to:search_text}"`
		Name       string `json:"name,omitempty" elastic_mapping:"name:{type:keyword,copy_to:search_text}"`
		ID         string `json:"id,omitempty" elastic_mapping:"id:{type:keyword}"`
	} `json:"owner,omitempty" elastic_mapping:"owner:{type:object}"`
	Labels        util.MapStr `json:"labels,omitempty"`
	Tags          []string    `json:"tags,omitempty" elastic_mapping:"tags:{type:keyword,copy_to:search_text}"`
	SearchText    string      `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
	MaxCachedSize int64       `json:"max_cached_size,omitempty" elastic_mapping:"max_cached_size:{type:integer}"`
	MetadataConfigs *MetadataConfig `config:"metadata_configs" json:"metadata_configs,omitempty" elastic_mapping:"metadata_configs:{type:object}"`
	ClusterUUID string  `json:"cluster_uuid,omitempty" elastic_mapping:"cluster_uuid:{type:keyword}"`
	RawName string  `json:"raw_name,omitempty" elastic_mapping:"raw_name:{type:keyword}"`
}

type GeoLocation struct{
	Provider string `json:"provider,omitempty" elastic_mapping:"provider:{type:keyword}"`
	Region string `json:"region,omitempty" elastic_mapping:"region:{type:keyword}"`
	DC string `json:"dc,omitempty" elastic_mapping:"dc:{type:keyword}"`
	Rack string `json:"rack,omitempty" elastic_mapping:"rack:{type:keyword}"`
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
	StoreInBytes int64 `json:"store_in_bytes,omitempty"`
	NodeID           string `json:"id,omitempty"`
	NodeName         string `json:"node,omitempty"`
	NodeIP           string `json:"ip,omitempty"`
}

type TraceMeta struct {
	TraceID   bool     `json:"trace_id" elastic_mapping:"trace_id:{type:keyword}"`
	Index string `json:"index" elastic_mapping:"index:{type:keyword}"`
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
}

type NodeConfig struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	Metadata  NodeMetadata `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Fields     util.MapStr `json:"payload" elastic_mapping:"payload:{type:object,enabled:false}"`
	SearchText string `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
}
type NodeMetadata struct {
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	ClusterName string `json:"cluster_name" elastic_mapping:"cluster_name:{type:keyword,copy_to:search_text}"`
	Host string `json:"host" elastic_mapping:"host:{type:keyword,copy_to:search_text}"`
	NodeID string `json:"node_id" elastic_mapping:"node_id:{type:keyword}"`
	NodeName string `json:"node_name" elastic_mapping:"node_name:{type:keyword,copy_to:search_text}"`
	Tags []string `json:"tags,omitempty" elastic_mapping:"tags:{type:keyword,copy_to:search_text}"`
	Labels util.MapStr `json:"labels,omitempty"`
	Category string `json:"category,omitempty"`
}

type HostMetadata struct {
	ID        string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Timestamp time.Time `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	ClusterID string    `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	NodeID    string    `json:"node_id" elastic_mapping:"node_id:{type:keyword}"`
	Metadata  struct {
		Host string                 `json:"host" elastic_mapping:"host:{type:keyword}"`
		IPs  []string               `json:"ips" elastic_mapping:"ips:{type:keyword}"`
		OS   map[string]interface{} `json:"os" elastic_mapping:"os:{type:object,enabled:false}"`
	} `json:"metadata" elastic_mapping:"metadata:{type:object}"`
}

type IndexConfig struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	Metadata  IndexMetadata `json:"metadata"`
	Fields     util.MapStr `json:"payload" elastic_mapping:"payload:{type:object,enabled:false}"`
	SearchText string `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
}

type IndexMetadata struct {
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	IndexID string `json:"index_id" elastic_mapping:"index_id:{type:keyword}"` //cluster_id:index_name (some metrics not return index uuid of lower es version )
	IndexName string `json:"index_name" elastic_mapping:"index_name:{type:keyword,copy_to:search_text}"`
	ClusterName string `json:"cluster_name" elastic_mapping:"cluster_name:{type:keyword,copy_to:search_text}"`
	Labels util.MapStr `json:"labels,omitempty"`
	Aliases interface{} `json:"aliases,omitempty" elastic_mapping:"aliases:{type:keyword,copy_to:search_text}"`
	Category string `json:"category,omitempty"`
	Tags []interface{} `json:"tags,omitempty" elastic_mapping:"tags:{type:keyword,copy_to:search_text}"`
}

package elastic

import (
	"errors"
	"github.com/buger/jsonparser"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/util"
	"strings"
	"time"
)

type Indexes map[string]interface{}

type ShardResponse struct {
	Total      int `json:"total,omitempty"`
	Successful int `json:"successful,omitempty"`
	Skipped    int `json:"skipped,omitempty"`
	Failed     int `json:"failed,omitempty"`
	Failures   []struct {
		Shard  int             `json:"shard,omitempty"`
		Index  string          `json:"index,omitempty,nocopy,intern"`
		Status int             `json:"status,omitempty"`
		Reason json.RawMessage `json:"reason,omitempty,nocopy"`
	} `json:"failures,omitempty"`
}

type ClusterInformation struct {
	Name        string `json:"name,nocopy,intern"`
	ClusterName string `json:"cluster_name,nocopy,intern"`
	Version     struct {
		Number        string `json:"number,nocopy,intern"`
		LuceneVersion string `json:"lucene_version,nocopy,intern"`
	} `json:"version"`
}

//"cluster_name": "pi",
//"status": "green",
//"timed_out": false,
//"number_of_nodes": 3,
//"number_of_data_nodes": 3,
//"active_primary_shards": 58,
//"active_shards": 116,
//"relocating_shards": 0,
//"initializing_shards": 0,
//"unassigned_shards": 0,
//"delayed_unassigned_shards": 0,
//"number_of_pending_tasks": 0,
//"number_of_in_flight_fetch": 0,
//"task_max_waiting_in_queue_millis": 0,
//"active_shards_percent_as_number": 100

type ClusterHealth struct {
	ResponseBase
	Name                        string  `json:"cluster_name"`
	Status                      string  `json:"status"`
	TimedOut                    bool    `json:"timed_out"`
	NumberOfNodes               int     `json:"number_of_nodes"`
	NumberOf_data_nodes         int     `json:"number_of_data_nodes"`
	ActivePrimary_shards        int     `json:"active_primary_shards"`
	ActiveShards                int     `json:"active_shards"`
	RelocatingShards            int     `json:"relocating_shards"`
	InitializingShards          int     `json:"initializing_shards"`
	UnassignedShards            int     `json:"unassigned_shards"`
	DelayedUnassignedShards     int     `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        int     `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       int     `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis float64 `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber float64 `json:"active_shards_percent_as_number"`
}

type ClusterState struct {
	ResponseBase

	ClusterName string `json:"cluster_name"`
	Version     int64  `json:"version"`
	StateUUID   string `json:"state_uuid"`
	MasterNode  string `json:"master_node"`
	//Nodes        map[string]ClusterStateNodes `json:"nodes"`
	RoutingTable *ClusterRoutingTable `json:"routing_table,omitempty"`

	CompressedSizeInBytes int `json:"compressed_size_in_bytes"` //v6.0+
}

type ClusterStateNodes struct {
	Name string `json:"name"`

	EphemeralId string `json:"ephemeral_id"` //v5.0+

	TransportAddress string                 `json:"transport_address"`
	Attributes       map[string]interface{} `json:"attributes,omitempty"`
}
type ClusterRoutingTable struct {
	Indices map[string]struct {
		Shards map[string][]IndexShardRouting`json:"shards"`
	} `json:"indices"`
}

type IndexShardRouting struct {
	State          string      `json:"state"`
	Primary        bool        `json:"primary"`
	Node           string      `json:"node"`
	RelocatingNode interface{} `json:"relocating_node,omitempty"`
	Shard          int         `json:"shard"`
	Index          string      `json:"index"`
	Version        int         `json:"version"` //< v5.0

	//v5.0+ START
	RecoverySource *struct {
		Type string `json:"type"`
	} `json:"recovery_source,omitempty"`
	UnassignedInfo *struct {
		Reason           string    `json:"reason"`
		At               time.Time `json:"at"`
		Delayed          bool      `json:"delayed"`
		AllocationStatus string    `json:"allocation_status"`
	} `json:"unassigned_info,omitempty"`
	//END

	AllocationId *struct {
		Id string `json:"id"`
	} `json:"allocation_id,omitempty"`
}

type ClusterStats struct {
	ResponseBase
	ClusterName string                 `json:"cluster_name"`
	Status      string                 `json:"status"`
	ClusterUUID string                 `json:"cluster_uuid"`
	Timestamp   int64                  `json:"timestamp"`
	Indices     map[string]interface{} `json:"indices"`
	Nodes       map[string]interface{} `json:"nodes"`
}

type NodesStats struct {
	ResponseBase
	Nodes map[string]interface{} `json:"nodes"`
}

type IndicesStats struct {
	ResponseBase
	Nodes map[string]interface{} `json:"indices"`
}

// IndexDocument used to construct indexing document
type IndexDocument struct {
	Index     string                   `json:"_index,omitempty"`
	Type      string                   `json:"_type,omitempty"`
	ID        string                   `json:"_id,omitempty"`
	Routing   string                   `json:"_routing,omitempty"`
	Source    map[string]interface{}   `json:"_source,omitempty"`
	Highlight map[string][]interface{} `json:"highlight,omitempty"`
}

type BucketBase map[string]interface{}

type Bucket struct {
	KeyAsString interface{} `json:"key_as_string,omitempty"`
	Key         interface{} `json:"key,omitempty"`
	DocCount    int64       `json:"doc_count,omitempty"`
}

type AggregationResponse struct {
	Buckets []BucketBase `json:"buckets,omitempty"`
}

type ResponseBase struct {
	RawResult 		*util.Result  `json:"-"`
	StatusCode  int   `json:"-"`
	ErrorObject error `json:"-"`
}

func (this *ResponseBase)GetIntByJsonPath(path string) (interface{},error) {
	if this.RawResult.Body!=nil{
		pathArray:=strings.Split(path,".")
		v,err:=jsonparser.GetInt(this.RawResult.Body,pathArray...)
		return v,err
	}
	return nil,errors.New("nil body")
}

func (this *ResponseBase)GetBytesByJsonPath(path string) ([]byte,error) {
	if this.RawResult.Body!=nil{
		pathArray:=strings.Split(path,".")
		v,_,_,err:=jsonparser.Get(this.RawResult.Body,pathArray...)
		return v,err
	}
	return nil,errors.New("nil body")
}

func (this *ResponseBase)GetStringByJsonPath(path string) (interface{},error) {
	if this.RawResult.Body!=nil{
		pathArray:=strings.Split(path,".")
		v,err:=jsonparser.GetString(this.RawResult.Body,pathArray...)
		return v,err
	}
	return nil,errors.New("nil body")
}

func (this *ResponseBase)GetBoolByJsonPath(path string) (interface{},error) {
	if this.RawResult.Body!=nil{
		pathArray:=strings.Split(path,".")
		v,err:=jsonparser.GetBoolean(this.RawResult.Body,pathArray...)
		return v,err
	}
	return nil,errors.New("nil body")
}

// InsertResponse is a index response object
type InsertResponse struct {
	ResponseBase
	Result  string `json:"result"`
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	ID      string `json:"_id"`
	Version int    `json:"_version"`

	Shards struct {
		Total      int `json:"total" `
		Failed     int `json:"failed"`
		Successful int `json:"successful"`
	} `json:"_shards"` //es 2.x index api
}

// GetResponse is a get response object
type GetResponse struct {
	ResponseBase
	Found   bool                   `json:"found"`
	Index   string                 `json:"_index"`
	Type    string                 `json:"_type"`
	ID      string                 `json:"_id"`
	Version int                    `json:"_version"`
	Source  map[string]interface{} `json:"_source"`
}

// DeleteResponse is a delete response object
type DeleteResponse struct {
	ResponseBase
	Result  string `json:"result"`
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	ID      string `json:"_id"`
	Version int    `json:"_version"`
	Shards  struct {
		Total      int `json:"total" `
		Failed     int `json:"failed"`
		Successful int `json:"successful"`
	} `json:"_shards"` //es 2.x index api
}

// CountResponse is a count response object
type CountResponse struct {
	ResponseBase
	Count int64 `json:"count"`
}

// SearchResponse is a count response object
type SearchResponse struct {
	ResponseBase
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Hits     struct {
		Total    interface{}     `json:"total"`
		MaxScore float32         `json:"max_score"`
		Hits     []IndexDocument `json:"hits,omitempty"`
	} `json:"hits"`
	Aggregations map[string]AggregationResponse `json:"aggregations,omitempty"`
}

func (response *SearchResponse) GetTotal() int64 {

	if response.Hits.Total != nil {

		if util.TypeIsMap(response.Hits.Total) {
			v := response.Hits.Total.(map[string]interface{})
			return util.GetInt64Value(v["value"])
		} else {
			return util.GetInt64Value(response.Hits.Total)
		}
	}
	return -1
}

// RangeQuery is used to find value in range
type RangeQuery struct {
	Range map[string]map[string]interface{} `json:"range,omitempty"`
}

func (query *RangeQuery) Gt(field string, value interface{}) {
	query.Range = map[string]map[string]interface{}{}
	v := map[string]interface{}{}
	v["gt"] = value
	query.Range[field] = v
}

func (query *RangeQuery) Gte(field string, value interface{}) {
	query.Range = map[string]map[string]interface{}{}
	v := map[string]interface{}{}
	v["gte"] = value
	query.Range[field] = v
}

func (query *RangeQuery) Lt(field string, value interface{}) {
	query.Range = map[string]map[string]interface{}{}
	v := map[string]interface{}{}
	v["lt"] = value
	query.Range[field] = v
}

func (query *RangeQuery) Lte(field string, value interface{}) {
	query.Range = map[string]map[string]interface{}{}
	v := map[string]interface{}{}
	v["lte"] = value
	query.Range[field] = v
}

type MatchQuery struct {
	Match map[string]interface{} `json:"match,omitempty"`
}

type TermsQuery struct {
	Match map[string][]interface{} `json:"terms,omitempty"`
}

func (match *TermsQuery) Set(field string, v []interface{}) {
	match.Match = map[string][]interface{}{}
	match.Match[field] = v
}

type QueryStringQuery struct {
	Query map[string]interface{} `json:"query_string,omitempty"`
}

func NewQueryString(q string) *QueryStringQuery {
	query := QueryStringQuery{}
	query.Query = map[string]interface{}{}
	return &query
}

type TermsAggregationQuery struct {
	term string
	size int
}

func (query *TermsAggregationQuery) Field(field string) *TermsAggregationQuery {
	query.term = field
	return query
}

func (query *TermsAggregationQuery) Size(size int) *TermsAggregationQuery {
	query.size = size
	return query
}

func NewTermsAggregation() (query *TermsAggregationQuery) {
	return &TermsAggregationQuery{}
}

func (query *QueryStringQuery) QueryString(q string) {
	query.Query["query"] = q
}

func (query *QueryStringQuery) DefaultOperator(op string) {
	query.Query["default_operator"] = op
}

func (query *QueryStringQuery) Fields(fields ...string) {
	query.Query["default_operator"] = fields
}

// Init match query's condition
func (match *MatchQuery) Set(field string, v interface{}) {
	match.Match = map[string]interface{}{}
	match.Match[field] = v
}

// BoolQuery wrapper queries
type BoolQuery struct {
	Must    []interface{} `json:"must,omitempty"`
	MustNot []interface{} `json:"must_not,omitempty"`
	Should  []interface{} `json:"should,omitempty"`
}

// Query is the root query object
type Query struct {
	BoolQuery *BoolQuery `json:"bool"`
}

// SearchRequest is the root search query object
type SearchRequest struct {
	Query              *Query         `json:"query,omitempty"`
	From               int            `json:"from"`
	Collapse           *Collapse `json:"collapse,omitempty"`
	Size               int            `json:"size"`
	Sort               *[]interface{} `json:"sort,omitempty"`
	AggregationRequest `json:"aggs,omitempty"`
}

type Collapse struct {
	Field string `json:"field,omitempty"`
}

type AggregationRequest struct {
	Aggregations map[string]Aggregation `json:"aggregations,omitempty"`
}

type Aggregation struct {
}

// AddSort add sort conditions to SearchRequest
func (request *SearchRequest) AddSort(field string, order string) {
	if (request.Sort) == nil {
		s := []interface{}{}
		request.Sort = &s
	}
	s := map[string]interface{}{}
	v := map[string]interface{}{}
	v["order"] = order
	s[field] = v
	*request.Sort = append(*request.Sort, s)
}

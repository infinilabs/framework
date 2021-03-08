package elastic

import (
	"infini.sh/framework/core/util"
)

type Indexes map[string]interface{}

type ScrollResponseAPI interface {
	GetScrollId() string
	SetScrollId(id string)
	GetHitsTotal() int
	GetShardResponse() ShardResponse
	GetDocs() []interface{}
}

type ShardResponse struct {
	Total      int `json:"total,omitempty"`
	Successful int `json:"successful,omitempty"`
	Skipped    int `json:"skipped,omitempty"`
	Failed     int `json:"failed,omitempty"`
	Failures   []struct {
		Shard  int         `json:"shard,omitempty"`
		Index  string      `json:"index,omitempty"`
		Status int         `json:"status,omitempty"`
		Reason interface{} `json:"reason,omitempty"`
	} `json:"failures,omitempty"`
}

type ScrollResponse struct {
	Took     int    `json:"took,omitempty"`
	ScrollId string `json:"_scroll_id,omitempty"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Hits     struct {
		MaxScore float32       `json:"max_score,omitempty"`
		Total    int           `json:"total,omitempty"`
		Docs     []interface{} `json:"hits,omitempty"`
	} `json:"hits"`
	Shards ShardResponse `json:"_shards,omitempty"`
}

type ScrollResponseV7 struct {
	ScrollResponse
	Hits struct {
		MaxScore float32 `json:"max_score,omitempty"`
		Total    struct {
			Value    int    `json:"value,omitempty"`
			Relation string `json:"relation,omitempty"`
		} `json:"total,omitempty"`
		Docs []interface{} `json:"hits,omitempty"`
	} `json:"hits"`
}

func (scroll *ScrollResponse) GetHitsTotal() int {
	return scroll.Hits.Total
}

func (scroll *ScrollResponse) GetScrollId() string {
	return scroll.ScrollId
}

func (scroll *ScrollResponse) SetScrollId(id string) {
	scroll.ScrollId = id
}

func (scroll *ScrollResponse) GetDocs() []interface{} {
	return scroll.Hits.Docs
}

func (scroll *ScrollResponse) GetShardResponse() ShardResponse {
	return scroll.Shards
}

func (scroll *ScrollResponseV7) GetHitsTotal() int {
	return scroll.Hits.Total.Value
}

func (scroll *ScrollResponseV7) GetScrollId() string {
	return scroll.ScrollId
}

func (scroll *ScrollResponseV7) SetScrollId(id string) {
	scroll.ScrollId = id
}

func (scroll *ScrollResponseV7) GetDocs() []interface{} {
	return scroll.Hits.Docs
}

func (scroll *ScrollResponseV7) GetShardResponse() ShardResponse {
	return scroll.Shards
}

type ClusterInformation struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
	Version     struct {
		Number        string `json:"number"`
		LuceneVersion string `json:"lucene_version"`
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
	Name   string `json:"cluster_name"`
	Status string `json:"status"`
	TimedOut bool `json:"timed_out"`
	NumberOfNodes int `json:"number_of_nodes"`
	NumberOf_data_nodes int `json:"number_of_data_nodes"`
	ActivePrimary_shards int `json:"active_primary_shards"`
	ActiveShards int `json:"active_shards"`
	RelocatingShards int `json:"relocating_shards"`
	InitializingShards int `json:"initializing_shards"`
	UnassignedShards int `json:"unassigned_shards"`
	DelayedUnassignedShards int `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks int `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch int `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis float64 `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber float64 `json:"active_shards_percent_as_number"`
}


type ClusterStats struct {
	ResponseBase
	ClusterName   string `json:"cluster_name"`
	Status string `json:"status"`
	ClusterUUID string `json:"cluster_uuid"`
	Timestamp int64 `json:"timestamp"`
	Indices map[string]interface{} `json:"indices"`
	Nodes map[string]interface{} `json:"nodes"`
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
	ID        interface{}              `json:"_id,omitempty"`
	Routing   string                   `json:"_routing,omitempty"`
	Source    map[string]interface{}   `json:"_source,omitempty"`
	Highlight map[string][]interface{} `json:"highlight,omitempty"`
}

type Bucket struct {
	Key      string `json:"key,omitempty"`
	DocCount int    `json:"doc_count,omitempty"`
}

type AggregationResponse struct {
	Buckets []Bucket `json:"buckets,omitempty"`
}

type ResponseBase struct {
	StatusCode int `json:"-"`
	ErrorObject  error `json:"-"`
}

// InsertResponse is a index response object
type InsertResponse struct {
	ResponseBase
	Result  string `json:"result"`
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	ID      string `json:"_id"`
	Version int    `json:"_version"`

	Shards struct{
		Total int `json:"total" `
		Failed int `json:"failed"`
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
	Shards struct{
		Total int `json:"total" `
		Failed int `json:"failed"`
		Successful int `json:"successful"`
	} `json:"_shards"` //es 2.x index api
}

// CountResponse is a count response object
type CountResponse struct {
	ResponseBase
	Count int `json:"count"`
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

func (response *SearchResponse) GetTotal() int {

	if response.Hits.Total != nil {

		if util.TypeIsMap(response.Hits.Total) {
			v := response.Hits.Total.(map[string]interface{})
			return util.GetIntValue(v["value"])
		} else {
			return util.GetIntValue(response.Hits.Total)
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
	Size               int            `json:"size"`
	Sort               *[]interface{} `json:"sort,omitempty"`
	AggregationRequest `json:"aggs,omitempty"`
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

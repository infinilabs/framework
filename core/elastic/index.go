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

package elastic

import (
	"errors"
	"github.com/buger/jsonparser"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/util"
	"strings"
	"time"
)

type ShardResponse struct {
	Total      int `json:"total,omitempty"`
	Successful int `json:"successful,omitempty"`
	Skipped    int `json:"skipped,omitempty"`
	Failed     int `json:"failed,omitempty"`
	Failures   []struct {
		Shard  int             `json:"shard,omitempty"`
		Index  string          `json:"index,omitempty,intern"`
		Status int             `json:"status,omitempty"`
		Reason json.RawMessage `json:"reason,omitempty,nocopy"`
	} `json:"failures,omitempty"`
}

type ClusterInformation struct {
	Name        string `json:"name,intern"`
	ClusterName string `json:"cluster_name,intern"`
	ClusterUUID string `json:"cluster_uuid"`
	Version     struct {
		Number        string `json:"number,intern"`
		LuceneVersion string `json:"lucene_version,intern"`
		Distribution  string `json:"distribution"`
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
	Name                        string                            `json:"cluster_name"`
	Status                      string                            `json:"status"`
	TimedOut                    bool                              `json:"timed_out"`
	NumberOfNodes               int                               `json:"number_of_nodes"`
	NumberOf_data_nodes         int                               `json:"number_of_data_nodes"`
	ActivePrimary_shards        int                               `json:"active_primary_shards"`
	ActiveShards                int                               `json:"active_shards"`
	RelocatingShards            int                               `json:"relocating_shards"`
	InitializingShards          int                               `json:"initializing_shards"`
	UnassignedShards            int                               `json:"unassigned_shards"`
	DelayedUnassignedShards     int                               `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        int                               `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       int                               `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis float64                           `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber float64                           `json:"active_shards_percent_as_number"`
	Indices                     map[string]map[string]interface{} `json:"indices"`
}

type ClusterState struct {
	ResponseBase

	ClusterName string `json:"cluster_name"`
	Version     int64  `json:"version"`
	StateUUID   string `json:"state_uuid"`
	ClusterUUID string `json:"cluster_uuid"`
	MasterNode  string `json:"master_node"`
	//Nodes        map[string]ClusterStateNodes `json:"nodes"`
	RoutingTable *ClusterRoutingTable `json:"routing_table,omitempty"`

	CompressedSizeInBytes int `json:"compressed_size_in_bytes"` //v6.0+
	Metadata              *struct {
		Indices map[string]interface{} `json:"indices"`
	} `json:"metadata,omitempty"`
}

type ClusterStateNodes struct {
	Name string `json:"name"`

	EphemeralId string `json:"ephemeral_id"` //v5.0+

	TransportAddress string                 `json:"transport_address"`
	Attributes       map[string]interface{} `json:"attributes,omitempty"`
}
type ClusterRoutingTable struct {
	Indices map[string]struct {
		Shards map[string][]IndexShardRouting `json:"shards"`
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
	Score     float32                  `json:"_score,omitempty"`
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
	Value   interface{}  `json:"value,omitempty"`
}

type ResponseBase struct {
	RawResult   *util.Result `json:"-"`
	StatusCode  int          `json:"-"`
	ErrorObject error        `json:"-"`
	InternalError
}

type InternalError struct {
	Error  *ErrorDetail `json:"error,omitempty"`
	Status int          `json:"status,omitempty"`
}

type ErrorDetail struct {
	RootCause []RootCause `json:"root_cause,omitempty"`
	Type      string      `json:"type,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

type RootCause struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func (this *ResponseBase) GetIntByJsonPath(path string) (interface{}, error) {
	if this.RawResult.Body != nil {
		pathArray := strings.Split(path, ".")
		v, err := jsonparser.GetInt(this.RawResult.Body, pathArray...)
		return v, err
	}
	return nil, errors.New("nil body")
}

func (this *ResponseBase) GetBytesByJsonPath(path string) ([]byte, error) {
	if this.RawResult.Body != nil {
		pathArray := strings.Split(path, ".")
		v, _, _, err := jsonparser.Get(this.RawResult.Body, pathArray...)
		return v, err
	}
	return nil, errors.New("nil body")
}

func (this *ResponseBase) GetStringByJsonPath(path string) (interface{}, error) {
	if this.RawResult.Body != nil {
		pathArray := strings.Split(path, ".")
		v, err := jsonparser.GetString(this.RawResult.Body, pathArray...)
		return v, err
	}
	return nil, errors.New("nil body")
}

func (this *ResponseBase) GetBoolByJsonPath(path string) (interface{}, error) {
	if this.RawResult.Body != nil {
		pathArray := strings.Split(path, ".")
		v, err := jsonparser.GetBoolean(this.RawResult.Body, pathArray...)
		return v, err
	}
	return nil, errors.New("nil body")
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
	Index   string                 `json:"_index,omitempty"`
	Type    string                 `json:"_type,omitempty"`
	ID      string                 `json:"_id,omitempty"`
	Version int                    `json:"_version,omitempty"`
	Source  map[string]interface{} `json:"_source,omitempty"`
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
	Took     int  `json:"took,omitempty"`
	TimedOut bool `json:"timed_out,omitempty"`
	Hits     struct {
		Total    interface{}     `json:"total,omitempty"`
		MaxScore float32         `json:"max_score,omitempty"`
		Hits     []IndexDocument `json:"hits,omitempty"`
	} `json:"hits,omitempty"`
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

func (match *TermsQuery) SetStringArray(field string, v []string) {
	match.Match = map[string][]interface{}{}
	obj := []interface{}{}
	for _, s := range v {
		if s != "" {
			obj = append(obj, s)
		}
	}
	match.Match[field] = obj
}

type QueryStringQuery struct {
	Query map[string]interface{} `json:"query_string,omitempty"`
}

func NewQueryString(q string) *QueryStringQuery {
	query := QueryStringQuery{}
	query.Query = map[string]interface{}{}
	query.QueryString(q)
	return &query
}

type PrefixQuery struct {
	Prefix map[string]string `json:"prefix,omitempty"`
}

func (query *PrefixQuery) Set(field string, val string) {
	query.Prefix = map[string]string{}
	query.Prefix[field] = val
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
	query.Query["fields"] = fields
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
	Filter  []interface{} `json:"filter,omitempty"`
}

// Query is the root query object
type Query struct {
	BoolQuery *BoolQuery `json:"bool"`
}

func (q *Query) Must(query interface{}) {
	if q.BoolQuery == nil {
		q.BoolQuery = &BoolQuery{}
	}
	q.BoolQuery.Must = append(q.BoolQuery.Must, query)
}

// SearchRequest is the root search query object
type SearchRequest struct {
	rootField util.MapStr

	Query *Query `json:"query,omitempty"`
	From  int    `json:"from"`
	Size  int    `json:"size"`

	Collapse *Collapse `json:"collapse,omitempty"`

	Sort               *[]interface{}      `json:"sort,omitempty"`
	Source             interface{}         `json:"_source,omitempty"`
	AggregationRequest *AggregationRequest `json:"aggs,omitempty"`
}

func (request *SearchRequest) ToJSONString() string {
	if request.Query != nil {
		request.Set("query", request.Query)
	}

	if request.From >= 0 {
		request.Set("from", request.From)
	}
	if request.Size >= 0 {
		request.Set("size", request.Size)
	}

	if request.Collapse != nil {
		request.Set("collapse", request.Collapse)
	}
	if request.Sort != nil {
		request.Set("sort", request.Sort)
	}

	if request.Source != nil {
		request.Set("_source", request.Source)
	}

	if request.AggregationRequest != nil {
		request.Set("aggs", request.AggregationRequest)
	}

	return util.ToJson(request.rootField, false)
}

func GetSearchRequest(querystring, dsl, sourceFields string, sortField, sortType string) *SearchRequest {
	var query = &SearchRequest{}
	if dsl != "" {
		err := util.FromJSONBytes([]byte(dsl), query)
		if err != nil {
			panic(err)
		}
	}

	if querystring != "" {
		queryString := NewQueryString(querystring)
		if query.Query == nil {
			query.Query = &Query{}
		}
		query.Query.Must(queryString)
	}

	//handle sort
	if len(sortField) > 0 {
		if len(sortType) == 0 {
			sortType = "asc"
		}
		query.AddSort(sortField, sortType)
	}

	//handle _source
	if len(sourceFields) > 0 {
		if !strings.Contains(sourceFields, ",") {
			query.Source = sourceFields
		} else {
			query.Source = strings.Split(sourceFields, ",")
		}
	}
	return query
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

func (request *SearchRequest) Set(key string, value interface{}) error {
	if request.rootField == nil {
		request.rootField = util.MapStr{}
	}
	_, err := request.rootField.Put(key, value)
	return err
}

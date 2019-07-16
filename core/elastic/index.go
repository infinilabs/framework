package elastic

import (
	"github.com/infinitbyte/framework/core/util"
)

type Indexes map[string]interface{}

type ScrollResponse struct {
	Took     int    `json:"took"`
	ScrollId string `json:"_scroll_id"`
	TimedOut bool   `json:"timed_out"`
	Hits     struct {
		MaxScore float32       `json:"max_score"`
		Total    int           `json:"total"`
		Docs     []interface{} `json:"hits"`
	} `json:"hits"`
	Shards struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Failed     int `json:"failed"`
		Failures   []struct {
			Shard  int         `json:"shard"`
			Index  string      `json:"index"`
			Status int         `json:"status"`
			Reason interface{} `json:"reason"`
		} `json:"failures"`
	} `json:"_shards"`
}

type ClusterVersion struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
	Version     struct {
		Number        string `json:"number"`
		LuceneVersion string `json:"lucene_version"`
	} `json:"version"`
}

type ClusterHealth struct {
	Name   string `json:"cluster_name"`
	Status string `json:"status"`
}

// IndexDocument used to construct indexing document
type IndexDocument struct {
	Index     string                   `json:"_index,omitempty"`
	Type      string                   `json:"_type"`
	ID        interface{}              `json:"_id,omitempty"`
	Routing   string                   `json:"_routing"`
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

// InsertResponse is a index response object
type InsertResponse struct {
	Result  string `json:"result"`
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	ID      string `json:"_id"`
	Version int    `json:"_version"`
}

// GetResponse is a get response object
type GetResponse struct {
	Found   bool                   `json:"found"`
	Index   string                 `json:"_index"`
	Type    string                 `json:"_type"`
	ID      string                 `json:"_id"`
	Version int                    `json:"_version"`
	Source  map[string]interface{} `json:"_source"`
}

// DeleteResponse is a delete response object
type DeleteResponse struct {
	Result  string `json:"result"`
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	ID      string `json:"_id"`
	Version int    `json:"_version"`
}

// CountResponse is a count response object
type CountResponse struct {
	Count int `json:"count"`
}

// SearchResponse is a count response object
type SearchResponse struct {
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

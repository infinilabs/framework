package elastic

import (
	"infini.sh/framework/core/util"
	"time"
)

type SearchTemplate struct {
	ID string   `json:"-" index:"id"`
	Name string `json:"name" elastic_mapping:"name:{type:text}"`
	Source string `json:"source" elastic_mapping:"source:{type:text}"`
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
	Updated     time.Time `json:"updated,omitempty" elastic_mapping:"updated:{type:date}"`
}

type SearchTemplateHistory struct {
	ID string `json:"-" index:"id"`
	TemplateID string `json:"template_id" elastic_mapping:"template_id:{type:keyword}"`
	Action string `json:"action" elastic_mapping:"action:{type:keyword}"`
	Content map[string]interface{} `json:"content,omitempty" elastic_mapping:"content:{type:object}"`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
}

type AliasAction map[string]AliasActionBody

type AliasActionBody struct{
	Index string `json:"index,omitempty"`
	Alias string `json:"alias"`
	Indices []string `json:"indices,omitempty"`
	Filter map[string]interface{} `json:"filter,omitempty"`
	Routing string `json:"routing,omitempty"`
	SearchRouting string `json:"search_routing,omitempty"`
	IndexRouting string `json:"index_routing,omitempty"`
	IsWriteIndex bool `json:"is_write_index,omitempty"`
}

type AliasRequest struct{
	Actions []AliasAction `json:"actions"`
}

type TraceTemplate struct {
	ID string   `json:"-" index:"id"`
	Name string `json:"name" elastic_mapping:"name:{type:text}"`
	MetaIndex string `json:"meta_index" elastic_mapping:"meta_index:{type:keyword}"`
	TraceField string `json:"trace_field" elastic_mapping:"trace_field:{type:keyword}"`
	TimestampField string `json:"timestamp_field" elastic_mapping:"timestamp_field:{type:keyword}"`
	AggField string `json:"agg_field" elastic_mapping:"agg_field:{type:keyword}"`
	Description string `json:"description" elastic_mapping:"description:{type:text}"`
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
	Updated     time.Time `json:"updated,omitempty" elastic_mapping:"updated:{type:date}"`
}

type SearchAggParam struct {
	Field string `json:"field"`
	TermsAggParams util.MapStr `json:"params"`
}

func BuildSearchTermAggregations(params []SearchAggParam) util.MapStr {
	var aggregations = util.MapStr{}
	for _, param := range params {
		if param.TermsAggParams["field"] == nil {
			param.TermsAggParams["field"] = param.Field
		}
		aggregations[param.Field] = util.MapStr{
			"terms": param.TermsAggParams,
		}
	}
	return aggregations
}

type SearchHighlightParam struct {
	Fields []string `json:"fields"`
	FragmentSize int `json:"fragment_size"`
	NumberOfFragment int `json:"number_of_fragment"`
}
func BuildSearchHighlight(highlightParam *SearchHighlightParam) util.MapStr{
	if highlightParam == nil {
		return util.MapStr{}
	}
	esFields := util.MapStr{}
	for _, field := range highlightParam.Fields {
		esFields[field] = util.MapStr{}
	}
	return util.MapStr{
		"fields": esFields,
		"fragment_size": highlightParam.FragmentSize,
		"number_of_fragments": highlightParam.NumberOfFragment,
	}
}

type SearchFilterParam map[string][]string
func BuildSearchTermFilter(filterParam SearchFilterParam) []util.MapStr{
	var filter []util.MapStr
	if filterParam == nil {
		return filter
	}
	for k, v := range filterParam {
		terms := make([]interface{},0, len(v))
		for _, vitem := range v {
			terms = append(terms, util.MapStr{
				"term": util.MapStr{
					k: vitem,
				},
			})
		}
		filter = append(filter, util.MapStr{
			"bool": util.MapStr{
				"minimum_should_match": 1,
				"should": terms,
			},
		})
	}
	return filter
}

func GetDateHistogramIntervalField(version string, bucketSize string) (string, error){
	cr, err := util.VersionCompare(version, "7.2")
	if err != nil {
		return "", err
	}
	if cr > -1 {
		return "fixed_interval", nil
	}
	return "interval", nil
}
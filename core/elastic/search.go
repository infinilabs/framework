package elastic

import "time"

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
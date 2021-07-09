package elastic

import "time"

type IndexPatternRequest struct {
	Attributes IndexPattern `json:"attributes"`
}

type IndexPattern struct {
	ID string `json:"-" index:"id"`
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	Title string `json:"title" elastic_mapping:"title:{type:keyword}"`
	TimeFieldName string `json:"timeFieldName" elastic_mapping:"timeFieldName:{type:keyword}"`
	Fields string `json:"fields" elastic_mapping:"fields:{type:text}"`
	FieldFormatMap string `json:"fieldFormatMap" elastic_mapping:"fields:{type:text}`
	UpdatedAt     time.Time `json:"updated_at,omitempty" elastic_mapping:"updated_at:{type:date}"`
}

type AAIR_Alias struct {
	Name string `json:"name"`
	Indices []string `json:"indices"`
}
type AAIR_Indices struct {
	Name string `json:"name"`
	Attributes []string `json:"attributes"`
	Aliases []string `json:"aliases,omitempty"`
}
type AliasAndIndicesResponse struct {
	Aliases []AAIR_Alias `json:"aliases"`
	Indices []AAIR_Indices `json:"indices"`
}

type FieldCap struct {
	Type string `json:"type"`
	Searchable bool `json:"searchable"`
	Aggregatable bool `json:"aggregatable"`
	Indices []string `json:"indices"`
}

type FieldCapsResponse struct {
	Indices []string `json:"indices"`
	Fields map[string] map[string]FieldCap `json:"fields"`
}

type Setting struct {
	ID string `json:"-" index:"id"`
	ClusterID string `json:"cluster_id" elastic_mapping:"cluster_id:{type:keyword}"`
	Key string `json:"key" elastic_mapping:"key:{type:keyword}"`
	Value string `json:"value" elastic_mapping:"value:{type:keyword}"`
	UpdatedAt     time.Time `json:"updated_at,omitempty" elastic_mapping:"updated_at:{type:date}"`
}
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

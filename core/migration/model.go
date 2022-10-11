/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package migration

type ElasticDataConfig struct {
	Cluster struct {
		Source ClusterInfo `json:"source"`
		Target ClusterInfo `json:"target"`
	} `json:"cluster"`
	Indices []IndexConfig `json:"indices"`
	Settings struct {
		ParallelIndices      int `json:"parallel_indices"`
		ParallelTaskPerIndex int `json:"parallel_task_per_index"`
		ScrollSize           struct {
			Documents int    `json:"documents"`
			Timeout   string `json:"timeout"`
		} `json:"scroll_size"`
		BulkSize struct {
			Documents        int `json:"documents"`
			StoreSizeInMB int `json:"store_size_in_mb"`
		} `json:"bulk_size"`
		ExecInterval []struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"exec_interval"`
	} `json:"settings"`
	Creator struct {
		Name string `json:"name"`
		Id   string `json:"id"`
	} `json:"creator"`
}

type IndexConfig struct {
	Source IndexInfo `json:"source"`
	Target IndexInfo `json:"target"`
	RawFilter interface{} `json:"raw_filter"`
	IndexRename map[string]interface{} `json:"index_rename"`
	TypeRename map[string]interface{} `json:"type_rename"`
	Partition *IndexPartition `json:"partition,omitempty"`
	ID string `json:"id,omitempty"`
	Percent float64 `json:"percent,omitempty"`
	ErrorPartitions int `json:"error_partitions,omitempty"`
}
type IndexPartition struct {
	FieldType string `json:"field_type"`
	FieldName string `json:"field_name"`
	Step      interface{}    `json:"step"`
}

type IndexInfo struct {
	Name             string `json:"name"`
	Docs        int64    `json:"docs"`
	StoreSizeInBytes int    `json:"store_size_in_bytes"`
}

type ClusterInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}
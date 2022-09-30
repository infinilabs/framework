/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package migration

type Cluster struct {
	Cluster struct {
		Source ClusterInfo `json:"source"`
		Target ClusterInfo `json:"target"`
	} `json:"cluster"`
	MigrateIndices []IndexConfig `json:"migrate_indices"`
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
	SourceIndex IndexInfo `json:"source_index"`
	TargetIndex IndexInfo `json:"target_index"`
	RawFilter interface{} `json:"raw_filter"`
	IndexRename map[string]interface{} `json:"index_rename"`
	TypeRename map[string]interface{} `json:"type_rename"`
	Partition *IndexPartition `json:"partition,omitempty"`
}
type IndexPartition struct {
	FieldType string `json:"field_type"`
	FieldName string `json:"field_name"`
	Step      interface{}    `json:"step"`
}

type IndexInfo struct {
	Name             string `json:"name"`
	Documents        int    `json:"documents"`
	StoreSizeInBytes int    `json:"store_size_in_bytes"`
}

type ClusterInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}
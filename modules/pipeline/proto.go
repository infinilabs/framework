package pipeline

import "infini.sh/framework/core/pipeline"

type GetPipelinesResponse map[string]*PipelineStatus

type CreatePipelineRequest struct {
	pipeline.PipelineConfigV2
	Processors []map[string]interface{} `json:"processor"`
}

type SearchPipelinesRequest struct {
	Ids []string `json:"ids"`
}

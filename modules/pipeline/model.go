package pipeline

import (
	"time"

	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

type PipelineStatus struct {
	State      pipeline.RunningState      `json:"state"`
	CreateTime time.Time                  `json:"create_time"`
	StartTime  *time.Time                 `json:"start_time"`
	EndTime    *time.Time                 `json:"end_time"`
	Context    util.MapStr                `json:"context"`
	Config     *pipeline.PipelineConfigV2 `json:"config"`
	Processors []map[string]interface{}   `json:"processor"`
}

package pipeline

import (
	"time"

	"infini.sh/framework/core/pipeline"
)

type PipelineStatus struct {
	State     pipeline.RunningState `json:"state"`
	StartTime *time.Time            `json:"start_time"`
	EndTime   *time.Time            `json:"end_time"`
}

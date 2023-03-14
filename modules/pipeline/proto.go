package pipeline

type GetPipelinesResponse map[string]PipelineStatus

type GetPipelineResponse PipelineStatus

type CreatePipelineRequest struct {
	Name           string                   `json:"name"`
	AutoStart      bool                     `json:"auto_start"`
	KeepRunning    bool                     `json:"keep_running"`
	RetryDelayInMs int                      `json:"retry_delay_in_ms"`
	Processors     []map[string]interface{} `json:"processor"`
}

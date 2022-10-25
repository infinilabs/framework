package pipeline

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

type PipelineConfigV2 struct {
	Name           string `config:"name" json:"name,omitempty"`
	AutoStart      bool   `config:"auto_start" json:"auto_start"`
	KeepRunning    bool   `config:"keep_running" json:"keep_running"`
	RetryDelayInMs int    `config:"retry_delay_in_ms" json:"retry_delay_in_ms"`
	Processors []*config.Config `config:"processor" json:"processor,omitempty"`
}


func (this PipelineConfigV2) Equals(target PipelineConfigV2) bool {
	if this.Name != target.Name ||
		this.AutoStart != target.AutoStart ||
		this.KeepRunning != target.KeepRunning ||
		this.RetryDelayInMs != target.RetryDelayInMs ||
		util.MustToJSON(this.Processors) != util.MustToJSON(target.Processors) {
		return false
	}
	return true
}
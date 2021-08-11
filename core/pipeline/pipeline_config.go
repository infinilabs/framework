package pipeline

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"time"
)

// ProcessorConfig configs for each joint
type ProcessorConfig struct {
	Name       string                 `json:"joint" config:"joint"`                     //the joint name
	Parameters map[string]interface{} `json:"parameters,omitempty" config:"parameters"` //kv parameters for this joint
	Enabled    bool                   `json:"enabled" config:"enabled"`
}

// PipelineConfig config for each pipeline, a pipeline may have more than one processors
type PipelineConfig struct {
	ID   string `gorm:"not null;unique;primary_key" json:"id,omitempty" index:"id"`
	Name string `json:"name,omitempty" config:"name"`
	Enabled bool `json:"enabled,omitempty" config:"enabled"`
	MaxGoRoutine int `config:"max_go_routine"`

	//Speed Control
	ThresholdInMs int `config:"threshold_in_ms"`

	//Timeout Control
	TimeoutInMs int `config:"timeout_in_ms"`

	InputQueue string `config:"input_queue"`
	Schedule string `config:"schedule"`

	//TODO remove
	StartProcessor *ProcessorConfig   `json:"start,omitempty" config:"start"`
	Processors     []*ProcessorConfig `json:"process,omitempty" config:"process"`
	EndProcessor   *ProcessorConfig   `json:"end,omitempty" config:"end"`
	ErrorProcessor *ProcessorConfig   `json:"error,omitempty" config:"error"`

	Input   *ProcessorConfig   `json:"input,omitempty" config:"input"`
	Filters []*ProcessorConfig `json:"filters,omitempty" config:"filters"`
	Output  *ProcessorConfig   `json:"output,omitempty" config:"output"`

	Created time.Time `json:"created,omitempty"`
	Updated time.Time `json:"updated,omitempty"`
	Tags    []string  `json:"tags,omitempty" config:"tags"`
}

var m map[string]PipelineConfig

func GetStaticPipelineConfig(pipelineID string) PipelineConfig {
	t1:= GetPipelineConfigs()
	v, ok := t1[pipelineID]
	if !ok {
		panic("pipeline config not found")
	}
	return v
}

func GetPipelineConfigs() map[string]PipelineConfig {
	if m == nil {
		m = map[string]PipelineConfig{}
		var pipelines []PipelineConfig
		exist, err := env.ParseConfig("pipelines", &pipelines)
		if exist&&err != nil {
			panic(err)
		}
		if exist {
			for _, v := range pipelines {

				if v.MaxGoRoutine<=0{
					v.MaxGoRoutine=1
				}
				if v.TimeoutInMs<=0{
					v.TimeoutInMs=5000
				}

				if v.ID == "" {
					if v.Name == "" {
						panic(errors.Errorf("invalid pipeline config, %v", v))
					}
					v.ID = v.Name
				}
				m[v.ID] = v
			}
		}
	}
	return m
}

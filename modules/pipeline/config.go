/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipeline

import "infini.sh/framework/core/pipeline"

// PipeRunnerConfig defines module related configs
type PipeRunnerConfig struct {
	Name string `json:"name,omitempty" config:"name"`

	Enabled bool `json:"enabled,omitempty" config:"enabled"`

	MaxGoRoutine int `config:"max_go_routine"`

	//Speed Control
	ThresholdInMs int `config:"threshold_in_ms"`

	//Timeout Control
	TimeoutInMs int `config:"timeout_in_ms"`

	PipelineID string `config:"pipeline_id"`

	pipelineConfig pipeline.PipelineConfig

	InputQueue string `config:"input_queue"`

	Schedule string `config:"schedule"`
}

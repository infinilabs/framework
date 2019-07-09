/*
Copyright 2016 Medcl (m AT medcl.net)

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

import (
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/errors"
	"time"
)

// JointConfig configs for each joint
type JointConfig struct {
	JointName  string                 `json:"joint" config:"joint"`                     //the joint name
	Parameters map[string]interface{} `json:"parameters,omitempty" config:"parameters"` //kv parameters for this joint
	Enabled    bool                   `json:"enabled" config:"enabled"`
}

// PipelineConfig config for each pipeline, a pipeline may have more than one joints
type PipelineConfig struct {
	ID            string         `json:"id,omitempty" index:"id"`
	Name          string         `json:"name,omitempty" config:"name"`
	StartJoint    *JointConfig   `json:"start,omitempty" config:"start"`
	ProcessJoints []*JointConfig `json:"process,omitempty" config:"process"`
	EndJoint      *JointConfig   `json:"end,omitempty" config:"end"`
	ErrorJoint    *JointConfig   `json:"error,omitempty" config:"error"`
	Created       time.Time      `json:"created,omitempty"`
	Updated       time.Time      `json:"updated,omitempty"`
	Tags          []string       `json:"tags,omitempty" config:"tags"`
}

var m map[string]PipelineConfig

func GetStaticPipelineConfig(pipelineID string) PipelineConfig {

	if m == nil {
		m = map[string]PipelineConfig{}
		var pipelines []PipelineConfig
		exist, err := env.ParseConfig("pipelines", &pipelines)
		if err != nil {
			panic(err)
		}
		if exist {
			for _, v := range pipelines {
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
	v, ok := m[pipelineID]
	if !ok {
		panic("pipeline config not found")
	}
	return v
}

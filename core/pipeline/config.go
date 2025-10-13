// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package pipeline

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
)

type PipelineConfigV2 struct {
	Name           string `config:"name" json:"name,omitempty"`
	Enabled        *bool  `config:"enabled" json:"enabled,omitempty"`
	Singleton      bool   `config:"singleton" json:"singleton"`
	AutoStart      bool   `config:"auto_start" json:"auto_start"`
	KeepRunning    bool   `config:"keep_running" json:"keep_running"`
	RetryDelayInMs int    `config:"retry_delay_in_ms" json:"retry_delay_in_ms"`
	MaxRunningInMs int64  `config:"max_running_in_ms" json:"max_running_in_ms"`
	Logging        struct {
		Enabled bool `config:"enabled" json:"enabled"`
	} `config:"logging" json:"logging"`
	Processors []map[string]interface{} `config:"processor" json:"processor"`
	Labels     map[string]interface{}   `config:"labels" json:"labels"`

	Transient bool `config:"-" json:"transient"`
}

func (this PipelineConfigV2) GetProcessorsConfig() ([]*config.Config, error) {
	var processors []*config.Config
	for _, processorDict := range this.Processors {
		processor, err := ucfg.NewFrom(processorDict)
		if err != nil {
			return nil, errors.Errorf("failed to parse processor config: %v", err)
		}
		processors = append(processors, config.FromConfig(processor))
	}
	return processors, nil
}

func (this PipelineConfigV2) Equals(target PipelineConfigV2) bool {
	if this.Name != target.Name ||
		this.AutoStart != target.AutoStart ||
		this.KeepRunning != target.KeepRunning ||
		this.RetryDelayInMs != target.RetryDelayInMs ||
		this.Logging.Enabled != target.Logging.Enabled ||
		!this.ProcessorsEquals(target) {
		return false
	}

	if this.Enabled != nil && target.Enabled != nil && *this.Enabled != *target.Enabled {
		return false
	}

	return true
}

func (this PipelineConfigV2) ProcessorsEquals(target PipelineConfigV2) bool {

	srcCfg, err := this.GetProcessorsConfig()
	if err != nil {
		panic(err)
	}
	targetCfg, err := this.GetProcessorsConfig()
	if err != nil {
		panic(err)
	}

	if srcCfg == nil || targetCfg == nil {
		panic(errors.Errorf("invalid pipeline config, src is nil:%v, target is nil:%v", srcCfg == nil, targetCfg == nil))
	}

	if len(srcCfg) != len(targetCfg) {
		return false
	}
	var length = len(srcCfg)
	for i := 0; i < length; i++ {
		srcM := map[string]interface{}{}
		err := srcCfg[i].Unpack(srcM)
		if err != nil {
			panic(err)
		}
		dstM := map[string]interface{}{}
		err = targetCfg[i].Unpack(dstM)
		if err != nil {
			panic(err)
		}
		clog, err := util.DiffTwoObject(srcM, dstM)
		if err != nil {
			panic(err)
		}
		if len(clog) > 0 {
			return false
		}
	}
	return true
}

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
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
)

// PipelineConfig is the declarative definition of a pipeline: a named,
// ordered chain of processors plus the policies that govern how the framework
// runs it.
//
// It is pure configuration. Once loaded into memory and instantiated, a
// PipelineConfigV2 becomes a PipelineTask — the live runtime form, consisting
// of a compiled processor chain, an execution context, and a managed
// goroutine.
type PipelineConfigV2 struct {
	orm.ORMObjectBase

	// Human-readable identifier for this pipeline.
	// Required when creating a pipeline via the API.
	Name string `config:"name" json:"name,omitempty"`

	// A disabled pipeline is treated as if it does not exist. Nil means
	// "use the module default".
	Enabled *bool `config:"enabled" json:"enabled,omitempty"`

	// Singleton ensures that at most one instance of this pipeline runs
	// across the entire cluster (not just within a single node). It is
	// enforced jointly by two things:
	//   1. Setting this field to true.
	//   2. MaxRunningInMs — used as the TTL of the distributed lock that
	//      guards the singleton slot. The lock prevents other nodes from
	//      starting a competing instance while one is already running.
	Singleton bool `config:"singleton" json:"singleton"`

	// AutoStart controls the initial running state once the pipeline is
	// loaded into memory. When true the pipeline begins executing
	// immediately; when false it stays idle until an explicit start request
	// arrives.
	AutoStart bool `config:"auto_start" json:"auto_start"`

	// KeepRunning marks the pipeline as long-lived: after the processor
	// chain finishes one pass it loops and runs again. When false the
	// pipeline executes once and exits.
	KeepRunning bool `config:"keep_running" json:"keep_running"`

	// RetryDelayInMs is the pause between successive runs of the processor
	// chain in the keep-running loop. Applies whether the previous run
	// finished successfully or failed.
	//
	// If it is smaller than or equal to 0, use the default value 1000.
	RetryDelayInMs int `config:"retry_delay_in_ms" json:"retry_delay_in_ms"`

	// MaxRunningInMs is the TTL of the cross-node singleton lock: the
	// maximum time this node is allowed to hold the lock for one run. Once
	// the TTL elapses the lock is considered released and another node may
	// claim the singleton slot. The running goroutine itself is unaffected
	// — it is NOT canceled or killed when the TTL expires.
	//
	// Only consulted when Singleton is true. When the value is zero or
	// negative it falls back to a built-in default of 60000 ms (1 minute).
	//
	// NOTE: the name of this field is misleading — it sounds like a run
	// timeout but is actually a lock TTL. It is kept for backwards
	// compatibility.
	MaxRunningInMs int64 `config:"max_running_in_ms" json:"max_running_in_ms"`

	Logging struct {
		Enabled bool `config:"enabled" json:"enabled"`
	} `config:"logging" json:"logging"`

	// Processors is the ordered chain of processor definitions that makes
	// up the actual work the pipeline performs. Each entry is a single
	// processor config keyed by processor type.
	Processors []map[string]interface{} `config:"processor" json:"processor"`

	// Labels are arbitrary metadata attached to the pipeline. 
	Labels map[string]interface{} `config:"labels" json:"labels"`

	// Transient, if false, marks that this pipeline config shpuld be persisted.
	//
	// In the current implementation, only the pipeline configs created with the 
	// `POST /pipeline` API will set this to true.
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

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
	"fmt"
	log "github.com/cihub/seelog"
	config2 "infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"strings"
)

type DAGProcessor struct {
	cfg DAGConfig
	dag *Dag
}

func getProcessors(cfg []*config2.Config) ([]Processor, error) {
	newProcessors := []Processor{}
	for _, procConfig := range cfg {

		if len(procConfig.GetFields()) != 1 {
			return nil, errors.Errorf("each processor must have exactly one "+
				"action, but found %d actions (%v)",
				len(procConfig.GetFields()),
				strings.Join(procConfig.GetFields(), ","))
		}

		actionName := procConfig.GetFields()[0]
		actionCfg, err := procConfig.Child(actionName, -1)
		if err != nil {
			return nil, err
		}

		//log.Info("get dag plugin:",actionName,actionCfg)

		gen, exists := registry.processorReg[actionName]
		if !exists {
			var validActions []string
			for k := range registry.processorReg {
				validActions = append(validActions, k)

			}
			return nil, errors.Errorf("the processor %s does not exist. valid processors: %v", actionName, strings.Join(validActions, ", "))
		}

		//actionCfg.PrintDebugf("Configure processor action '%v' with:", actionName)
		constructor := gen.ProcessorPlugin()
		plugin, err := constructor(actionCfg)
		if err != nil {
			return nil, err
		}
		//fmt.Println("init processor:",plugin.Name())
		newProcessors = append(newProcessors, plugin)
	}
	return newProcessors, nil
}

func NewDAGProcessor(c *config2.Config) (Processor, error) {
	cfg := DAGConfig{}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of dag processor: %s", err)
	}

	processor := DAGProcessor{
		cfg: cfg,
	}

	//log.Info("init dag processor")

	if len(cfg.ParallelProcessors) == 0 {
		return nil, errors.New("parallel is not set")
	}

	processor.dag = NewDAG(cfg.Mode)
	var dsl *spawnsResult
	p, err := getProcessors(cfg.ParallelProcessors)
	if err != nil {
		panic(err)
	}
	if len(p) > 0 {
		dsl = processor.dag.Spawns(p...)
	}

	p, err = getProcessors(cfg.AfterJoinAllProcessors)
	if err != nil {
		panic(err)
	}
	if len(p) > 0 {
		dsl.Join().Pipeline(p...)
	}

	p, err = getProcessors(cfg.AfterAnyProcessors)
	if err != nil {
		panic(err)
	}
	if len(p) > 0 {
		dsl.OnComplete(p...)
	}

	return &processor, nil
}

func (this DAGProcessor) Name() string {
	return "dag"
}

type DAGConfig struct {
	Enabled                 bool              `config:"enabled"`
	Mode                    string            `config:"mode"`
	ParallelProcessors      []*config2.Config `config:"parallel"`
	FirstFinishedProcessors []*config2.Config `config:"first"`
	AfterJoinAllProcessors  []*config2.Config `config:"join"`
	AfterAnyProcessors      []*config2.Config `config:"end"`
}

func (this DAGProcessor) Process(c *Context) error {

	this.dag.Run(c)
	log.Debug("dag finished.")
	return nil
}

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
	"infini.sh/framework/core/global"
	"runtime"
	log "github.com/cihub/seelog"
)

// Dag represents directed acyclic graph
type Dag struct {
	mode string
	jobs []*Job
}

// NewPipeline creates new DAG
func NewDAG(mode string) *Dag {
	return &Dag{
		mode: mode,
		jobs: make([]*Job, 0),
	}
}

func (dag *Dag) lastJob() *Job {
	jobsCount := len(dag.jobs)
	if jobsCount == 0 {
		return nil
	}

	return dag.jobs[jobsCount-1]
}

func (dag *Dag) Parse(dsl string) *Dag {
	return dag
}

// Run starts the tasks
// It will block until all functions are done
func (dag *Dag) Run(ctx *Context) {

	//fmt.Println("total jobs:",len(dag.jobs))
	for _, job := range dag.jobs {
		run(job,ctx)
	}

}

// RunAsync executes Run on another goroutine
func (dag *Dag) RunAsync(ctx *Context,onComplete func()) {
	go func() {
		defer func() {
			if !global.Env().IsDebug {
				if r := recover(); r != nil {
					var v string
					switch r.(type) {
					case error:
						v = r.(error).Error()
					case runtime.Error:
						v = r.(runtime.Error).Error()
					case string:
						v = r.(string)
					}
					log.Error(v)
				}
			}
		}()

		dag.Run(ctx)

		if onComplete != nil {
			onComplete()
		}

	}()
}

// Pipeline executes tasks sequentially
func (dag *Dag) Pipeline(tasks ...Processor) *pipelineResult {

	job := &Job{
		tasks:      make([]Processor, len(tasks)),
		sequential: true,
	}

	for i, task := range tasks {
		job.tasks[i] = task
	}

	dag.jobs = append(dag.jobs, job)

	return &pipelineResult{
		dag,
	}
}

func (dag *Dag) Spawns(tasks ...Processor) *spawnsResult {

	job := &Job{
		tasks:      make([]Processor, len(tasks)),
		sequential: false,
		mode:dag.mode,
	}

	for i, task := range tasks {
		job.tasks[i] = task
	}

	dag.jobs = append(dag.jobs, job)

	return &spawnsResult{
		dag,
	}
}

type anyResult struct {
	dag *Dag
}

func (dag *Dag) Any(tasks ...func()) *anyResult {
	return &anyResult{
		dag,
	}
}

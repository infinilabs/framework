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
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/global"
	"runtime"
)

func runAsync(job *Job, ctx *Context) {

	if job.onFailure != nil {
		defer job.onFailure.Process(ctx)
	}

	var ch chan bool
	ch = make(chan bool, len(job.tasks)*2)

	waitSignal := len(job.tasks)
	if job.mode == "first_win" {
		waitSignal = 1
	} else {
		ch = make(chan bool, len(job.tasks))
	}

	for _, task := range job.tasks {
		go func(task Processor) {
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
						log.Error(r, v)
					}
				}
			}()

			task.Process(ctx)
			ch <- true
		}(task)
	}

	for i := 0; i < waitSignal; i++ {
		_ = <-ch
	}

	if job.onComplete != nil {
		for _, v := range job.onComplete {
			v.Process(ctx)
		}
	}
}

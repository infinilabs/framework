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

package task

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"net/http"
	"time"
)

type TaskModule struct {
	api.Handler
	pool                    *pipeline.Pool
	TimeZone                string `config:"time_zone" json:"time_zone,omitempty"`
	MaxConcurrentNumOfTasks int    `config:"max_concurrent_tasks" json:"max_concurrent_tasks,omitempty"`
}

func (module *TaskModule) Name() string {
	return "task"
}

func (module *TaskModule) Setup() {

	module.TimeZone = "UTC"
	module.MaxConcurrentNumOfTasks = 100
	ok, err := env.ParseConfig("task", &module)
	if ok && err != nil  &&global.Env().SystemConfig.Configs.PanicOnConfigError{
		panic(err)
	}

	tz, err := time.LoadLocation(module.TimeZone)
	if tz == nil {
		tz = time.UTC
	}
	module.pool, _ = pipeline.NewPoolWithTag("tasks",module.MaxConcurrentNumOfTasks)
	global.RegisterShutdownCallback(func() {
		pipeline.Release()
	})

	api.HandleAPIMethod(api.GET, "/tasks/", module.GetTaskList)
	api.HandleAPIMethod(api.POST, "/task/:id/_start", module.StartTask)
	api.HandleAPIMethod(api.POST, "/task/:id/_stop", module.StopTask)
	api.HandleAPIMethod(api.DELETE, "/task/:id", module.DeleteTask)

}

func (module *TaskModule) Start() error {
	task.RunTasks()
	return nil
}
func (module *TaskModule) Stop() error {
	task.StopTasks()
	return nil
}

func (module *TaskModule) GetTaskList(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	r := map[string]interface{}{}
	task.Tasks.Range(func(key, value any) bool {
		r[util.ToString(key)] = value
		return true
	})
	module.WriteJSON(w, r, 200)
}

func (module *TaskModule) StartTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	task.StartTask(ps.ByName("id"))
	module.WriteAckOKJSON(w)
}

func (module *TaskModule) StopTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	task.StopTask(ps.ByName("id"))
	module.WriteAckOKJSON(w)
}

func (module *TaskModule) DeleteTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	task.DeleteTask(ps.ByName("id"))
	module.WriteAckOKJSON(w)
}

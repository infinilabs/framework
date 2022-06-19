package task

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/task/ants"
	"infini.sh/framework/core/util"
	"net/http"
	"time"
)

type TaskModule struct {
	api.Handler
	pool                    *ants.Pool
	TimeZone                string `config:"time_zone" json:"time_zone,omitempty"`
	MaxConcurrentNumOfTasks int    `config:"max_concurrent_tasks" json:"max_concurrent_tasks,omitempty"`
}

func (module *TaskModule) Name() string {
	return "task"
}

func (module *TaskModule) Setup(cfg *config.Config) {

	module.TimeZone = "UTC"
	module.MaxConcurrentNumOfTasks = 100
	ok, err := env.ParseConfig("task", &module)
	if ok && err != nil {
		panic(err)
	}

	tz, err := time.LoadLocation(module.TimeZone)
	if tz == nil {
		tz = time.UTC
	}
	module.pool, _ = ants.NewPool(module.MaxConcurrentNumOfTasks)
	global.RegisterShutdownCallback(func() {
		ants.Release()
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

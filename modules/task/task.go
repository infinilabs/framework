package task

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/task/ants"
	"time"
)

type TaskModule struct {
	pool                    *ants.Pool
	TimeZone string `config:"time_zone" json:"time_zone,omitempty"`
	MaxConcurrentNumOfTasks int `config:"max_concurrent_tasks" json:"max_concurrent_tasks,omitempty"`
}

func (module *TaskModule) Name() string {
	return "task"
}

func (module *TaskModule) Setup(cfg *config.Config) {

	module.TimeZone="UTC"
	module.MaxConcurrentNumOfTasks=100
	ok,err:=env.ParseConfig("task", &module)
	if ok&&err!=nil{
		panic(err)
	}

	tz,err:=time.LoadLocation(module.TimeZone)
	if tz==nil{
		tz=time.UTC
	}
	module.pool, _ = ants.NewPool(module.MaxConcurrentNumOfTasks)
	global.RegisterShutdownCallback(func() {
		ants.Release()
	})

}

func (module *TaskModule) Start() error {
	task.RunTasks()
	return nil
}
func (module *TaskModule) Stop() error {
	task.StopTasks()
	return nil
}

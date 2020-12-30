package task

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/ui"
)

type TaskModule struct {
}

func (module TaskModule) Name() string {
	return "task"
}

func (module TaskModule) Setup(cfg *config.Config) {

	uiConfig := ui.UIConfig{}

	env.ParseConfig("task", &uiConfig)

}

func (module TaskModule) Start() error {
	task.RunTasks()
	return nil
}
func (module TaskModule) Stop() error {
	task.StopTasks()
	return nil
}

package chrono

import (
	"context"
	"infini.sh/framework/core/global"
	"runtime"
	log "github.com/cihub/seelog"
)

type TaskRunner interface {
	Run(task Task)
}

type SimpleTaskRunner struct {
}

func NewDefaultTaskRunner() TaskRunner {
	return NewSimpleTaskRunner()
}

func NewSimpleTaskRunner() *SimpleTaskRunner {
	return &SimpleTaskRunner{}
}

func (runner *SimpleTaskRunner) Run(task Task) {
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

		task(context.Background())
	}()
}

package task

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"runtime"
	"sync"
	"time"
)

var scheduleTasks = []ScheduleTask{}
var taskLock sync.RWMutex

type ScheduleTask struct {
	Description string
	Type        string
	Interval    string
	Task        func()
}

func RegisterScheduleTask(task ScheduleTask) {
	taskLock.Lock()
	scheduleTasks = append(scheduleTasks, task)
	taskLock.Unlock()
}

var quit = make(chan struct{})

func RunTasks() {
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
					log.Error("error on run tasks,", v)
				}
			}
		}()
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				taskLock.RLock()
				for _, task := range scheduleTasks {
					if global.Env().IsDebug {
						log.Tracef("task: %s, %v, %v", task.Description, task.Type, task.Interval)
					}
					go func(task ScheduleTask) {
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
									log.Error("error on run task,", v)
								}
							}
						}()
						task.Task()
					}(task)
				}
				taskLock.RUnlock()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

}

func StopTasks() {
	close(quit)
}

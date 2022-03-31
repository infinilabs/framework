package task

import (
	"context"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task/chrono"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

var scheduleTasks = []ScheduleTask{}
var taskLock sync.RWMutex

type ScheduleTask struct {
	Description string
	Type        string
	Interval    string
	Task        func(ctx context.Context)
}

func RegisterScheduleTask(task ScheduleTask) {
	taskLock.Lock()
	scheduleTasks = append(scheduleTasks, task)
	taskLock.Unlock()
}

var quit = make(chan struct{})
var taskScheduler = chrono.NewDefaultTaskScheduler()

func RunTasks() {

	var defaultInterval=time.Duration(10) * time.Second
	taskLock.RLock()
	defer taskLock.RUnlock()

	for _, task := range scheduleTasks {
		if global.Env().IsDebug{
			log.Debug("scheduled task:",task.Type,",",task.Interval,",",task.Description)
		}
		_, err := taskScheduler.ScheduleAtFixedRate(task.Task, util.GetDurationOrDefault(task.Interval,defaultInterval))
		if err != nil {
			log.Error("failed to scheduled task:",task.Type,",",task.Interval,",",task.Description)
		}
	}
}

func StopTasks() {
	shutdownChannel := taskScheduler.Shutdown()
	<- shutdownChannel

	close(quit)
}

func ReceiveTask(task func())  {

}


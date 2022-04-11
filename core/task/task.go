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

var Tasks = sync.Map{}

type ScheduleTask struct {

	ID string  `config:"id" json:"id,omitempty"`
	Description string  `config:"description" json:"description,omitempty"`
	Type        string  `config:"type" json:"type,omitempty"`
	Interval    string  `config:"interval" json:"interval,omitempty"`
	Crontab    string  `config:"crontab" json:"crontab,omitempty"`

	Task        func(ctx context.Context)
	taskItem chrono.ScheduledTask
}

func RegisterScheduleTask(task ScheduleTask) {
	if task.ID==""{
		task.ID=util.GetUUID()
	}
	Tasks.Store(task.ID,task)
}

var quit = make(chan struct{})
var taskScheduler = chrono.NewDefaultTaskScheduler()

func RunTasks() {

	var defaultInterval=time.Duration(10) * time.Second
	Tasks.Range(func(key, value any) bool {
		task,ok:=value.(ScheduleTask)
		//log.Error(task,ok)
		if ok{
			if global.Env().IsDebug{
				log.Debug("scheduled task:",task.ID,",",task.Type,",",task.Interval,",",task.Crontab,",",task.Description)
			}

			switch task.Type {
			case "interval":
				task1, err := taskScheduler.ScheduleAtFixedRate(task.Task, util.GetDurationOrDefault(task.Interval,defaultInterval))
				if err != nil {
					log.Error("failed to scheduled interval task:",task.Type,",",task.Interval,",",task.Description)
				}
				task.taskItem=task1
				break
			case "crontab":
				task1, err := taskScheduler.ScheduleWithCron(task.Task, task.Crontab)
				if err != nil {
					log.Error("failed to scheduled crontab task:",task.Type,",",task.Interval,",",task.Description)
				}
				task.taskItem=task1
				break
			}
		}
		return true
	})
}

func StopTask(id string) {
	task,ok:=Tasks.Load(id)
	if ok{
		item,ok:=task.(ScheduleTask)
		if ok{
			item.taskItem.Cancel()
		}
	}
}

func StopTasks() {
	shutdownChannel := taskScheduler.Shutdown()
	<- shutdownChannel

	close(quit)
}

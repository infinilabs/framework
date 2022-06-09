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
	ID          string `config:"id" json:"id,omitempty"`
	Description string `config:"description" json:"description,omitempty"`
	Type        string `config:"type" json:"type,omitempty"`
	Interval    string `config:"interval" json:"interval,omitempty"`
	Crontab     string `config:"crontab" json:"crontab,omitempty"`

	Task     func(ctx context.Context) `config:"-" json:"-"`
	taskItem chrono.ScheduledTask
}

const Interval = "interval"
const Crontab = "crontab"

func RegisterScheduleTask(task ScheduleTask) {
	if task.ID == "" {
		task.ID = util.GetUUID()
	}

	if task.Type == "" && task.Interval != "" {
		task.Type = Interval
	} else if task.Type == "" && task.Crontab != "" {
		task.Type = Crontab
	}

	_, ok := Tasks.Load(task.ID)
	if ok {
		StopTask(task.ID)
	}

	Tasks.Store(task.ID, &task)

	//start after register
	if started {
		runTask(&task)
	}

}

var quit = make(chan struct{})
var taskScheduler = chrono.NewDefaultTaskScheduler()
var defaultInterval = time.Duration(10) * time.Second
var started bool
func RunTasks() {
	started = true
	Tasks.Range(func(key, value any) bool {
		task, ok := value.(*ScheduleTask)
		if ok {
			runTask(task)
		}
		return true
	})
}

func runTask(task *ScheduleTask) {
	if global.Env().IsDebug {
		log.Debug("scheduled task:", task.ID, ",", task.Type, ",", task.Interval, ",", task.Crontab, ",", task.Description)
	}

	switch task.Type {
	case Interval:
		task1, err := taskScheduler.ScheduleAtFixedRate(task.Task, util.GetDurationOrDefault(task.Interval, defaultInterval))
		if err != nil {
			log.Error("failed to scheduled interval task:", task.Type, ",", task.Interval, ",", task.Description)
		}
		task.taskItem = task1
		break
	case Crontab:
		task1, err := taskScheduler.ScheduleWithCron(task.Task, task.Crontab)
		if err != nil {
			log.Error("failed to scheduled crontab task:", task.Type, ",", task.Interval, ",", task.Description)
		}
		task.taskItem = task1
		break
	default:
		log.Error("unknown task type:", task)
	}
}

func StartTask(id string) {
	task, ok := Tasks.Load(id)
	if ok {
		item, ok := task.(*ScheduleTask)
		if ok {
			if item != nil {
				runTask(item)
			} else {
				log.Error("invalid task:", item)
			}
		}
	}
}

func StopTask(id string) {
	task, ok := Tasks.Load(id)
	if ok {
		item, ok := task.(*ScheduleTask)
		if ok {
			if item != nil && item.taskItem != nil {
				item.taskItem.Cancel()
			}
		}
	}
}

func StopAllTasks() {

	Tasks.Range(func(key, value any) bool {
		task, ok := value.(*ScheduleTask)
		if ok {
			go StopTask(task.ID)
		}
		return true
	})
}

func DeleteTask(id string) {
	StopTask(id)
	Tasks.Delete(id)
}

func StopTasks() {
	started = false
	StopAllTasks()
	shutdownChannel := taskScheduler.Shutdown()
	<-shutdownChannel

	close(quit)
}

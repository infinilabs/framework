package task

import (
	"context"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task/chrono"
	"infini.sh/framework/core/util"
	"runtime"
	"sync"
	"time"
)

var Tasks = sync.Map{}

type State string

const (
	Pending  State = "PENDING"
	Running  State = "STARTED"
	Canceled       = "CANCELED"
	Finished       = "FINISHED"
)

func RunWithinGroup(groupName string, f func(ctx context.Context) error) (taskID string) {
	return registerTransientTask(groupName,"",f,context.Background())
}

func MustGetString(ctx context.Context, key string) string {
	v := ctx.Value(key)
	if v != nil {
		x, ok := v.(string)
		if ok {
			return x
		}
	}
	panic(errors.Errorf("invalid key: %v", key))
}

func RunWithContext(tag string, f func(ctx context.Context) error, ctxInput context.Context) (taskID string) {
	return registerTransientTask("default",tag,f,ctxInput)
}

func registerTransientTask(group,tag string, f func(ctx context.Context) error, ctxInput context.Context) (taskID string) {
	task := ScheduleTask{}
	task.ID = util.GetUUID()
	task.Group=group
	task.Description = tag
	task.Type = Transient
	task.CreateTime = time.Now()
	task.State = Pending
	task.Ctx = ctxInput
	Tasks.Store(task.ID, &task)

	go func(func2 func(ctx context.Context) error) {

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
			task.State = Finished
			t := time.Now()
			task.EndTime = &t
			Tasks.Delete(task.ID)
		}()

		t := time.Now()
		task.StartTime = &t
		task.State = Running
		err := func2(ctxInput)
		if err != nil {
			log.Error(err)
		}
	}(f)
	return task.ID
}

type ScheduleTask struct {
	ID          string     `config:"id" json:"id,omitempty"`
	Group       string     `config:"group" json:"group,omitempty"`
	Description string     `config:"description" json:"description,omitempty"`
	Type        string     `config:"type" json:"type,omitempty"`
	Interval    string     `config:"interval" json:"interval,omitempty"`
	Crontab     string     `config:"crontab" json:"crontab,omitempty"`
	CreateTime  time.Time  `config:"create_time" json:"create_time,omitempty"`
	StartTime   *time.Time `config:"start_time" json:"start_time,omitempty"`
	EndTime     *time.Time `config:"end_time" json:"end_time,omitempty"`

	Task     func(ctx context.Context) `config:"-" json:"-"`
	taskItem chrono.ScheduledTask
	State    State           `config:"state" json:"state,omitempty"`
	Ctx      context.Context `config:"-" json:"-"` //for transient task
}

const Interval = "interval"
const Crontab = "crontab"
const Transient = "transient"

func RegisterScheduleTask(task ScheduleTask) (taskID string) {
	if task.ID == "" {
		task.ID = util.GetUUID()
	}
	task.CreateTime = time.Now()
	task.State = Pending
	if task.Type == "" && task.Interval != "" {
		task.Type = Interval
	} else if task.Type == "" && task.Crontab != "" {
		task.Type = Crontab
	}

	tempTask := task.Task
	task.Task = func(ctx context.Context) {
		t := time.Now()
		task.StartTime = &t
		task.EndTime = nil

		tempTask(ctx)

		t = time.Now()
		task.EndTime = &t
		task.State = Finished
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

	return task.ID
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
	if task.State == Running {
		return
	}
	if global.Env().IsDebug {
		log.Debug("scheduled task:", task.ID, ",", task.Type, ",", task.Interval, ",", task.Crontab, ",", task.Description)
	}

	switch task.Type {
	case Interval:
		task1, err := taskScheduler.ScheduleAtFixedRate(task.Task, util.GetDurationOrDefault(task.Interval, defaultInterval))
		if err != nil {
			log.Error("failed to scheduled interval task:", task.Type, ",", task.Interval, ",", task.Description)
		}
		task.State = Running
		task.taskItem = task1
		break
	case Crontab:
		task1, err := taskScheduler.ScheduleWithCron(task.Task, task.Crontab)
		if err != nil {
			log.Error("failed to scheduled crontab task:", task.Type, ",", task.Interval, ",", task.Description)
		}
		task.State = Running
		task.taskItem = task1
		break
	case Transient:
		//no need to schedule
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
			if item != nil {
				switch item.Type {
				case Interval:
					if item.taskItem != nil {
						item.taskItem.Cancel()
						item.State = Canceled
					}
					break
				case Crontab:
					if item.taskItem != nil {
						item.taskItem.Cancel()
						item.State = Canceled
					}
					break
				case Transient:
					if item.Ctx != nil {
						item.Ctx.Done()
						item.State = Canceled
					}
				}
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

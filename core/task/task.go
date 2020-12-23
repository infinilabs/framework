package task

import (
	"time"
	log "github.com/cihub/seelog"
)

var scheduleTasks = []ScheduleTask{}

type ScheduleTask struct {
	Description string
	Type string
	Interval string
	Task func()
}

func RegisterScheduleTask(task ScheduleTask) {
	scheduleTasks = append(scheduleTasks, task)
}

var quit = make(chan struct{})

func RunTasks()  {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <- ticker.C:
				for _,task:=range scheduleTasks{
					log.Tracef("task: %s, %v, %v",task.Description,task.Type,task.Interval)
					task.Task()
				}
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()

}

func StopTasks() {
	close(quit)
}
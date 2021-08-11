package pipeline

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"runtime"
)

func runAsync(job *Job,ctx *Context) {

	if job.onFailure != nil {
		defer job.onFailure.Process(ctx)
	}

	var ch chan bool
	ch = make(chan bool, len(job.tasks)*2)

	waitSignal:=len(job.tasks)
	if job.mode=="first_win"{
		waitSignal=1
	}else{
		ch = make(chan bool, len(job.tasks))
	}

	for _, task := range job.tasks {
		go func(task Processor) {
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
						log.Error(r,v)
					}
				}
			}()

			task.Process(ctx)
			ch <- true
		}(task)
	}

	for i:=0;i<waitSignal;i++{
		_ = <-ch
	}

	if job.onComplete != nil {
		for _,v:=range job.onComplete{
			v.Process(ctx)
		}
	}
}

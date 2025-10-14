/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"runtime"
	"sync"
	"time"
)

const PipelineSingleton = "pipeline_transient_tasks_singleton"

var singletonMap = sync.Map{}

type LockPayload struct {
	PipelineName string
	TaskID string
	Started time.Time
}

func RunPipelineSync(v PipelineConfigV2, ctx *Context) error {
	pipelineKey := fmt.Sprintf("[%v][%v]", v.Name, ctx.id)

	log.Tracef("pipeline %v task entered", pipelineKey)

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
				log.Errorf("error on running transient pipeline, %v, %v",pipelineKey, v)
			}
		}
		if v.Singleton {
			if v1,ok:=singletonMap.Load(v.Name);ok{
				t1,ok:=v1.(LockPayload)
				if ok &&pipelineKey==t1.TaskID{
					singletonMap.Delete(t1.PipelineName)
				}
			}
		}
		log.Tracef("pipeline %v task exited", pipelineKey)
	}()

	if v.Singleton {
		//inter-process lock check
		payload:=LockPayload{PipelineName: v.Name,TaskID: pipelineKey,Started: util.GetLowPrecisionCurrentTime()}
		if t,exists:=singletonMap.LoadOrStore(v.Name,payload);exists{
			t1,ok:=t.(LockPayload)
			if ok{
				elapsed:=time.Since(t1.Started)
				if elapsed.Milliseconds()>v.MaxRunningInMs{
					log.Warnf("pipeline %v is already running, but elapsed: %v > %v ms",v.Name,elapsed,v.MaxRunningInMs)
				}
			}
			log.Debugf("pipeline %v task skipped", pipelineKey)
			return nil
		}


		log.Debugf("pipeline %v is configured as singleton, checking distributed lock", pipelineKey)
		if v.MaxRunningInMs <= 0 {
			v.MaxRunningInMs = 30*60*1000
			log.Debugf("pipeline %v MaxRunningInMs not set, using default: %v ms", pipelineKey, v.MaxRunningInMs)
		}
		//distributed lock check
		ok, err := locker.Hold(PipelineSingleton, v.Name, global.Env().SystemConfig.NodeConfig.ID, time.Duration(v.MaxRunningInMs)*time.Millisecond, true)
		log.Trace("check locker, name:",v.Name,",id:",ctx.id,",ok:",ok,",err:",err,",",time.Duration(v.MaxRunningInMs)*time.Millisecond)
		if !ok {
			ctx.Finished()
			log.Debugf("pipeline %v is already running somewhere, skipped, %v", pipelineKey, err)
			return nil
		}
		log.Debugf("pipeline %v acquired distributed lock successfully", pipelineKey)
	}

	log.Infof("pipeline %v task started", pipelineKey)

	processors, err := v.GetProcessorsConfig()
	if err != nil {
		return errors.Errorf("failed to get processor config, %v", err)
	}

	processor, err := NewPipeline(processors)
	if err != nil {
		return errors.Errorf("pipeline %v failed to create processor: %v", pipelineKey, err)
	}
	log.Debugf("pipeline %v processor created successfully", pipelineKey)

	ctx.Started()
	log.Debugf("pipeline %v start processing...", pipelineKey)

	err = processor.Process(ctx)
	if err != nil {
		return errors.Errorf("pipeline %v processor failed: %v", pipelineKey, err)
	} else {
		log.Debugf("pipeline %v processor completed successfully", pipelineKey)
	}

	log.Infof("pipeline %v task ended", pipelineKey)

	return nil
}

func RunPipelineAsync(v PipelineConfigV2, ctx *Context) error {

	pipelineKey := fmt.Sprintf("[%v][%v]", v.Name, ctx.id)
	log.Tracef("starting RunPipelineAsync for %v", pipelineKey)

	taskCtx := context.WithValue(context.Background(), "cfg", v)
	taskCtx = context.WithValue(taskCtx, "ctx", ctx)
	task.RegisterTransientTask("pipeline:transient",pipelineKey,"", func(taskCtx context.Context) error {
		cfgV := taskCtx.Value("cfg")
		v, ok := cfgV.(PipelineConfigV2)
		if !ok {
			return errors.Errorf("pipeline %v failed to get valid pipeline config from task context", pipelineKey)
		}

		ctxV := taskCtx.Value("ctx")
		ctx, ok := ctxV.(*Context)
		if !ok {
			return errors.Errorf("pipeline %v failed to get valid pipeline context from task context", pipelineKey)
		}

		log.Tracef("pipeline %v config retrieved successfully", pipelineKey)

		return RunPipelineSync(v, ctx)
	}, taskCtx)

	log.Debugf("pipeline %v task scheduled successfully", pipelineKey)

	return nil
}

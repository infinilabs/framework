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
	"time"
)

const PipelineSingleton = "pipeline_transient_tasks_singleton"

func RunPipelineSync(v PipelineConfigV2, ctx *Context) error {
	pipelineKey := fmt.Sprintf("[%v][%v]", v.Name, ctx.id)

	log.Debugf("pipeline %v task started", pipelineKey)

	//distribution lock check
	if v.Singleton {
		log.Debugf("pipeline %v is configured as singleton, checking distributed lock", pipelineKey)
		if v.MaxRunningInMs <= 0 {
			v.MaxRunningInMs = 60000
			log.Debugf("pipeline %v MaxRunningInMs not set, using default: %v ms", pipelineKey, v.MaxRunningInMs)
		}
		ok, err := locker.Hold(PipelineSingleton, v.Name, global.Env().SystemConfig.NodeConfig.ID, time.Duration(v.MaxRunningInMs)*time.Millisecond, true)
		if !ok {
			ctx.Finished()
			log.Debugf("pipeline %v is already running somewhere, %v", pipelineKey, err)
			return errors.Errorf("pipeline %v is already running somewhere, %v", pipelineKey, err)
		}
		log.Debugf("pipeline %v acquired distributed lock successfully", pipelineKey)
	}

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
	return nil
}

func RunPipelineAsync(v PipelineConfigV2, ctx *Context) error {

	pipelineKey := fmt.Sprintf("[%v][%v]", v.Name, ctx.id)
	log.Tracef("starting RunPipelineAsync for %v", pipelineKey)

	taskCtx := context.WithValue(context.Background(), "cfg", v)
	taskCtx = context.WithValue(taskCtx, "ctx", ctx)
	task.RunWithContext("pipeline:transient:"+pipelineKey, func(taskCtx context.Context) error {
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

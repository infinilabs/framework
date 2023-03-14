/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipeline

import (
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
)

type PipeModule struct {
	api.Handler
	started bool
	closing bool

	pipelines sync.Map
	configs   sync.Map
	contexts  sync.Map
}

func (module PipeModule) Name() string {
	return "Pipeline"
}

var moduleCfg = struct{}{}

func (module *PipeModule) Setup() {

	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	module.pipelines = sync.Map{}
	module.contexts = sync.Map{}
	module.configs = sync.Map{}

	pipeline.RegisterProcessorPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)

	api.HandleAPIMethod(api.GET, "/pipeline/tasks/", module.getPipelinesHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/tasks/", module.createPipelineHandler)
	api.HandleAPIMethod(api.GET, "/pipeline/task/:id", module.getPipelineHandler)
	api.HandleAPIMethod(api.DELETE, "/pipeline/task/:id", module.deletePipelineHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_start", module.startTaskHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_stop", module.stopTaskHandler)

	//listen on changes
	config.NotifyOnConfigChange(func() {
		configFile := global.Env().GetConfigFile()
		configDir := global.Env().GetConfigDir()
		parentCfg, err := config.LoadFile(configFile)
		if err != nil {
			log.Error("failed to load config file: ", err, ", path: ", configFile)
			return
		}
		childCfg, err := config.LoadPath(configDir)
		if err != nil {
			log.Error("failed to load config dir: ", err, ", path: ", configDir)
			return
		}
		err = parentCfg.Merge(childCfg)
		if err != nil {
			log.Error("failed to merge configs: ", err)
		}

		newConfig := []pipeline.PipelineConfigV2{}

		if ok := parentCfg.HasField("pipeline"); ok {
			parentCfg, err = parentCfg.Child("pipeline", -1)
			if err != nil {
				log.Error(err)
				return
			}
			err := parentCfg.Unpack(&newConfig)
			if err != nil {
				log.Error(err)
				return
			}
		}

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
					log.Error("error on apply pipeline change,", v)
				}
			}
		}()

		needStopAndClean := map[string]struct{}{}
		newPipelines := map[string]pipeline.PipelineConfigV2{}

		for _, v := range newConfig {
			newPipelines[v.Name] = v
		}

		module.configs.Range(func(k, v any) bool {
			oldC, ok := v.(pipeline.PipelineConfigV2)
			if !ok {
				log.Errorf("impossible value from configs: %t", v)
				return true
			}
			// Don't stop dynamic pipelines
			if oldC.IsDynamic {
				return true
			}
			newC, ok := newPipelines[oldC.Name]
			// If old pipeline is present in the new pipeline configs and the same as the new config,
			// there's no need to stop and clean the old pipeline
			if ok && newC.Equals(oldC) {
				return true
			}
			needStopAndClean[oldC.Name] = struct{}{}
			return true
		})

		log.Debug("stopping and cleaning old pipelines")
		for k := range needStopAndClean {
			log.Infof("removing pipeline [%s]", k)
			module.stopTask(k)
			module.deleteTask(k)
		}

		log.Debug("starting new pipeline")
		for _, v := range newPipelines {
			err := module.createPipeline(v, false)
			if err != nil {
				log.Error("failed to create pipeline: ", err)
			}
		}
	})
}

func (module *PipeModule) startTask(taskID string) bool {
	ctx, ok := module.contexts.Load(taskID)
	if ok {
		if ctx, ok := ctx.(*pipeline.Context); ok {
			if ctx.IsPause() {
				ctx.Resume()
			}

			if ctx.IsExit() {
				ctx.Resume()
			}

			if ctx.GetRunningState() != pipeline.STARTED {
				ctx.Starting()
			}
			return true
		}
	}
	return false
}

func (module *PipeModule) stopTask(taskID string) bool {
	ctx, ok := module.contexts.Load(taskID)
	if ok {
		v1, ok := ctx.(*pipeline.Context)
		if ok {
			if global.Env().IsDebug {
				if rate.GetRateLimiterPerSecond("pipeline", "shutdown "+taskID+string(v1.GetRunningState()), 1).Allow() {
					log.Trace("start shutting down pipeline:", taskID, ",state:", v1.GetRunningState())
				}
			}

			if v1.GetRunningState() == pipeline.FAILED || v1.GetRunningState() == pipeline.STARTED || v1.GetRunningState() == pipeline.STARTING {
				v1.CancelTask()
				v1.Exit()
			}
		} else {
			log.Errorf("context for pipeline [%v] was missing", taskID)
		}
		return true
	}
	return false
}

func (module *PipeModule) deleteTask(taskID string) {
	module.pipelines.Delete(taskID)
	module.configs.Delete(taskID)
	module.contexts.Delete(taskID)
}

func (module *PipeModule) Start() error {
	if module.started {
		return errors.New("pipeline framework already started, please stop it first.")
	}

	//load pipeline from configs
	var pipelines []pipeline.PipelineConfigV2
	ok, err := env.ParseConfig("pipeline", &pipelines)
	if ok && err != nil {
		panic(err)
	}
	if ok {
		for _, v := range pipelines {
			err := module.createPipeline(v, false)
			if err != nil {
				log.Errorf("error on running pipeline: %v, err: %v", v.Name, err)
				continue
			}
		}
	}

	module.started = true
	module.closing = false
	return nil
}

func (module *PipeModule) Stop() error {

	if !module.started {
		log.Error("pipeline framework is not started")
		module.closing = false
		return nil
	}

	module.closing = true
	total := util.GetSyncMapSize(&module.contexts)
	if total <= 0 {
		return nil
	}

	log.Info("shutting down pipeline framework")
	start := time.Now()

	for {
		if time.Now().Sub(start).Minutes() > 5 {
			log.Error("pipeline framework failed to stop all tasks, quiting")
			return errors.New("pipeline framework failed to stop all tasks, quiting")
		}

		// Send stop signal to all contexts
		module.contexts.Range(func(key, value any) bool {
			k, ok := key.(string)
			if !ok {
				return false
			}
			ok = module.stopTask(k)
			return true
		})

		needRetry := false

		module.contexts.Range(func(k, v any) bool {
			taskID, ok := k.(string)
			if !ok {
				log.Errorf("impossible key from contexts: %t", k)
				return true
			}
			ctx, ok := v.(*pipeline.Context)
			if !ok {
				log.Errorf("impossible value from contexts: %t", v)
				return true
			}
			if ctx.GetRunningState() == pipeline.STARTED || ctx.GetRunningState() == pipeline.STARTING || ctx.GetRunningState() == pipeline.STOPPING {
				if rate.GetRateLimiterPerSecond("pipeline", "shutdown"+taskID+string(ctx.GetRunningState()), 1).Allow() {
					log.Debug("pipeline still running: ", taskID, ",state: ", ctx.GetRunningState())
				}
				needRetry = true
				return false
			}
			return true
		})

		if !needRetry {
			log.Info("finished shut down pipelines")
			module.started = false
			break
		}
	}

	return nil
}

func (module *PipeModule) createPipeline(v pipeline.PipelineConfigV2, dynamic bool) error {

	if _, ok := module.configs.Load(v.Name); ok {
		log.Tracef("pipeline [%v] is already created, skip", v.Name)
		return nil
	}

	v.IsDynamic = dynamic
	module.configs.Store(v.Name, v)

	log.Info("creating pipeline: " + v.Name)

	processor, err := pipeline.NewPipeline(v.Processors)
	if err != nil {
		return err
	}
	ctx := pipeline.AcquireContext()
	ctx.Config = v

	if v.RetryDelayInMs <= 0 {
		v.RetryDelayInMs = 1000
	}

	module.pipelines.Store(v.Name, processor)
	module.contexts.Store(v.Name, ctx)

	go func(cfg pipeline.PipelineConfigV2, p *pipeline.Processors, ctx *pipeline.Context) {
		defer func() {
			if !global.Env().IsDebug {
				if r := recover(); r != nil {
					var err string
					switch r.(type) {
					case error:
						err = r.(error).Error()
					case runtime.Error:
						err = r.(runtime.Error).Error()
					case string:
						err = r.(string)
					}
					log.Errorf("error on pipeline: %v, retry delay: %vms", cfg.Name, err)
				}
			}
		}()

		if !cfg.AutoStart {
			ctx.Stopped()
		} else {
			ctx.Starting()
		}

		log.Debug("processing pipeline_v2:", cfg.Name)

		for {
			state := ctx.GetRunningState()
			log.Tracef("%v, state:%v", cfg.Name, state)

			switch state {
			case pipeline.STARTING:
			RESTART:
				if global.Env().IsDebug {
					log.Debugf("pipeline [%v] start running", cfg.Name)
				}

				ctx.Started()
				err = p.Process(ctx)
				if cfg.KeepRunning && !ctx.IsExit() {
					if ctx.GetRunningState() != pipeline.STOPPED && ctx.GetRunningState() != pipeline.STOPPING {
						log.Tracef("pipeline [%v] end running, restart again, retry in [%v]ms", cfg.Name, cfg.RetryDelayInMs)
						if cfg.RetryDelayInMs > 0 {
							time.Sleep(time.Duration(cfg.RetryDelayInMs) * time.Millisecond)
						}

						if module.closing {
							log.Debugf("pipeline module stopped, skip running [%v]", cfg.Name)
							ctx.Finished()
							return
						}
						goto RESTART
					}
				}

				if err != nil {
					ctx.Failed()
					log.Errorf("error on pipeline:%v, %v", cfg.Name, err)
					break
				} else {
					ctx.Stopped()
				}

				log.Debugf("pipeline [%v] end running", cfg.Name)
				ctx.Finished()
				break
			case pipeline.FAILED:
				log.Debugf("pipeline [%v] failed", cfg.Name)
				if !cfg.KeepRunning {
					ctx.Pause()
				}
				break
			case pipeline.STOPPING:
				ctx.CancelTask()
				ctx.Pause()
				break
			case pipeline.STOPPED:
				log.Debugf("pipeline [%v] stopped", cfg.Name)
				ctx.Pause()
				break
			case pipeline.FINISHED:
				log.Debugf("pipeline [%v] finished", cfg.Name)
				ctx.Pause()
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

	}(v, processor, ctx)

	return err
}

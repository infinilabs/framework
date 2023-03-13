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
	"net/http"
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
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
	runningPipelines sync.Map
	configs          map[string]pipeline.PipelineConfigV2
	contexts         sync.Map
	started          bool
	closing          bool
	wg               sync.WaitGroup
}

func (module PipeModule) Name() string {
	return "Pipeline"
}

var moduleCfg = struct{}{}

func (module *PipeModule) Setup() {

	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	module.runningPipelines = sync.Map{}
	module.contexts = sync.Map{}
	module.configs = map[string]pipeline.PipelineConfigV2{}

	pipeline.RegisterProcessorPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)

	api.HandleAPIMethod(api.GET, "/pipeline/tasks/", module.getPipelines)
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

		old := module.configs
		skipKeys := map[string]string{}
		newPipeline := map[string]pipeline.PipelineConfigV2{}

		for _, v := range newConfig {
			newPipeline[v.Name] = v
			oldC, ok := old[v.Name]
			if ok {
				if v.Equals(oldC) {
					skipKeys[v.Name] = v.Name
					continue
				}
			}
		}

		// If all pipelines are unchanged, return early
		if len(newPipeline) == len(skipKeys) {
			return
		}

		log.Debug("stopping old entry points")
		for _, v := range old {
			_, ok := skipKeys[v.Name]
			if ok {
				continue
			}

			log.Infof("removing pipeline [%s]", v.Name)
			module.stopTask(v.Name)
			module.runningPipelines.Delete(v.Name)
			module.contexts.Delete(v.Name)
		}

		module.configs = newPipeline

		log.Debug("starting new pipeline")
		for _, v := range newPipeline {
			err := module.startPipeline(v)
			if err != nil {
				log.Error(err)
			}
		}

	})

}

func (module *PipeModule) startTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	if module.startTask(id) {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
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

func (module *PipeModule) stopTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	if module.stopTask(id) {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
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

func (module *PipeModule) getPipelines(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj := util.MapStr{}
	module.runningPipelines.Range(func(key, value any) bool {
		k, ok := key.(string)
		if ok {
			c, ok := module.contexts.Load(k)
			if ok {
				if c1, ok := c.(*pipeline.Context); ok {
					obj[k] = util.MapStr{
						"state":      c1.GetRunningState(),
						"start_time": c1.GetStartTime(),
						"end_time":   c1.GetEndTime(),
					}
				}
			}
		}
		return true
	})
	module.WriteJSON(w, obj, 200)
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
			err := module.startPipeline(v)
			if err != nil {
				log.Errorf("error on running pipeline: %v, err: %v", v.Name, err)
				continue
			}
			module.configs[v.Name] = v
		}
	}

	module.started = true
	module.closing = false
	return nil
}

func (module *PipeModule) Stop() error {

	if module.started {
		module.closing = true
		total := util.GetSyncMapSize(&module.contexts)
		if total <= 0 {
			return nil
		}

		log.Info("shutting down pipeline framework")
		start := time.Now()

	CLOSING:

		module.contexts.Range(func(key, value any) bool {
			k, ok := key.(string)
			if !ok {
				return false
			}
			ok = module.stopTask(k)
			return true
		})

		for k, _ := range module.configs {
			//if global.Env().IsDebug{
			//	log.Trace("checking config: ",k)
			//}
			v1, ok := module.contexts.Load(k)
			if ok {
				v, ok := v1.(*pipeline.Context)
				//if global.Env().IsDebug{
				//	log.Trace(v.Config.Name,",",v.GetRunningState())
				//}
				if ok {
					if v.GetRunningState() == pipeline.STARTED || v.GetRunningState() == pipeline.STARTING || v.GetRunningState() == pipeline.STOPPING {
						if time.Now().Sub(start).Minutes() > 5 {
							log.Error("pipeline framework failed to stop tasks, quiting")
							return errors.New("pipeline framework failure to stop tasks, quiting")
						}
						if rate.GetRateLimiterPerSecond("pipeline", "shutdown"+k+string(v.GetRunningState()), 1).Allow() {
							log.Debug("pipeline still running:", k, ",state:", v.GetRunningState(), ", closing")
						}
						goto CLOSING
					}
				}
			} else {
				log.Errorf("context for pipeline [%v] was missing", k)
			}
		}
		log.Info("finished shut down pipelines")
		module.started = false
	} else {
		log.Error("pipeline framework is not started")
	}

	module.closing = false
	return nil
}

func (module *PipeModule) startPipeline(v pipeline.PipelineConfigV2) error {

	if _, ok := module.runningPipelines.Load(v.Name); ok {
		log.Tracef("pipeline [%v] is already running, skip", v.Name)
		return nil
	}

	log.Info("starting pipeline: " + v.Name)

	processor, err := pipeline.NewPipeline(v.Processors)
	if err != nil {
		return err
	}
	ctx := pipeline.AcquireContext()
	ctx.Config = v

	if v.RetryDelayInMs <= 0 {
		v.RetryDelayInMs = 1000
	}

	module.runningPipelines.Store(v.Name, processor)
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

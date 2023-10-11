package pipeline

import (
	"context"
	"fmt"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/task"
	"runtime"
	"github.com/fsnotify/fsnotify"
	"sync"
	"sync/atomic"
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
	closed atomic.Bool

	pipelines sync.Map
	configs   sync.Map
	contexts  sync.Map
}

func (module *PipeModule) Name() string {
	return "pipeline"
}

var moduleCfg = struct {
	PipelineEnabledByDefault bool `config:"pipeline_enabled_by_default"`
}{PipelineEnabledByDefault: true}

func (module *PipeModule) Setup() {
	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	ok, err := env.ParseConfig("preference", &moduleCfg)
	if ok && err != nil  &&global.Env().SystemConfig.Configs.PanicOnConfigError{
		panic(err)
	}

	module.pipelines = sync.Map{}
	module.contexts = sync.Map{}
	module.configs = sync.Map{}

	pipeline.RegisterProcessorPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)

	api.HandleAPIMethod(api.GET, "/pipeline/tasks/", module.getPipelinesHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/tasks/_search", module.searchPipelinesHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/tasks/", module.createPipelineHandler)
	api.HandleAPIMethod(api.GET, "/pipeline/task/:id", module.getPipelineHandler)
	api.HandleAPIMethod(api.DELETE, "/pipeline/task/:id", module.deletePipelineHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_start", module.startTaskHandler)
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_stop", module.stopTaskHandler)

}

func (module *PipeModule) startTask(taskID string) (exists bool) {
	if module.closed.Load() {
		return false
	}

	ctx, ok := module.contexts.Load(taskID)
	if !ok {
		return
	}
	v1, ok := ctx.(*pipeline.Context)
	if !ok {
		return
	}

	exists = true

	// Mark exited pipeline to start again
	if v1.IsExit() {
		v1.Restart()
	}
	// Resume pipeline loop
	if v1.IsPause() {
		// Mark pipeline status as starting
		v1.Starting()
		v1.Resume()
	}

	return
}

// stopTask will cancel the current pipeline context, abort the pipeline execution.
func (module *PipeModule) stopTask(taskID string) (exists bool) {
	ctx, ok := module.contexts.Load(taskID)
	if !ok {
		return
	}
	v1, ok := ctx.(*pipeline.Context)
	if !ok {
		return
	}

	exists = true

	if global.Env().IsDebug {
		if rate.GetRateLimiterPerSecond("pipeline", "shutdown "+taskID+string(v1.GetRunningState()), 1).Allow() {
			log.Trace("start shutting down pipeline:", taskID, ",state:", v1.GetRunningState())
		}
	}

	// Mark pipeline as exited
	v1.Exit()
	// Mark pipeline as STOPPING as needed
	v1.Stopping()
	// call cancelFunc(), will mark IsCanceled asynchronously
	v1.CancelTask()

	return
}

// deleteTask will clean all in-memory states and release the pipeline context
func (module *PipeModule) deleteTask(taskID string) {
	module.pipelines.Delete(taskID)
	module.configs.Delete(taskID)
	module.releaseContext(taskID)
	module.contexts.Delete(taskID)
}

// releaseContext will release the task context
func (module *PipeModule) releaseContext(taskID string) {
	ctx, ok := module.contexts.Load(taskID)
	if ok {
		v1, ok := ctx.(*pipeline.Context)
		if ok {
			pipeline.ReleaseContext(v1)
			if v1.IsPause() {
				// release loop
				v1.Resume()
			}
		}
	}
}

func getPipelineConfig() ([]pipeline.PipelineConfigV2, error) {
	configFile := global.Env().GetConfigFile()
	configDir := global.Env().GetConfigDir()
	parentCfg, err := config.LoadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %v, path: %s", err, configFile)
	}
	childCfg, err := config.LoadPath(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load config dir: %v, path: %s", err, configDir)
	}
	err = parentCfg.Merge(childCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configs: %v", err)
	}

	pipelineCfg := []pipeline.PipelineConfigV2{}

	if ok := parentCfg.HasField("pipeline"); ok {
		parentCfg, err = parentCfg.Child("pipeline", -1)
		if err != nil {
			return nil, err
		}
		err = parentCfg.Unpack(&pipelineCfg)
		return pipelineCfg, err
	}
	return pipelineCfg, nil
}

func (module *PipeModule) Start() error {
	var (
		pipelines []pipeline.PipelineConfigV2
		err error
	)
	pipelines, err = getPipelineConfig()
	if err != nil &&global.Env().SystemConfig.Configs.PanicOnConfigError{
		panic(err)
	}
	for _, v := range pipelines {
		err := module.createPipeline(v, false)
		if err != nil {
			log.Errorf("error on running pipeline: %v, err: %v", v.Name, err)
			continue
		}
	}

	//listen on changes
	config.NotifyOnConfigChange(func(ev fsnotify.Event) {
		if module.closed.Load() || global.ShuttingDown(){
			log.Warn("module closed, skip reloading pipelines")
			return
		}

		log.Infof("config changed, checking for new pipeline configs, %v, %v",ev.Op,ev.Name)

		newConfig := []pipeline.PipelineConfigV2{}
		newConfig, err = getPipelineConfig()
		if err != nil {
			log.Error(err)
			return
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

		needStopAndClean := []string{}
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
			// Don't stop transient pipelines
			if oldC.Transient {
				return true
			}
			newC, ok := newPipelines[oldC.Name]
			// Skip condition: (old pipeline is present in the new pipeline configs, config is the same, new config is also enabled)
			if ok && newC.Equals(oldC) && isPipelineEnabled(newC.Enabled) {
				return true
			}
			needStopAndClean = append(needStopAndClean, oldC.Name)
			return true
		})

		if len(needStopAndClean) > 0 {
			log.Debug("stop and wait for pipelines to release")

			module.stopAndWaitForRelease(needStopAndClean, time.Minute)
			log.Info("old pipelines released")

			for _, taskID := range needStopAndClean {
				log.Infof("removing pipeline [%s]", taskID)
				module.deleteTask(taskID)
			}
		}

		log.Debug("starting new pipeline")
		for _, v := range newPipelines {
			err := module.createPipeline(v, false)
			if err != nil {
				log.Error("failed to create pipeline: ", err)
			}
		}
	})

	return nil
}

func (module *PipeModule) Stop() error {
	if module.closed.Load() {
		return nil
	}
	module.closed.Store(true)

	total := util.GetSyncMapSize(&module.contexts)
	if total <= 0 {
		return nil
	}

	log.Info("shutting down pipeline framework")

	var taskIDs []string
	module.contexts.Range(func(key, value any) bool {
		taskID, ok := key.(string)
		if !ok {
			return false
		}
		taskIDs = append(taskIDs, taskID)
		return true
	})

	module.stopAndWaitForRelease(taskIDs, time.Minute*5)

	log.Info("finished shut down pipelines")
	return nil
}

func (module *PipeModule) stopAndWaitForRelease(taskIDs []string, timeout time.Duration) {
	start := time.Now()

	for {
		if time.Now().Sub(start) > timeout {
			log.Error("waitForStop timed out")
			break
		}

		// Send stop signal to all contexts
		for _, taskID := range taskIDs {
			// cancel & stop
			module.stopTask(taskID)
			// release loop
			module.releaseContext(taskID)
			// don't delete context yet
		}

		needRetry := false
		for _, taskID := range taskIDs {
			v, ok := module.contexts.Load(taskID)
			if !ok {
				continue
			}

			ctx, ok := v.(*pipeline.Context)
			if !ok {
				log.Errorf("impossible value from contexts: %t", v)
				continue
			}

			if !ctx.IsLoopReleased() {
				if rate.GetRateLimiterPerSecond("pipeline", "shutdown"+taskID+string(ctx.GetRunningState()), 1).Allow() {
					log.Debug("pipeline still running: ", taskID, ",state: ", ctx.GetRunningState())
				}
				needRetry = true
				break
			}
		}

		if !needRetry {
			break
		}
	}
}

const pipelineSingleton = "pipeline_singleton"
var creatingLocker = sync.Mutex{}
func (module *PipeModule) createPipeline(v pipeline.PipelineConfigV2, transient bool) error {
	if module.closed.Load() {
		return errors.New("module closed")
	}

	if !isPipelineEnabled(v.Enabled) {
		// pipeline config explicitly disabled
		return nil
	}

	creatingLocker.Lock()
	defer creatingLocker.Unlock()

	v.Transient = transient

	// NOTE: hold the slot before creating pipeline loops
	if _, ok := module.configs.LoadOrStore(v.Name, v); ok {
		log.Tracef("pipeline [%v] is already created, skip", v.Name)
		return nil
	}

	log.Info("creating pipeline: "+v.Name+", singleton: ", v.Singleton)

	return task.RunWithContext("pipeline:"+v.Name, func(taskCtx context.Context) error {

		cfgV:=taskCtx.Value("cfg")
		cfg, ok := cfgV.(pipeline.PipelineConfigV2)
		if !ok{
			return errors.New("invalid pipeline config")
		}

		processor, err := pipeline.NewPipeline(v.Processors)
		if err != nil {
			return err
		}

		ctx := pipeline.AcquireContext(v)
		module.pipelines.Store(v.Name, processor)
		module.contexts.Store(v.Name, ctx)

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

			ctx.SetLoopReleased()
			processor.Release()
		}()

		if !cfg.AutoStart {
			// Mark pipeline as exited, don't run automatically
			ctx.Exit()
		} else {
			ctx.Starting()
		}

		log.Debug("processing pipeline_v2: ", cfg.Name)

		retryDelayInMs := 1000
		if cfg.RetryDelayInMs > 0 {
			retryDelayInMs = cfg.RetryDelayInMs
		}

		started := false

		for {
			if global.ShuttingDown() {
				log.Debugf("system is shutting down, pipeline [%v] will be stopped", cfg.Name)
				break
			}

			if ctx.IsReleased() {
				break
			}
			if module.closed.Load() {
				break
			}
			if ctx.IsExit() {
				ctx.Stopped()
			}

			// NOTE: state must be checked as the last step
			state := ctx.GetRunningState()
			if global.Env().IsDebug {
				log.Tracef("pipeline [%v], state: %v", cfg.Name, state)
				time.Sleep(500 * time.Millisecond)
			}
			switch state {
			case pipeline.STARTING:

				//check
				if v.Singleton {
					if v.MaxRunningInMs <= 0 {
						v.MaxRunningInMs = 60000
					}
					ok, err := locker.Hold(pipelineSingleton, v.Name, global.Env().SystemConfig.NodeConfig.ID, time.Duration(v.MaxRunningInMs)*time.Millisecond, true)
					if !ok {
						log.Debugf("pipeline [%v] is already running somewhere, %v", cfg.Name, err)
						ctx.Finished()
						continue
					}
				}

				// Pipeline needs to run
				if started {
					log.Errorf("pipeline [%v] started twice, should not happen", cfg.Name)
				}
				started = true
				ctx.Started()
				ctx.ResetContext()

				err = processor.Process(ctx)

				if err != nil {
					log.Errorf("error on pipeline:%v, %v", cfg.Name, err)
					ctx.Failed(err)
				} else {
					if global.Env().IsDebug {
						log.Debugf("pipeline [%v] end running", cfg.Name)
					}
					ctx.Finished()
				}
				started = false
			case pipeline.STARTED, pipeline.STOPPING:
				log.Errorf("pipeline [%v] loop should not detect %s", cfg.Name, state)
			case pipeline.FINISHED, pipeline.FAILED:
				// Pipeline ended, pause or start next round
				// keep_running: true & not stopped manually by Exit()
				// For IsExit, don't pause here, wait for STOPPED state, or we could Pause twice for STOPPED & IsExit.
				if cfg.KeepRunning {
					if global.Env().IsDebug {
						log.Tracef("pipeline [%v] end running, restart again, retry in [%v]ms", cfg.Name, retryDelayInMs)
					}

					if retryDelayInMs > 0 {
						log.Tracef("start sleep [%v]ms", retryDelayInMs)
						time.Sleep(time.Duration(retryDelayInMs) * time.Millisecond)
						log.Tracef("end sleep [%v]ms", retryDelayInMs)
					}

					// restart after delay.
					ctx.Starting()
				} else {
					ctx.Stopped()
					ctx.Pause()
				}
			case pipeline.STOPPED:
				// Pipeline manually stopped, pause
				ctx.Pause()
			}
		}

		log.Debugf("pipeline [%v] loop exited with state [%v]", cfg.Name, ctx.GetRunningState())


		return nil
	} , context.WithValue(context.Background(), "cfg", v))
}

func isPipelineEnabled(enabled *bool) bool {
	// if not configured `enabled: true`, by default true
	if enabled == nil {
		return moduleCfg.PipelineEnabledByDefault
	}
	return *enabled
}

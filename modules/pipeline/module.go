// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package pipeline

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/task"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	configs_client "infini.sh/framework/modules/configs/client"
)

type PipeModule struct {
	api.Handler
	closed atomic.Bool

	/*
	 * States of pipeline tasks. keyed by pipeline ID.
	 */
	pipelines sync.Map
	configs   sync.Map
	contexts  sync.Map
	// failures records pipelines whose config failed to compile on THIS
	// instance (e.g. a processor missing from the local catalog) — the
	// deployment status needs to see WHY, not just that it is absent
	failures sync.Map // name -> string (compile/load error)
	// generations counts the config revisions applied per task (1 = the
	// startup version); each successful hot reload increments it and the
	// runtime report exposes it so controllers can see a reload landed
	generations sync.Map // taskID -> int
	// cordoned marks the instance under maintenance: running tasks drain
	// to completion, but no task starts (auto_start/_start/_resume, and
	// tasks recreated by hot reload park STOPPED) until uncordoned
	cordoned atomic.Bool
}

// recordFailure stores a pipeline compile/load failure for the runtime
// report; recordSuccess clears a stale entry after the pipeline runs.
func (module *PipeModule) recordFailure(name string, err error) {
	if name == "" || err == nil {
		return
	}
	module.failures.Store(name, err.Error())
}

func (module *PipeModule) recordSuccess(name string) {
	module.failures.Delete(name)
}

// RuntimeSnapshot returns the current pipeline task states for the
// configs _sync runtime report (consumed by the deployment reconciler).
func (module *PipeModule) RuntimeSnapshot() map[string]interface{} {
	out := map[string]interface{}{}
	module.contexts.Range(func(k, v interface{}) bool {
		ctx, ok := v.(*pipeline.Context)
		if !ok {
			return true
		}
		// report under the pipeline NAME — consumers (deployment status)
		// match by name; the map key is the config ID which only equals
		// the name for id-less YAML configs
		name := ctx.Config.Name
		if name == "" {
			if s, ok := k.(string); ok {
				name = s
			}
		}
		state := "UNKNOWN"
		switch ctx.GetRunningState() {
		case pipeline.STARTED, pipeline.STARTING:
			state = "STARTED"
		case pipeline.FAILED:
			state = "FAILED"
		case pipeline.STOPPED, pipeline.FINISHED:
			state = "STOPPED"
		}
		entry := map[string]interface{}{
			"state": state,
		}
		if taskID, ok := k.(string); ok {
			entry["generation"] = module.generationOf(taskID)
		}
		out[name] = entry
		return true
	})
	// pipelines that never made it into contexts (config rejected or
	// processor missing locally) are reported as FAILED with the reason —
	// absent entries hide breakage from the deployment status
	module.failures.Range(func(k, v interface{}) bool {
		name, ok := k.(string)
		if !ok {
			return true
		}
		if _, running := out[name]; running {
			return true
		}
		if msg, ok := v.(string); ok {
			out[name] = map[string]interface{}{
				"state":  "FAILED",
				"reason": msg,
			}
		}
		return true
	})
	return out
}

func (module *PipeModule) Name() string {
	return "pipeline"
}

var moduleCfg = struct {
	PipelineEnabledByDefault bool `config:"pipeline_enabled_by_default"`
}{PipelineEnabledByDefault: true}

// loadLaneConfigs reads the top-level `lane:` stanzas (the static plane;
// dynamic lanes come via lane.Create at runtime).
func (module *PipeModule) loadLaneConfigs() {
	cfgs := []lane.LaneConfig{}
	if exist, err := env.ParseConfig("lane", &cfgs); !exist || err != nil {
		log.Debugf("no lane config section (exist=%v err=%v)", exist, err)
	}
	for _, c := range cfgs {
		if c.Name == "" {
			continue
		}
		if _, err := lane.Create(c); err != nil {
			log.Warnf("lane %s: %v", c.Name, err)
		}
	}
}

func (module *PipeModule) Setup() {
	// provide the runtime snapshot for the configs _sync client
	// (deployment reconciler reads it for health status)
	configs_client.SetPipelineRuntimeSnapshotProvider(module.RuntimeSnapshot)
	// ORM schema registration (table/index auto-creation on startup)
	orm.MustRegisterSchemaWithIndexName(pipeline.PipelineConfigV2{}, "pipeline-configs")
	orm.MustRegisterSchemaWithIndexName(PipelineVersion{}, "pipeline-versions")
	orm.MustRegisterSchemaWithIndexName(PipelineDeployment{}, "pipeline-deployments")
	orm.MustRegisterSchemaWithIndexName(PipelineStageStat{}, "pipeline-stage-stats")

	module.loadLaneConfigs()
	installFlight()
	installStats()
	module.startDeploymentReconciler()
	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	ok, err := env.ParseConfig("preference", &moduleCfg)
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	module.pipelines = sync.Map{}
	module.contexts = sync.Map{}
	module.configs = sync.Map{}

	pipeline.RegisterProcessorPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)
	pipeline.RegisterDomainProcessorWithConfigMetadata("event", "for_each", NewForEachProcessor, ForEachConfig{})

	//TODO remove
	// task list / processor discovery are consumed via the reverse channel
	// by managers, task delete is used for stream teardown; served on the
	// web port behind login + RBAC
	api.HandleAPIMethod(api.GET, "/pipeline/tasks/", module.getRunningPipelineTasksHandler)
	api.HandleUIMethod(api.GET, "/pipeline/tasks/", module.getRunningPipelineTasksHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.POST, "/pipeline/tasks/_search", module.searchPipelineTasksHandler)
	api.HandleUIMethod(api.POST, "/pipeline/tasks/_search", module.searchPipelineTasksHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.POST, "/pipeline/tasks/", module.createPipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/tasks/", module.createPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineCreate))
	api.HandleAPIMethod(api.GET, "/pipeline/task/:id", module.getPipelineTaskHandler)
	api.HandleUIMethod(api.GET, "/pipeline/task/:id", module.getPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.DELETE, "/pipeline/task/:id", module.deletePipelineTaskHandler)
	api.HandleUIMethod(api.DELETE, "/pipeline/task/:id", module.deletePipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineDelete))
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_start", module.startPipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_start", module.startPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_stop", module.stopPipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_stop", module.stopPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	// graceful stop (#121): finish the current batch, commit, park
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_drain", module.drainPipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_drain", module.drainPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	// instance maintenance (#121): drain everything / cordon against new starts
	api.HandleAPIMethod(api.POST, "/pipeline/_drain", module.drainInstanceHandler)
	api.HandleUIMethod(api.POST, "/pipeline/_drain", module.drainInstanceHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.POST, "/pipeline/_cordon", module.cordonInstanceHandler)
	api.HandleUIMethod(api.POST, "/pipeline/_cordon", module.cordonInstanceHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.POST, "/pipeline/_uncordon", module.uncordonInstanceHandler)
	api.HandleUIMethod(api.POST, "/pipeline/_uncordon", module.uncordonInstanceHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.GET, "/pipeline/_cordon", module.getCordonHandler)
	api.HandleUIMethod(api.GET, "/pipeline/_cordon", module.getCordonHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	// processor discovery: names + config schemas for pipeline designers
	api.HandleAPIMethod(api.GET, "/pipeline/processors", module.getProcessorsHandler)
	api.HandleUIMethod(api.GET, "/pipeline/processors", module.getProcessorsHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	// config pre-flight: compile check without persisting (editor save hint)
	api.HandleAPIMethod(api.POST, "/pipeline/_validate", module.validatePipelineHandler)
	api.HandleUIMethod(api.POST, "/pipeline/_validate", module.validatePipelineHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	api.HandleAPIMethod(api.GET, "/pipeline/codecs", module.getCodecsHandler)
	api.HandleUIMethod(api.GET, "/pipeline/codecs", module.getCodecsHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	api.HandleAPIMethod(api.GET, "/pipeline/lanes", module.getLanesHandler)
	api.HandleUIMethod(api.GET, "/pipeline/lanes", module.getLanesHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	// Fleet API (deployment management, D7)
	api.HandleAPIMethod(api.GET, "/pipeline/deployments/", module.listDeploymentsHandler)
	api.HandleUIMethod(api.GET, "/pipeline/deployments/", module.listDeploymentsHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.POST, "/pipeline/deployments/", module.createDeploymentHandler)
	api.HandleUIMethod(api.POST, "/pipeline/deployments/", module.createDeploymentHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineCreate))
	api.HandleAPIMethod(api.GET, "/pipeline/deployments/:id", module.getDeploymentHandler)
	api.HandleUIMethod(api.GET, "/pipeline/deployments/:id", module.getDeploymentHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.PUT, "/pipeline/deployments/:id", module.updateDeploymentHandler)
	api.HandleUIMethod(api.PUT, "/pipeline/deployments/:id", module.updateDeploymentHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.DELETE, "/pipeline/deployments/:id", module.deleteDeploymentHandler)
	api.HandleUIMethod(api.DELETE, "/pipeline/deployments/:id", module.deleteDeploymentHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineDelete))
	api.HandleAPIMethod(api.GET, "/pipeline/deployments/:id/status", module.getDeploymentStatusHandler)
	api.HandleUIMethod(api.GET, "/pipeline/deployments/:id/status", module.getDeploymentStatusHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	api.HandleAPIMethod(api.GET, "/pipeline/cluster/overview", module.getClusterOverviewHandler)
	api.HandleUIMethod(api.GET, "/pipeline/cluster/overview", module.getClusterOverviewHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	// pipeline version history + rollback
	api.HandleAPIMethod(api.GET, "/pipelines/:id/versions", module.listPipelineVersionsHandler)
	api.HandleUIMethod(api.GET, "/pipelines/:id/versions", module.listPipelineVersionsHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.GET, "/pipelines/:id/versions/:version", module.getPipelineVersionHandler)
	api.HandleUIMethod(api.GET, "/pipelines/:id/versions/:version", module.getPipelineVersionHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))
	api.HandleAPIMethod(api.POST, "/pipelines/:id/versions/:version/rollback", module.rollbackPipelineHandler)
	api.HandleUIMethod(api.POST, "/pipelines/:id/versions/:version/rollback", module.rollbackPipelineHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	api.HandleAPIMethod(api.POST, "/pipeline/dry-run", module.postDryRunHandler)
	api.HandleUIMethod(api.POST, "/pipeline/dry-run", module.postDryRunHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	api.HandleAPIMethod(api.POST, "/pipeline/ai-generate", module.postAIGenerateHandler)
	api.HandleUIMethod(api.POST, "/pipeline/ai-generate", module.postAIGenerateHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	api.HandleAPIMethod(api.GET, "/pipeline/task/:id/stages", module.stagesPipelineTaskHandler)
	api.HandleUIMethod(api.GET, "/pipeline/task/:id/stages", module.stagesPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	api.HandleAPIMethod(api.GET, "/pipeline/task/:id/flight", module.flightPipelineTaskHandler)
	api.HandleUIMethod(api.GET, "/pipeline/task/:id/flight", module.flightPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineRead))

	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_pause", module.pausePipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_pause", module.pausePipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_resume", module.resumePipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_resume", module.resumePipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))
	api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_inject", module.injectPipelineTaskHandler)
	api.HandleUIMethod(api.POST, "/pipeline/task/:id/_inject", module.injectPipelineTaskHandler, api.RequireLogin(), api.RequirePermission(security.PermissionSystemPipelineUpdate))

	//use pipelines to avoid naming conflicts
	api.HandleUIMethod(api.POST, "/pipelines/_search", module.searchPipelineHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Search)))
	api.HandleUIMethod(api.POST, "/pipelines/", module.createPipelineHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Create)))
	api.HandleUIMethod(api.GET, "/pipelines/:id", module.getPipelineHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Read)))
	api.HandleUIMethod(api.PUT, "/pipelines/:id", module.updatePipelineHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Update)))
	api.HandleUIMethod(api.DELETE, "/pipelines/:id", module.deletePipelineHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Delete)))

	api.HandleUIMethod(api.GET, "/pipelines/_running", module.getRunningPipelineTasksHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Admin)))
	api.HandleUIMethod(api.POST, "/pipelines/:id/_start", module.startPipelineTaskHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Admin)))
	api.HandleUIMethod(api.POST, "/pipelines/:id/_stop", module.stopPipelineTaskHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Admin)))
	//api.HandleUIMethod(api.POST, "/pipeline/task/:id/logging/_search", module.getPipelineLoggingsHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Admin)))
	//api.HandleUIMethod(api.POST, "/pipeline/task/:id/metrics/_search", module.getPipelineMetricsHandler, api.RequirePermission(security.GetOrInitPermission("generic", "pipeline", security.Admin)))
}

// pauseTask parks a running pipeline after its CURRENT batch: flow
// stops between fetches, in-flight records finish (no mid-batch abort).
func (module *PipeModule) pauseTask(taskID string) (exists bool) {
	ctx, ok := module.contexts.Load(taskID)
	if !ok {
		return
	}
	if v1, ok := ctx.(*pipeline.Context); ok {
		v1.Pause()
	}
	return true
}

func (module *PipeModule) resumeTask(taskID string) (exists bool) {
	return module.startTask(taskID)
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

	// A STOPPING pipeline is still winding down its processor chain (the
	// consumer workers drain their in-flight batches before returning).
	// Clearing the exit flag here would let keep_running resurrect the
	// pipeline the moment Process returns — silently undoing the stop.
	// Only an idle pipeline (STOPPED/FINISHED/FAILED) may be started.
	if v1.GetRunningState() == pipeline.STOPPING {
		if global.Env().IsDebug {
			log.Debug("pipeline:", taskID, " is stopping, skip start until it settles")
		}
		return
	}

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
		if err != nil {
			return nil, err
		}
		// YAML pipeline configs rarely set an explicit id field; fall back to
		// name so the in-memory task map key is stable and predictable.
		for i := range pipelineCfg {
			if pipelineCfg[i].ID == "" {
				pipelineCfg[i].ID = pipelineCfg[i].Name
			}
		}
		return pipelineCfg, nil
	}
	return pipelineCfg, nil
}

func (module *PipeModule) Start() error {
	module.startMetricsPersist()
	var (
		pipelines []pipeline.PipelineConfigV2
		err       error
	)
	pipelines, err = getPipelineConfig()
	if err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}
	for _, v := range pipelines {
		err := module.createPipelineTask(v, false)
		if err != nil {
			log.Errorf("error on running pipeline: %v(%v), err: %v", v.Name, v.ID, err)
			continue
		}
	}

	//listen on changes
	config.NotifyOnConfigChange(func(ev fsnotify.Event) {
		if module.closed.Load() || global.ShuttingDown() {
			log.Warn("module closed, skip reloading pipelines")
			return
		}

		log.Infof("config changed, checking for new pipeline configs, %v, %v", ev.Op, ev.Name)

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

		newPipelineNames := []string{}
		for _, v := range newConfig {
			newPipelines[v.ID] = v
			newPipelineNames = append(newPipelineNames, v.ID)
		}

		log.Debugf("we now have %v new pipelines: %v", len(newPipelineNames), newPipelineNames)

		module.configs.Range(func(k, v any) bool {
			oldC, ok := v.(pipeline.PipelineConfigV2)
			if !ok {
				log.Warnf("impossible value from configs: %v", v)
				return true
			}

			// Don't stop transient pipelines
			if oldC.Transient {
				log.Debugf("transient pipeline %v should not be reloaded", oldC.Name)
				return true
			}
			newC, ok := newPipelines[oldC.ID]
			isSame := newC.Equals(oldC)
			// Skip condition: (old pipeline is present in the new pipeline configs, config is the same, new config is also enabled)
			if ok && isSame && isPipelineEnabled(newC.Enabled) {
				log.Debugf("pipeline %v config not changed, skip reloading", oldC.Name)
				return true
			}

			log.Debug("pipeline config changed, stop and clean:", oldC.Name, ",", oldC, ",", ok, ",", isSame, ",", isPipelineEnabled(newC.Enabled))

			needStopAndClean = append(needStopAndClean, oldC.ID)
			return true
		})

		log.Debugf("we now have %v old pipelines need to clean: %v", len(needStopAndClean), needStopAndClean)

		if len(needStopAndClean) > 0 {
			log.Trace("stop and wait for pipelines to release: ", needStopAndClean)
			module.stopAndWaitForRelease(needStopAndClean, time.Minute)
			log.Debug("old pipelines released")

			for _, taskID := range needStopAndClean {
				log.Infof("removing pipeline [%s]", taskID)
				module.deleteTask(taskID)
			}
		}

		log.Debugf("starting %v pipelines", len(newPipelineNames))
		for k, v := range newPipelines {
			err := module.createPipelineTask(v, false)
			if err != nil {
				log.Errorf("failed to create pipeline: %v, err: %v", k, err)
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

	log.Info("shutting down pipelines")

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

// hotReloadLocker serializes hot reloads — two rapid API edits to the same
// pipeline would otherwise race their stop/delete/create sequences and
// LoadOrStore could converge on the older config.
var hotReloadLocker = sync.Mutex{}

// generationOf returns the config revision counter for a task (1 = the
// startup version; each successful hot reload increments it).
func (module *PipeModule) generationOf(taskID string) int {
	if v, ok := module.generations.Load(taskID); ok {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return 1
}

// hotReloadPipeline swaps the locally running task for pipelineID to newCfg
// (nil = stop and remove only) — the ORM CRUD path. API edits used to only
// persist: a running task kept its stale compiled chain until the instance
// restarted. Same semantics as the fsnotify reload: drain the in-flight
// batch (stopAndWaitForRelease), then rebuild from the new config.
// Transient tasks are never reloaded.
//
// Matching is strict on task ID — deployment-rendered configs keep the ORM
// pipeline ID, so an edit reaches every instance running the pipeline,
// while unrelated local YAML tasks (their own IDs) stay untouched.
func (module *PipeModule) hotReloadPipeline(pipelineID string, newCfg *pipeline.PipelineConfigV2) {
	v, ok := module.configs.Load(pipelineID)
	if !ok {
		return
	}
	oldC, ok := v.(pipeline.PipelineConfigV2)
	if !ok {
		return
	}
	if oldC.Transient {
		log.Debugf("pipeline [%s] is transient, skip hot reload", pipelineID)
		return
	}
	if newCfg != nil && oldC.Equals(*newCfg) && isPipelineEnabled(newCfg.Enabled) {
		return // semantically unchanged — nothing to rebuild
	}

	hotReloadLocker.Lock()
	defer hotReloadLocker.Unlock()

	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				log.Error("error on hot reloading pipeline,", pipelineID, ",", r)
			}
		}
	}()

	log.Infof("pipeline [%s] config changed via API, reloading task", pipelineID)
	module.stopAndWaitForRelease([]string{pipelineID}, time.Minute)
	module.deleteTask(pipelineID)

	if newCfg == nil {
		return
	}
	if err := module.createPipelineTask(*newCfg, false); err != nil {
		module.recordFailure(newCfg.Name, err)
		log.Errorf("pipeline [%s] hot reload: recreate failed: %v", pipelineID, err)
		return
	}
	module.generations.Store(pipelineID, module.generationOf(pipelineID)+1)
}

// reloadPipelineTaskAsync runs the hot reload off the request path —
// draining the current batch can legitimately take seconds, and the API
// response must not wait for it.
func (module *PipeModule) reloadPipelineTaskAsync(pipelineID string, newCfg *pipeline.PipelineConfigV2) {
	if _, ok := module.configs.Load(pipelineID); !ok {
		return // no local task runs this ORM row
	}
	go module.hotReloadPipeline(pipelineID, newCfg)
}

// drainTask marks the task for a graceful stop (#121): the consumer
// finishes the current batch and commits its offset, then the pipeline
// loop parks STOPPED (resumable via _start). Returns false when the task
// does not exist locally.
func (module *PipeModule) drainTask(taskID string) bool {
	v, ok := module.contexts.Load(taskID)
	if !ok {
		return false
	}
	ctx, ok := v.(*pipeline.Context)
	if !ok {
		return false
	}
	ctx.Drain()
	ctx.Exit() // no keep_running restarts after the current pass
	return true
}

// taskRunningState reports the task's current state ("" when absent).
func (module *PipeModule) taskRunningState(taskID string) pipeline.RunningState {
	v, ok := module.contexts.Load(taskID)
	if !ok {
		return ""
	}
	ctx, ok := v.(*pipeline.Context)
	if !ok {
		return ""
	}
	return ctx.GetRunningState()
}

// waitTaskSettled polls until the task leaves the running states or the
// timeout elapses; returns the last observed state.
func (module *PipeModule) waitTaskSettled(taskID string, timeout time.Duration) pipeline.RunningState {
	deadline := time.Now().Add(timeout)
	for {
		state := module.taskRunningState(taskID)
		if state != pipeline.STARTED && state != pipeline.STARTING && state != pipeline.STOPPING {
			return state
		}
		if time.Now().After(deadline) {
			return state
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// drainAllTasks gracefully stops every running task on this instance
// (maintenance path — pair with cordon so no new work starts while
// draining). All drains are marked first, then one bounded wait; the
// returned map carries each task's settled state.
func (module *PipeModule) drainAllTasks(timeout time.Duration) map[string]string {
	module.contexts.Range(func(k, v any) bool {
		if id, ok := k.(string); ok {
			module.drainTask(id)
		}
		return true
	})
	deadline := time.Now().Add(timeout)
	out := map[string]string{}
	for {
		running := 0
		module.contexts.Range(func(k, v any) bool {
			id, ok := k.(string)
			if !ok {
				return true
			}
			state := module.taskRunningState(id)
			out[id] = string(state)
			if state == pipeline.STARTED || state == pipeline.STARTING || state == pipeline.STOPPING {
				running++
			}
			return true
		})
		if running == 0 || time.Now().After(deadline) {
			return out
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// probeLockTakeover reports the previous owner when the singleton lock
// for name sits EXPIRED under another instance — the Hold that follows
// takes it over. kv-less environments (unit tests, lightweight agents)
// report "" instead of panicking.
func probeLockTakeover(bucket, name string, ttl time.Duration) string {
	defer func() { _ = recover() }()
	ok, info, err := locker.GetAllocateInfo(bucket, name)
	if err != nil || !ok || info == nil {
		return ""
	}
	self := global.Env().SystemConfig.NodeConfig.ID
	if info.ClientID == "" || info.ClientID == self {
		return ""
	}
	if time.Since(info.Timestamp) > ttl {
		return info.ClientID
	}
	return ""
}

// Helper function to run a pipeline, i.e., create a pipeline task.
func (module *PipeModule) createPipelineTask(v pipeline.PipelineConfigV2, transient bool) error {
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
	if _, ok := module.configs.LoadOrStore(v.ID, v); ok {
		log.Tracef("pipeline [%v] is already created, skip", v.Name)
		return nil
	}

	if v.Singleton {
		log.Info("creating pipeline: " + v.Name + ", singleton")
	} else {
		log.Info("creating pipeline: " + v.Name)
	}

	// The pipeline loop registers its context asynchronously; ready works
	// like a oneshot channel so this call only returns once the task is
	// visible to the API (or the processor build has failed).
	ready := make(chan error, 1)

	task.RunWithContext("pipeline:"+v.Name, func(taskCtx context.Context) error {

		cfgV := taskCtx.Value("cfg")
		cfg, ok := cfgV.(pipeline.PipelineConfigV2)
		if !ok {
			err := errors.New("invalid pipeline config")
			ready <- err
			return err
		}
		processorConfigs, err := cfg.GetProcessorsConfig()
		if err != nil {
			module.recordFailure(v.Name, err)
			ready <- err
			return errors.Errorf("failed to get processor config, %v", err)
		}

		processors, err := pipeline.NewPipeline(processorConfigs)
		if err != nil {
			module.recordFailure(v.Name, err)
			ready <- err
			return err
		}

		ctx := pipeline.AcquireContext(v)
		module.pipelines.Store(v.ID, processors)
		module.contexts.Store(v.ID, ctx)
		module.recordSuccess(v.Name)

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
			processors.Release()
		}()

		if !cfg.AutoStart {
			// Mark pipeline as exited, don't run automatically
			ctx.Exit()
		} else {
			ctx.Starting()
		}
		ready <- nil

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

				// cordoned instance: the task registers but does not run —
				// it parks STOPPED and resumes via _start after _uncordon
				if module.cordoned.Load() {
					ctx.Stopped()
					ctx.Pause()
					continue
				}

				//check
				if v.Singleton {
					if v.MaxRunningInMs <= 0 {
						v.MaxRunningInMs = 60000
					}
					ttl := time.Duration(v.MaxRunningInMs) * time.Millisecond
					// takeover detection (#122): the lock's lease expired
					// under another instance — the Hold below acquires it,
					// ending that instance's run; surface the event
					takeoverFrom := probeLockTakeover(pipelineSingleton, v.ID, ttl)
					ok, err := locker.Hold(pipelineSingleton, v.ID, global.Env().SystemConfig.NodeConfig.ID, ttl, true)
					if !ok {
						log.Debugf("pipeline [%v] is already running somewhere, %v", cfg.Name, err)
						ctx.Finished()
						continue
					}
					if takeoverFrom != "" {
						msg := fmt.Sprintf("singleton lock taken over from instance %s (lease expired)", takeoverFrom)
						log.Warnf("pipeline [%v]: %v", cfg.Name, msg)
						recordPipelineEvent(cfg.Name, pipeline.OutcomeTakeover, msg)
					}
				}

				// Pipeline needs to run
				if started {
					log.Errorf("pipeline [%v] started twice, should not happen", cfg.Name)
				}
				started = true
				ctx.Started()
				ctx.ResetContext()

				err = processors.Process(ctx)

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
	}, context.WithValue(context.Background(), "cfg", v))

	// The processor build failed; roll back the config slot so the task
	// does not linger as a phantom entry.
	if err := <-ready; err != nil {
		module.configs.Delete(v.ID)
		return err
	}

	return nil
}

func isPipelineEnabled(enabled *bool) bool {
	// if not configured `enabled: true`, by default true
	if enabled == nil {
		return moduleCfg.PipelineEnabledByDefault
	}
	return *enabled
}

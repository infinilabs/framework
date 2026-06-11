/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"net/http"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
)

<<<<<<< HEAD
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
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
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
	if ctx, ok := module.contexts.Load(taskID); ok {
		if v1, ok := ctx.(*pipeline.Context); ok && !v1.IsLoopReleased() {
			module.stopAndWaitForRelease([]string{taskID}, time.Minute)
		}
	}
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
=======
func (h *PipeModule) createPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &pipeline.PipelineConfigV2{}
	err := h.DecodeJSON(req, obj)
>>>>>>> origin/main
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if obj.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh

	err = orm.Create(ctx, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteCreatedOKJSON(w, obj.ID)
}

func (h *PipeModule) getPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := pipeline.PipelineConfigV2{}
	obj.ID = id

	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	h.WriteGetOKJSON(w, id, obj)
}

func (h *PipeModule) updatePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	replace := h.GetBoolOrDefault(req, "replace", false)
	var newConfig pipeline.PipelineConfigV2
	err := h.DecodeJSON(req, &newConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if newConfig.ID != "" && newConfig.ID != id {
		h.WriteError(w, "id in request body does not match id in URL", http.StatusBadRequest)
		return
	}

	if newConfig.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())

	var oldConfig pipeline.PipelineConfigV2
	oldConfig.ID = id
	exists, err := orm.GetWithSystemFields(ctx, &oldConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	if !replace {
		newConfig.Created = oldConfig.Created
	} else {
		t := time.Now()
		newConfig.Created = &t
	}

	// Ensure ID is set even if the client omitted it from the request body
	newConfig.ID = id

	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, &newConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteUpdatedOKJSON(w, newConfig.ID)
}

func (h *PipeModule) deletePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := pipeline.PipelineConfigV2{}
	obj.ID = id

	ctx := orm.NewContextWithParent(req.Context())

	exists, err := orm.GetV2(ctx, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	ctx.Refresh = orm.WaitForRefresh
	err = orm.Delete(ctx, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteDeletedOKJSON(w, id)
}

func (h *PipeModule) searchPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var err error
	builder, err := orm.NewQueryBuilderFromRequest(req, "name", "combined_fulltext")
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(builder.Sorts()) == 0 {
		builder.SortBy(orm.Sort{Field: "created", SortType: orm.DESC})
	}

	builder.EnableBodyBytes()

	ctx := orm.NewContextWithParent(req.Context())
	orm.WithModel(ctx, &pipeline.PipelineConfigV2{})

	var objs []pipeline.PipelineConfigV2

	err, res := elastic.SearchV2WithResultItemMapper(ctx, &objs, builder, nil)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = h.Write(w, res.Raw)
	if err != nil {
		h.Error(w, err)
	}
}

package pipeline

import (
	"net/http"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

func (module *PipeModule) getPipelinesHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resp := GetPipelinesResponse{}
	module.pipelines.Range(func(key, value any) bool {
		k, ok := key.(string)
		if ok {
			c, ok := module.contexts.Load(k)
			if ok {
				if c1, ok := c.(*pipeline.Context); ok {
					resp[k] = PipelineStatus{
						State:     c1.GetRunningState(),
						StartTime: c1.GetStartTime(),
						EndTime:   c1.GetEndTime(),
					}
				}
			}
		}
		return true
	})
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) getPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	v, ok := module.contexts.Load(id)
	if !ok {
		module.WriteError(w, "pipeline not found", http.StatusNotFound)
		return
	}
	ctx, ok := v.(*pipeline.Context)
	if !ok {
		module.WriteError(w, "invalid pipeline", http.StatusInternalServerError)
		return
	}
	resp := GetPipelineResponse{
		State:     ctx.GetRunningState(),
		StartTime: ctx.GetStartTime(),
		EndTime:   ctx.GetEndTime(),
	}
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) createPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = CreatePipelineRequest{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to parse pipeline config: ", err)
		return
	}
	var processors []*config.Config
	for _, processorDict := range obj.Processors {
		processor, err := config.NewConfigFrom(processorDict)
		if err != nil {
			module.WriteError(w, err.Error(), http.StatusBadRequest)
			log.Error("failed to parse processor config: ", err)
			return
		}
		processors = append(processors, processor)
	}
	err = module.createPipeline(pipeline.PipelineConfigV2{
		Name:           obj.Name,
		AutoStart:      obj.AutoStart,
		KeepRunning:    obj.KeepRunning,
		RetryDelayInMs: obj.RetryDelayInMs,
		Processors:     processors,
	})
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to start pipeline: ", err)
		return
	}
	module.WriteAckOKJSON(w)
}

func (module *PipeModule) deletePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	if module.stopTask(id) {
		module.deleteTask(id)
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
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

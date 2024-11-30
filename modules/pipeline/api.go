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
	"net/http"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
)

func (module *PipeModule) getPipelinesHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	resp := GetPipelinesResponse{}
	module.configs.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		status := module.getPipelineStatus(id, config, processor)
		if status != nil {
			resp[id] = status
		}
		return true
	})
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) searchPipelinesHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	var obj = SearchPipelinesRequest{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to parse request: ", err)
		return
	}
	resp := GetPipelinesResponse{}
	for _, id := range obj.Ids {
		status := module.getPipelineStatus(id, config, processor)
		if status != nil {
			resp[id] = status
		}
	}
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) getPipelineStatus(id string, config string, processor string) *PipelineStatus {
	c, ok := module.contexts.Load(id)
	if !ok {
		return nil
	}
	c1, ok := c.(*pipeline.Context)
	if !ok {
		return nil
	}
	ret := &PipelineStatus{
		State:      c1.GetRunningState(),
		CreateTime: c1.GetCreateTime(),
		StartTime:  c1.GetStartTime(),
		EndTime:    c1.GetEndTime(),
		Context:    c1.CloneData(),
	}
	if config != "false" {
		v1, ok := module.configs.Load(id)
		if !ok {
			return ret
		}
		cfg, ok := v1.(pipeline.PipelineConfigV2)
		if !ok {
			return ret
		}
		ret.Config = &cfg
		if processor != "false" {
			for i := range cfg.Processors {
				processorMap := map[string]interface{}{}
				cfg.Processors[i].Unpack(processorMap)
				ret.Processors = append(ret.Processors, processorMap)
			}
		}
	}
	return ret
}

func (module *PipeModule) getPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	status := module.getPipelineStatus(id, config, processor)
	if status == nil {
		module.WriteError(w, "pipeline not found", http.StatusNotFound)
		return
	}
	module.WriteJSON(w, status, 200)
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
		processor, err := ucfg.NewFrom(processorDict)
		if err != nil {
			module.WriteError(w, err.Error(), http.StatusBadRequest)
			log.Error("failed to parse processor config: ", err)
			return
		}
		processors = append(processors, config.FromConfig(processor))
	}
	pipelineConfig := obj.PipelineConfigV2
	pipelineConfig.Processors = processors
	err = module.createPipeline(pipelineConfig, true)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to start pipeline: ", err)
		return
	}
	module.WriteAckOKJSON(w)
}

func (module *PipeModule) deletePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	_, exists := module.contexts.Load(id)
	if exists {
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
	exists := module.startTask(id)
	if exists {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
}

func (module *PipeModule) stopTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	exists := module.stopTask(id)
	if exists {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
}

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

	"encoding/json"
	"github.com/infinitbyte/framework/core/api"
	"github.com/infinitbyte/framework/core/api/router"
	"github.com/infinitbyte/framework/core/pipeline"
	"strconv"
)

// API namespace
type API struct {
	api.Handler
}

func (handler API) Init() {

	//Pipeline API
	api.HandleAPIMethod(api.GET, "/pipeline/joints/", handler.getPipelineJoints)

	api.HandleAPIMethod(api.GET, "/pipeline/configs/", handler.getPipelineConfigs)
	api.HandleAPIMethod(api.POST, "/pipeline/config/", handler.createPipelineConfig)
	api.HandleAPIMethod(api.GET, "/pipeline/config/:id", handler.getPipelineConfig)
	api.HandleAPIMethod(api.PUT, "/pipeline/config/:id", handler.updatePipelineConfig)
	api.HandleAPIMethod(api.DELETE, "/pipeline/config/:id", handler.deletePipelineConfig)
}

//Pipeline config operations
func (handler API) getPipelineJoints(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	joints := pipeline.GetAllRegisteredJoints()
	handler.WriteJSON(w, joints, http.StatusOK)
}

func (handler API) getPipelineConfigs(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	fr := handler.GetParameter(req, "from")
	si := handler.GetParameter(req, "size")

	from, err := strconv.Atoi(fr)
	if err != nil {
		from = 0
	}
	size, err := strconv.Atoi(si)
	if err != nil {
		size = 10
	}

	total, configs, err := GetPipelineList(from, size)
	if err != nil {
		handler.Error(w, err)
	} else {
		handler.WriteJSONListResult(w, total, configs, http.StatusOK)
	}
}

func (handler API) createPipelineConfig(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := pipeline.PipelineConfig{}

	data, err := handler.GetRawBody(req)
	if err != nil {
		handler.Error(w, err)
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		handler.Error(w, err)
		return
	}

	err = CreatePipelineConfig(&config)
	if err != nil {
		handler.Error(w, err)
		return
	}

	handler.WriteJSON(w, config, http.StatusOK)
}

func (handler API) getPipelineConfig(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	id := ps.ByName("id")
	cfg, err := GetPipelineConfig(id)
	if err != nil {
		handler.Error(w, err)
	} else {
		handler.WriteJSON(w, cfg, http.StatusOK)
	}
}

func (handler API) updatePipelineConfig(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := pipeline.PipelineConfig{}
	id := ps.ByName("id")

	data, err := handler.GetRawBody(req)
	if err != nil {
		handler.Error(w, err)
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		handler.Error(w, err)
		return
	}

	err = UpdatePipelineConfig(id, &config)
	if err != nil {
		handler.Error(w, err)
		return
	}

	handler.WriteJSON(w, config, http.StatusOK)
}

func (handler API) deletePipelineConfig(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")

	err := DeletePipelineConfig(id)
	if err != nil {
		handler.Error(w, err)
		return
	}

	handler.WriteJSON(w, map[string]interface{}{"ok": true}, http.StatusOK)
}

//Pipeline runner operations
func (handler API) listPipelineRunners(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

func (handler API) startPipelineRunner(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

func (handler API) stopPipelineRunner(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

func (handler API) pausePipelineRunner(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

func (handler API) resumePipelineRunner(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

//Pipeline debug operations
func (handler API) debugPipeline(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

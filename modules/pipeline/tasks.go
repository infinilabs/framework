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
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

func (module *PipeModule) getRunningPipelineTasksHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	resp := GetPipelineTasksResponse{}
	module.configs.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		status := module.getPipelineTaskStatus(id, config, processor)
		if status != nil {
			resp[id] = status
		}
		return true
	})
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) searchPipelineTasksHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	var obj = SearchPipelineTasksRequest{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		_ = log.Error("failed to parse request: ", err)
		return
	}
	resp := GetPipelineTasksResponse{}
	for _, id := range obj.Ids {
		status := module.getPipelineTaskStatus(id, config, processor)
		if status != nil {
			resp[id] = status
		}
	}
	module.WriteJSON(w, resp, 200)
}

func (module *PipeModule) getPipelineTaskStatus(id string, config string, processor string) *PipelineTaskStatus {
	c, ok := module.contexts.Load(id)
	if !ok {
		return nil
	}
	c1, ok := c.(*pipeline.Context)
	if !ok {
		return nil
	}
	ret := &PipelineTaskStatus{
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
			processors, err := cfg.GetProcessorsConfig()
			if err != nil {
				panic(err)
			}
			if processors != nil {
				for i := range processors {
					processorMap := map[string]interface{}{}
					err := processors[i].Unpack(processorMap)
					if err != nil {
						panic(err)
					}
					ret.Processors = append(ret.Processors, processorMap)
				}
			}

		}
	}
	return ret
}

func (module *PipeModule) getPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	config := module.Get(req, "config", "false")
	processor := module.Get(req, "processor", "false")
	status := module.getPipelineTaskStatus(id, config, processor)
	if status == nil {
		module.WriteError(w, "pipeline not found", http.StatusNotFound)
		return
	}
	module.WriteJSON(w, status, 200)
}

// For the task pipeline config, if it is enabled, create a task to run it.
//
// eg: curl -XPOST http://localhost:2900/pipeline/tasks/ -d'{"name":"echo-test","enabled":true,"auto_start":true,"keep_running":true,"processor":[{"echo":{"message":"hello world"}}]}'
func (module *PipeModule) createPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = pipeline.PipelineConfigV2{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		_ = log.Error("failed to parse pipeline config: ", err)
		return
	}
	if obj.Name == "" {
		module.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}
	if obj.ID == "" {
		obj.ID = obj.Name
	}
	// compile gate BEFORE creating the task — createPipelineTask skips
	// disabled pipelines entirely, which would otherwise defer the
	// discovery of a broken chain to the moment it gets enabled
	if !module.validateAndReject(w, &obj) {
		return
	}
	err = module.createPipelineTask(obj, true)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		_ = log.Error("failed to start pipeline: ", err)
		return
	}
	module.WriteAckOKJSON(w)
}

// curl -XDELETE http://localhost:2900/pipeline/task/echo-test
func (module *PipeModule) deletePipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	_, exists := module.contexts.Load(id)
	if exists {
		// Cancel before deleting the registry entry: the pipeline goroutine
		// is blocked inside Process (consumer worker loops on IsCanceled,
		// lane processors on ctx.Done) — clearing the map alone would leave
		// the goroutine alive forever and the task registry stuck at
		// STARTED (a zombie "running" task). Same as the auto_reload path:
		// cancel and wait for release (≤1min), force the delete on timeout.
		module.stopAndWaitForRelease([]string{id}, time.Minute)
		module.deleteTask(id)
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
}

// curl -XPOST http://localhost:2900/pipeline/task/echo-test/_start
// stagesPipelineTaskHandler — GET /pipeline/task/:id/stages
// Standard per-stage metrics: processor invocations/errors/latency plus
// live lane carriers (depth-derived).
func (module *PipeModule) stagesPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	module.WriteJSON(w, util.MapStr{"stages": stageStats(id)}, http.StatusOK)
}

// flightPipelineTaskHandler — GET /pipeline/task/:id/flight
// Sampled execution trace of a pipeline: ?trace_id= for one record's
// journey, ?errors=1 for recent failures, &limit= (default 50, newest
// first). Memory-only (ring); persistence rides the metrics exporter.
func (module *PipeModule) flightPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	q := req.URL.Query()
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	events := flightQuery(id, q.Get("trace_id"), q.Get("errors") == "1", limit)
	module.WriteJSON(w, util.MapStr{"events": events}, http.StatusOK)
}

// requestedGeneration parses the optional ?generation= control guard.
// Returns (0, false) when absent or malformed — legacy unguarded behavior.
func requestedGeneration(req *http.Request) (int, bool) {
	g := req.URL.Query().Get("generation")
	if g == "" {
		return 0, false
	}
	n, err := strconv.Atoi(g)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// checkGenerationGuard enforces the fleet control contract (#120): an
// instruction stamped with a generation applies only when the local task
// runs exactly that config revision — an instance still on an older
// generation (hot reload not landed yet) must not act on instructions
// issued for the new one, and vice versa. Writes a 409 carrying both
// generations on mismatch; absent/malformed param keeps today's
// unconditional behavior.
func (module *PipeModule) checkGenerationGuard(w http.ResponseWriter, req *http.Request, id string) bool {
	n, ok := requestedGeneration(req)
	if !ok {
		return true
	}
	cur := module.generationOf(id)
	if cur != n {
		module.WriteAckJSON(w, false, http.StatusConflict, util.MapStr{
			"error":                  "generation mismatch, instruction skipped",
			"current_generation":     cur,
			"instruction_generation": n,
		})
		return false
	}
	return true
}

// pausePipelineTaskHandler — POST /pipeline/task/:id/_pause
func (module *PipeModule) pausePipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	if !module.checkGenerationGuard(w, req, id) {
		return
	}
	if !module.pauseTask(id) {
		module.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	module.WriteJSON(w, util.MapStr{"paused": true, "id": id}, http.StatusOK)
}

// resumePipelineTaskHandler — POST /pipeline/task/:id/_resume
func (module *PipeModule) resumePipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	if !module.checkGenerationGuard(w, req, id) {
		return
	}
	if module.cordoned.Load() {
		module.WriteAckJSON(w, false, http.StatusConflict, util.MapStr{"error": "instance is cordoned"})
		return
	}
	if !module.resumeTask(id) {
		module.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	module.WriteJSON(w, util.MapStr{"resumed": true, "id": id}, http.StatusOK)
}

// injectPipelineTaskHandler — POST /pipeline/task/:id/_inject
// Replays a sample into the task's input (queue or lane target).
type injectRequest struct {
	Queue   string         `json:"queue"`
	Lane    string         `json:"lane"`
	Samples []InjectSample `json:"samples"`
}

// InjectSample accepts raw as either base64 (Go json []byte default) or
// plain text — callers should not need to pre-encode log lines.
type InjectSample struct {
	Raw   []byte `json:"raw"`
	Codec string `json:"codec"`
}

func (s *InjectSample) UnmarshalJSON(b []byte) error {
	type alias struct {
		Raw   json.RawMessage `json:"raw"`
		Codec string          `json:"codec"`
	}
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	s.Codec = a.Codec
	// try standard base64 first ([]byte json encoding); fall back to the
	// literal string bytes (plain-text payloads)
	if err := json.Unmarshal(a.Raw, &s.Raw); err != nil {
		// not base64 — use the raw string content directly
		var str string
		if err2 := json.Unmarshal(a.Raw, &str); err2 == nil {
			s.Raw = []byte(str)
		} else {
			s.Raw = a.Raw
		}
	}
	return nil
}

func (module *PipeModule) injectPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	if _, ok := module.contexts.Load(id); !ok {
		module.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	var body injectRequest
	if err := module.DecodeJSON(req, &body); err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	targets := 0
	if body.Queue != "" {
		targets++
	}
	if body.Lane != "" {
		targets++
	}
	if targets != 1 {
		module.WriteError(w, "exactly one target (queue or lane) required", http.StatusBadRequest)
		return
	}
	pushed := 0
	for _, s := range body.Samples {
		if body.Lane != "" {
			l := lane.GetOrCreate(lane.LaneConfig{Name: body.Lane})
			rec, err := pipeline.GetRecordCodec(s.Codec).Decode(s.Raw)
			if err != nil {
				module.WriteError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := l.Emit(rec); err != nil {
				module.WriteError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			pushed++
			continue
		}
		if err := queue.Push(queue.SmartGetOrInitConfig(&queue.QueueConfig{Name: body.Queue}), s.Raw); err != nil {
			module.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pushed++
	}
	module.WriteJSON(w, util.MapStr{"injected": pushed, "id": id}, http.StatusOK)
}

func (module *PipeModule) startPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	if !module.checkGenerationGuard(w, req, id) {
		return
	}
	if module.cordoned.Load() {
		module.WriteAckJSON(w, false, http.StatusConflict, util.MapStr{"error": "instance is cordoned"})
		return
	}
	exists := module.startTask(id)
	if exists {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
}

// curl -XPOST http://localhost:2900/pipeline/task/echo-test/_stop
func (module *PipeModule) stopPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	if !module.checkGenerationGuard(w, req, id) {
		return
	}
	exists := module.stopTask(id)
	if exists {
		module.WriteAckOKJSON(w)
	} else {
		module.WriteAckJSON(w, false, 404, util.MapStr{
			"error": "task not found",
		})
	}
}

// drainPipelineTaskHandler — POST /pipeline/task/:id/_drain
// Graceful stop: the consumer finishes the current batch and commits its
// offset, then the task parks STOPPED (resumable via _start). Honors the
// ?generation= guard; ?timeout_seconds= bounds the settle wait (default
// 30, max 600).
func (module *PipeModule) drainPipelineTaskHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	if !module.checkGenerationGuard(w, req, id) {
		return
	}
	if !module.drainTask(id) {
		module.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	timeout := 30 * time.Second
	if v := req.URL.Query().Get("timeout_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 600 {
			timeout = time.Duration(n) * time.Second
		}
	}
	state := module.waitTaskSettled(id, timeout)
	settled := state != pipeline.STARTED && state != pipeline.STARTING && state != pipeline.STOPPING
	module.WriteJSON(w, util.MapStr{"drained": settled, "id": id, "state": string(state)}, http.StatusOK)
}

// drainInstanceHandler — POST /pipeline/_drain
// Graceful-stop every running task on this instance (maintenance path;
// pair with _cordon so no new work starts while draining). Optional
// ?timeout_seconds= bounds the shared settle wait (default 60).
func (module *PipeModule) drainInstanceHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	timeout := 60 * time.Second
	if v := req.URL.Query().Get("timeout_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 3600 {
			timeout = time.Duration(n) * time.Second
		}
	}
	states := module.drainAllTasks(timeout)
	module.WriteJSON(w, util.MapStr{"drained": states}, http.StatusOK)
}

// cordonInstanceHandler — POST /pipeline/_cordon
// Marks the instance cordoned: running tasks drain to completion, but no
// task starts (auto_start, _start, _resume; hot-reload recreations park
// STOPPED) until _uncordon.
func (module *PipeModule) cordonInstanceHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	module.cordoned.Store(true)
	log.Info("pipeline instance cordoned: task starts are rejected until _uncordon")
	module.WriteJSON(w, util.MapStr{"cordoned": true}, http.StatusOK)
}

// uncordonInstanceHandler — POST /pipeline/_uncordon
func (module *PipeModule) uncordonInstanceHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	module.cordoned.Store(false)
	log.Info("pipeline instance uncordoned")
	module.WriteJSON(w, util.MapStr{"cordoned": false}, http.StatusOK)
}

// getCordonHandler — GET /pipeline/_cordon
func (module *PipeModule) getCordonHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	module.WriteJSON(w, util.MapStr{"cordoned": module.cordoned.Load()}, http.StatusOK)
}

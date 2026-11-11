// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package pipeline

import (
	"net/http"

	httprouter "infini.sh/framework/core/api/router"

	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/pipeline/dryrun"
)

// getProcessorsHandler serves GET /pipeline/processors: the registry of
// available pipeline processors with their config schemas, so that
// pipeline designer UIs can render configuration forms.
//
//	?grouped=1   category-grouped catalog {category: {name: {...}}}
//	             (default)        flat {name: {...}} with a category field
func (module *PipeModule) getProcessorsHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	if req.URL.Query().Get("grouped") == "1" {
		module.WriteJSON(w, pipeline.GetProcessorCatalog(), 200)
		return
	}
	module.WriteJSON(w, pipeline.GetProcessorMetadata(), 200)
}

// postDryRunHandler — POST /pipeline/dry-run
// The unified dry-run (single definition): chain + inline samples +
// options → Report (per-step snapshots, field diffs, captured writes).
type dryRunRequest struct {
	Processor []map[string]interface{} `json:"processor"`
	Samples   []struct {
		Raw   string `json:"raw"`
		Codec string `json:"codec"`
	} `json:"samples"`
	Options struct {
		ScopeCheck  bool   `json:"scope_check"`
		SideEffects string `json:"side_effects"`
		LLMPolicy   string `json:"llm_policy"`
	} `json:"options"`
}

func (module *PipeModule) postDryRunHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var body dryRunRequest
	if err := module.DecodeJSON(req, &body); err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	samples := make([]dryrun.Sample, 0, len(body.Samples))
	for _, s := range body.Samples {
		samples = append(samples, dryrun.Sample{Raw: []byte(s.Raw), Codec: s.Codec})
	}
	report, err := dryrun.Run(body.Processor, samples, dryrun.Options{
		ScopeCheck:  body.Options.ScopeCheck,
		SideEffects: body.Options.SideEffects,
		LLMPolicy:   body.Options.LLMPolicy,
	})
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	module.WriteJSON(w, report, http.StatusOK)
}

// getLanesHandler — GET /pipeline/lanes
// Live snapshot of every lane: depth/rates/drops/spills/blocked_waits —
// the in-process counterpart of queue depth observability.
func (module *PipeModule) getLanesHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	module.WriteJSON(w, util.MapStr{"lanes": lane.List()}, 200)
}

// getCodecsHandler — GET /pipeline/codecs
// Discovery view of the record codec registry (name / namespace / version),
// feeding for_each's codec picker and the canvas.
func (module *PipeModule) getCodecsHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	module.WriteJSON(w, util.MapStr{"codecs": pipeline.ListRecordCodecs()}, 200)
}

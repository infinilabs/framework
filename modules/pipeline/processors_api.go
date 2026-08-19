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

	"infini.sh/framework/core/pipeline"
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

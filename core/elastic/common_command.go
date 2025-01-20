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

package elastic

import "time"

type CommandRequest struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Body   string `json:"body"`
}

type CommonCommand struct {
	ID       string           `json:"-" index:"id"`
	Title    string           `json:"title" elastic_mapping:"title:{type:text,fields:{keyword:{type:keyword}}}"`
	Tag      []string         `json:"tag" elastic_mapping:"tag:{type:keyword}"`
	Requests []CommandRequest `json:"requests" elastic_mapping:"requests:{type:object}"`
	Created  time.Time        `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
}

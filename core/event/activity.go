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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package event

import (
	"infini.sh/framework/core/util"
	"time"
)

type Activity struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp:{type: date }"`
	Metadata  ActivityMetadata `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Changelog interface{} `json:"changelog,omitempty" elastic_mapping:"changelog:{type: object,enabled:false }"`
	Fields    util.MapStr   `json:"payload" elastic_mapping:"payload:{type: object,enabled:false }"`
}

type ActivityMetadata struct {
	Labels util.MapStr `json:"labels,omitempty"  elastic_mapping:"labels:{type: object }"`
	Category string `json:"category,omitempty"`
	Group     string `json:"group,omitempty"`
	Name     string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	User util.MapStr `json:"user,omitempty"`
}
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
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

import (
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

//es process info with process id
type ESNodeInfo struct {
	ID             string      `json:"id,omitempty"  elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	AgentID        string      `json:"agent_id" elastic_mapping:"agent_id: { type: keyword }"`
	ClusterID      string      `json:"cluster_id,omitempty" elastic_mapping:"cluster_id: { type: keyword }"`
	ClusterUuid    string      `json:"cluster_uuid,omitempty" elastic_mapping:"cluster_uuid: { type: keyword }"`
	ClusterName    string      `json:"cluster_name,omitempty" elastic_mapping:"cluster_name: { type: keyword }"`
	NodeUUID       string      `json:"node_uuid,omitempty" elastic_mapping:"node_uuid: { type: keyword }"`
	NodeName       string      `json:"node_name,omitempty" elastic_mapping:"node_name: { type: keyword }"`
	Version        string      `json:"version,omitempty" elastic_mapping:"version: { type: keyword }"`
	Timestamp      int64       `json:"timestamp"`
	PublishAddress string      `json:"publish_address" elastic_mapping:"publish_address: { type: keyword }"`
	HttpPort       string      `json:"http_port"`
	Schema         string      `json:"schema"`
	Status         string      `json:"status" elastic_mapping:"status: { type: keyword }"`
	ProcessInfo    ProcessInfo `json:"process_info" elastic_mapping:"process_info : { type : object, enabled:false }"`
	Path           PathInfo    `json:"path"`
}

type PathInfo struct {
	Home   string `json:"home"`
	Data   string `json:"data"`
	Logs   string `json:"logs"`
	Config string `json:"config"`
}

type ListenAddr struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type ProcessInfo struct {
	PID             int          `json:"pid"`
	Name            string       `json:"name"`
	Cmdline         string       `json:"cmdline"`
	CreateTime      int64        `json:"create_time"`
	Status          string       `json:"status"`
	ListenAddresses []ListenAddr `json:"listen_addresses"`
}

type Setting struct {
	orm.ORMObjectBase
	Metadata Metadata    `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Payload  util.MapStr `json:"payload" elastic_mapping:"payload: { type: object}"`
}

type Metadata struct {
	Category string                 `json:"category" elastic_mapping:"category: { type: keyword }"`
	Name     string                 `json:"name" elastic_mapping:"name: { type: keyword }"`
	Labels   util.MapStr `json:"labels" elastic_mapping:"labels: { type: object }"`
}


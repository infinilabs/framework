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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package host

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"time"
)

type HostInfo struct {
	orm.ORMObjectBase

	AgentID     string    `json:"agent_id" elastic_mapping:"agent_id: { type: keyword, copy_to:search_text }"`
	IP          string    `json:"ip" elastic_mapping:"ip: { type: keyword, copy_to:search_text }"`
	Name        string    `json:"name" elastic_mapping:"name: { type: keyword, copy_to:search_text }"`    //eg: zhangsandeMacBook-Pro.local
	MemorySize  uint64    `json:"memory_size,omitempty" elastic_mapping:"memory_size: { type: keyword }"` //byte, eg: 17179869184
	DiskSize    uint64    `json:"disk_size,omitempty" elastic_mapping:"disk_size: { type: keyword }"`     //byte, eg: 494384795648
	CPUInfo     CPU       `json:"cpu_info,omitempty" elastic_mapping:"cpu_info: { type: object }"`
	OSInfo      OS        `json:"os_info,omitempty" elastic_mapping:"os_info: { type: object }"`
	UpTime      time.Time `json:"up_time,omitempty" elastic_mapping:"up_time: { type: date }"`
	Tags        []string  `json:"tags,omitempty" elastic_mapping:"tags: { type: keyword, copy_to:search_text }"`
	SearchText  string    `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
	Timestamp   time.Time `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	AgentStatus string    `json:"agent_status,omitempty" elastic_mapping:"agent_status: { type: keyword }"`
	NodeID      string    `json:"node_id,omitempty" elastic_mapping:"node_id: { type: keyword }"`
}

type CPU struct {
	Model       string `json:"model,omitempty" elastic_mapping:"model: { type: keyword }"` //eg: Apple M1 Pro
	PhysicalCPU int    `json:"physical_cpu,omitempty" elastic_mapping:"physical_cpu: { type: keyword }"`
	LogicalCPU  int    `json:"logical_cpu,omitempty" elastic_mapping:"logical_cpu: { type: keyword }"`
}

type OS struct {
	Platform        string `json:"platform,omitempty" elastic_mapping:"platform: { type: keyword }"`                 //eg: darwin
	PlatformVersion string `json:"platform_version,omitempty" elastic_mapping:"platform_version: { type: keyword }"` //eg: 12.5
	KernelVersion   string `json:"kernel_version,omitempty" elastic_mapping:"kernel_version: { type: keyword }"`     //eg: 21.6.0
	KernelArch      string `json:"kernel_arch,omitempty" elastic_mapping:"kernel_arch: { type: keyword }"`           //eg: arm64
}

type Usage struct {
	orm.ORMObjectBase

	AgentID         string               `json:"agent_id" elastic_mapping:"agent_id: { type: keyword }"`
	CPUPercent      float64              `json:"cpu_percent,omitempty" elastic_mapping:"cpu_percent: { type: keyword }"`
	DiskUsage       *DiskUsageInfo       `json:"disk_usage,omitempty" elastic_mapping:"disk_usage: { type: object }"`
	DiskIOUsage     *DiskIOUsageInfo     `json:"disk_io_usage,omitempty" elastic_mapping:"disk_io_usage: { type: object }"`
	NetIOUsage      *NetIOUsageInfo      `json:"net_io_usage,omitempty" elastic_mapping:"net_io_usage: { type: object }"`
	MemoryUsage     *MemoryUsageInfo     `json:"memory_usage,omitempty" elastic_mapping:"memory_usage: { type: object }"`
	SwapMemoryUsage *SwapMemoryUsageInfo `json:"swap_memory_usage,omitempty" elastic_mapping:"swap_memory_usage: { type: object }"`
	ESProcessInfo   string               `json:"es_process_info"`
}

type UsageCategory string

const (
	AllUsage      UsageCategory = "all"
	CPUUsage                    = "cpu"
	DiskUsage                   = "disk"
	DiskIOUsage                 = "disk_io"
	NetIOUsage                  = "net_io"
	MemoryUsage                 = "memory"
	ESProcessInfo               = "es_process"
)

type DiskUsageInfo struct {
	Total       uint64  `json:"total" elastic_mapping:"total: { type: keyword }"` //byte
	Free        uint64  `json:"free" elastic_mapping:"free: { type: keyword }"`   //byte
	Used        uint64  `json:"used" elastic_mapping:"used: { type: keyword }"`   //byte
	UsedPercent float64 `json:"used_percent" elastic_mapping:"used_percent: { type: keyword }"`
}

type DiskIOUsageInfo struct {
	ReadBytes     uint64 `json:"read_bytes" elastic_mapping:"read_bytes: { type: keyword }"`           //byte
	WriteBytes    uint64 `json:"write_bytes" elastic_mapping:"write_bytes: { type: keyword }"`         //byte
	ReadTimeCost  uint64 `json:"read_time_cost" elastic_mapping:"read_time_cost: { type: keyword }"`   //ms
	WriteTimeCost uint64 `json:"write_time_cost" elastic_mapping:"write_time_cost: { type: keyword }"` //ms
}

type NetIOUsageInfo struct {
	BytesSent   uint64 `json:"bytes_sent" elastic_mapping:"bytes_sent: { type: keyword }"` //byte
	BytesRecv   uint64 `json:"bytes_recv" elastic_mapping:"bytes_recv: { type: keyword }"` //byte
	PacketsSent uint64 `json:"packets_sent" elastic_mapping:"packets_sent: { type: keyword }"`
	PacketsRecv uint64 `json:"packets_recv" elastic_mapping:"packets_recv: { type: keyword }"`
}

type MemoryUsageInfo struct {
	Total       uint64  `json:"total" elastic_mapping:"total: { type: keyword }"`         //byte
	Available   uint64  `json:"available" elastic_mapping:"available: { type: keyword }"` //byte
	Used        uint64  `json:"used" elastic_mapping:"used: { type: keyword }"`           //byte
	UsedPercent float64 `json:"used_percent" elastic_mapping:"used_percent: { type: keyword }"`
}

type SwapMemoryUsageInfo struct {
	Total       uint64  `json:"total" elastic_mapping:"total: { type: keyword }"`         //byte
	Free        uint64  `json:"available" elastic_mapping:"available: { type: keyword }"` //byte
	Used        uint64  `json:"used" elastic_mapping:"used: { type: keyword }"`           //byte
	UsedPercent float64 `json:"used_percent" elastic_mapping:"used_percent: { type: keyword }"`
}

func UpdateHostAgentStatus(agentID, agentStatus string) {
	err, result := orm.GetBy("agent_id", agentID, HostInfo{})
	if err != nil {
		log.Errorf("update host agent [%s] status error: %v", agentID, err)
		return
	}
	if len(result.Result) > 0 {
		buf := util.MustToJSONBytes(result.Result[0])
		hostInfo := &HostInfo{}
		err = util.FromJSONBytes(buf, hostInfo)
		if err != nil {
			log.Errorf("update host agent [%s] status error: %v", agentID, err)
			return
		}
		hostInfo.AgentStatus = agentStatus
		err = orm.Update(nil, hostInfo)
		if err != nil {
			log.Errorf("update host agent [%s] status error: %v", agentID, err)
		}
	}
}

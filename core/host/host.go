/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package host

import (
	"infini.sh/framework/core/orm"
	"time"
)

type HostInfo struct {
	orm.ORMObjectBase

	AgentID    string    `json:"agent_id" elastic_mapping:"agent_id: { type: keyword }"`
	Name       string    `json:"name" elastic_mapping:"name: { type: keyword }"`                         //eg: zhangsandeMacBook-Pro.local
	MemorySize uint64    `json:"memory_size,omitempty" elastic_mapping:"memory_size: { type: keyword }"` //byte, eg: 17179869184
	DiskSize   uint64    `json:"disk_size,omitempty" elastic_mapping:"disk_size: { type: keyword }"`     //byte, eg: 494384795648
	CPUInfo    CPU       `json:"cpu_info,omitempty" elastic_mapping:"cpu_info: { type: object }"`
	OSInfo     OS        `json:"os_info,omitempty" elastic_mapping:"os_info: { type: object }"`
	UpTime     time.Time `json:"up_time,omitempty" elastic_mapping:"up_time: { type: date }"`
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
}

type UsageCategory string

const (
	AllUsage    UsageCategory = "all"
	CPUUsage                  = "cpu"
	DiskUsage                 = "disk"
	DiskIOUsage               = "disk_io"
	NetIOUsage                = "net_io"
	MemoryUsage               = "memory"
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

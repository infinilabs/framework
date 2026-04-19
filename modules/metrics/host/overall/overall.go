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

package overall

import (
	"runtime"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	hostInfo "infini.sh/framework/core/host"
	"infini.sh/framework/core/util"
)

// Metric collects overall system utilization and computes a composite health status.
type Metric struct {
	Enabled         bool    `config:"enabled"`
	YellowThreshold float64 `config:"yellow_threshold"`
	RedThreshold    float64 `config:"red_threshold"`

	mu              sync.Mutex
	prevDiskIO      *diskIOSnapshot
}

type diskIOSnapshot struct {
	readTime  uint64
	writeTime uint64
}

func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		Enabled:         true,
		YellowThreshold: hostInfo.DefaultYellowThreshold,
		RedThreshold:    hostInfo.DefaultRedThreshold,
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	log.Debugf("overall utilization metric enabled, yellow=%v%%, red=%v%%", me.YellowThreshold, me.RedThreshold)
	return me, nil
}

// Collect gathers CPU, memory, disk, and disk I/O utilization, computes a composite status,
// and emits a "host/overall" event.
func (m *Metric) Collect() error {
	if !m.Enabled {
		return nil
	}

	subsystems := make([]hostInfo.SubsystemHealth, 0, 4)

	// --- CPU utilization ---
	cpuPercent := m.collectCPU()
	subsystems = append(subsystems, hostInfo.SubsystemHealth{
		Name:    "cpu",
		Status:  hostInfo.ClassifyHealth(cpuPercent, m.YellowThreshold, m.RedThreshold),
		Percent: cpuPercent,
	})

	// --- Memory utilization ---
	memPercent := m.collectMemory()
	subsystems = append(subsystems, hostInfo.SubsystemHealth{
		Name:    "memory",
		Status:  hostInfo.ClassifyHealth(memPercent, m.YellowThreshold, m.RedThreshold),
		Percent: memPercent,
	})

	// --- Disk capacity utilization ---
	diskPercent := m.collectDiskUsage()
	subsystems = append(subsystems, hostInfo.SubsystemHealth{
		Name:    "disk",
		Status:  hostInfo.ClassifyHealth(diskPercent, m.YellowThreshold, m.RedThreshold),
		Percent: diskPercent,
	})

	// --- Disk I/O utilization (busy %) ---
	diskIOPercent := m.collectDiskIO()
	if diskIOPercent >= 0 {
		subsystems = append(subsystems, hostInfo.SubsystemHealth{
			Name:    "disk_io",
			Status:  hostInfo.ClassifyHealth(diskIOPercent, m.YellowThreshold, m.RedThreshold),
			Percent: diskIOPercent,
		})
	}

	// Determine overall status = worst subsystem
	overall := hostInfo.OverallStatus{
		Status:     "green",
		Subsystems: subsystems,
	}
	for _, s := range subsystems {
		if hostInfo.HealthPriority(s.Status) > hostInfo.HealthPriority(overall.Status) {
			overall.Status = s.Status
			overall.Bottleneck = s.Name
		}
	}

	fields := util.MapStr{
		"status":     overall.Status,
		"bottleneck": overall.Bottleneck,
	}
	for _, s := range subsystems {
		fields[s.Name+".status"] = s.Status
		fields[s.Name+".used_percent"] = s.Percent
	}

	return event.Save(&event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "overall",
			Datatype: "gauge",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"overall": fields,
			},
		},
	})
}

// collectCPU returns the current overall CPU utilization percentage (0-100).
func (m *Metric) collectCPU() float64 {
	percents, err := cpu.Percent(0, false)
	if err != nil {
		log.Errorf("overall: failed to get cpu percent: %v", err)
		return 0
	}
	if len(percents) > 0 {
		return percents[0]
	}
	return 0
}

// collectMemory returns the current memory utilization percentage (0-100).
func (m *Metric) collectMemory() float64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Errorf("overall: failed to get memory info: %v", err)
		return 0
	}
	if v == nil {
		return 0
	}
	return v.UsedPercent
}

// collectDiskUsage returns the disk capacity utilization percentage (0-100).
func (m *Metric) collectDiskUsage() float64 {
	if runtime.GOOS == "darwin" {
		v, err := disk.Usage("/")
		if err != nil {
			log.Errorf("overall: failed to get disk usage: %v", err)
			return 0
		}
		return v.UsedPercent
	}

	partitions, err := disk.Partitions(false)
	if err != nil || len(partitions) == 0 {
		log.Errorf("overall: failed to get disk partitions: %v", err)
		return 0
	}
	var total, used uint64
	for _, p := range partitions {
		if p.Device == "" {
			continue
		}
		v, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		total += v.Total
		used += v.Used
	}
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100.0
}

// collectDiskIO returns the disk I/O busy percentage (0-100) based on io time deltas.
// Returns -1 if data is not yet available (first call).
func (m *Metric) collectDiskIO() float64 {
	ret, err := disk.IOCounters()
	if err != nil {
		log.Debugf("overall: failed to get disk io counters: %v", err)
		return -1
	}
	if len(ret) == 0 {
		return -1
	}

	var totalReadTime, totalWriteTime uint64
	for _, io := range ret {
		totalReadTime += io.ReadTime
		totalWriteTime += io.WriteTime
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.prevDiskIO == nil {
		m.prevDiskIO = &diskIOSnapshot{
			readTime:  totalReadTime,
			writeTime: totalWriteTime,
		}
		return -1
	}

	// IO busy time delta in ms over the collection interval (10s = 10000ms)
	deltaRead := totalReadTime - m.prevDiskIO.readTime
	deltaWrite := totalWriteTime - m.prevDiskIO.writeTime
	deltaIO := deltaRead + deltaWrite

	m.prevDiskIO.readTime = totalReadTime
	m.prevDiskIO.writeTime = totalWriteTime

	// 10s interval = 10000ms total wall time
	const intervalMs = 10000.0
	busy := float64(deltaIO) / intervalMs * 100.0
	if busy > 100.0 {
		busy = 100.0
	}
	if busy < 0 {
		busy = 0
	}
	return busy
}

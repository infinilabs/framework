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
	"infini.sh/framework/core/util"
)

// Metric collects overall system utilization percentages for CPU, memory, disk and disk I/O.
type Metric struct {
	Enabled         bool    `config:"enabled"`
	IntervalSeconds float64 `config:"interval_seconds"`

	mu         sync.Mutex
	prevDiskIO *diskIOSnapshot
}

type diskIOSnapshot struct {
	readTime  uint64
	writeTime uint64
}

func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		Enabled:         true,
		IntervalSeconds: 10,
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	log.Debugf("overall utilization metric enabled")
	return me, nil
}

// Collect gathers CPU, memory, disk, and disk I/O utilization percentages
// and emits a "host/overall" event with raw values for the front layer to interpret.
func (m *Metric) Collect() error {
	if !m.Enabled {
		return nil
	}

	fields := util.MapStr{}

	// --- CPU utilization ---
	fields["cpu.used_percent"] = m.collectCPU()

	// --- Memory utilization ---
	fields["memory.used_percent"] = m.collectMemory()

	// --- Disk capacity utilization ---
	fields["disk.used_percent"] = m.collectDiskUsage()

	// --- Disk I/O utilization (busy %) ---
	diskIOPercent := m.collectDiskIO()
	if diskIOPercent >= 0 {
		fields["disk_io.used_percent"] = diskIOPercent
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

	// IO busy time delta in ms over the collection interval
	intervalMs := m.IntervalSeconds * 1000.0
	busy := float64(deltaIO) / intervalMs * 100.0
	if busy > 100.0 {
		busy = 100.0
	}
	if busy < 0 {
		busy = 0
	}
	return busy
}

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
	"strings"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// Metric collects overall system utilization percentages for CPU, memory, disk, disk I/O and network.
// Each disk and network interface is monitored independently to identify specific bottlenecks.
type Metric struct {
	Enabled         bool    `config:"enabled"`
	IntervalSeconds float64 `config:"interval_seconds"`
	YellowThreshold float64 `config:"yellow_threshold"`
	RedThreshold    float64 `config:"red_threshold"`

	mu sync.Mutex

	// Per-disk I/O tracking: map[deviceName] -> snapshot
	prevDiskIO map[string]*diskIOSnapshot

	// Per-network interface tracking: map[ifaceName] -> snapshot
	prevNetIO map[string]*netIOSnapshot

	// Per-network interface bandwidth (auto-detected): map[ifaceName] -> Mbps
	netBandwidth map[string]float64
}

// diskIOSnapshot stores previous I/O counters for a disk device
type diskIOSnapshot struct {
	readTime  uint64
	writeTime uint64
}

// netIOSnapshot stores previous I/O counters for a network interface
type netIOSnapshot struct {
	bytesRecv uint64
	bytesSent uint64
}

// deviceUtilization represents utilization info for a single device
type deviceUtilization struct {
	name        string
	usedPercent float64
}

func New(cfg *config.Config) (*Metric, error) {
	me := &Metric{
		Enabled:         true,
		IntervalSeconds: 10,
		YellowThreshold: 70,
		RedThreshold:    90,
		prevDiskIO:      make(map[string]*diskIOSnapshot),
		prevNetIO:       make(map[string]*netIOSnapshot),
		netBandwidth:    make(map[string]float64),
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	// Initialize network bandwidth detection for all interfaces
	me.initNetworkBandwidth()

	log.Debugf("overall utilization metric enabled")
	return me, nil
}

// initNetworkBandwidth detects and stores bandwidth for each network interface
func (m *Metric) initNetworkBandwidth() {
	bandwidths := detectNetworkBandwidthPerInterface()
	for name, bw := range bandwidths {
		m.netBandwidth[name] = bw
		log.Debugf("overall: interface %s bandwidth: %.0f Mbps", name, bw)
	}
}

// Collect gathers CPU, memory, disk, disk I/O and network utilization
// and emits a "host/overall" event with raw values for the front layer to interpret.
// Each disk and network interface is monitored independently.
func (m *Metric) Collect() error {
	if !m.Enabled {
		return nil
	}

	fields := util.MapStr{}

	// Collect all metrics
	cpuPercent := m.collectCPU()
	memPercent := m.collectMemory()
	diskPercent := m.collectDiskUsage()
	diskIODevices := m.collectDiskIO()
	netDevices := m.collectNetwork()

	// --- CPU utilization ---
	fields["cpu.used_percent"] = cpuPercent

	// --- Memory utilization ---
	fields["memory.used_percent"] = memPercent

	// --- Disk capacity utilization ---
	fields["disk.used_percent"] = diskPercent

	// --- Per-disk I/O utilization ---
	diskIOMap := util.MapStr{}
	var maxDiskIO deviceUtilization
	for _, dev := range diskIODevices {
		diskIOMap[dev.name] = util.MapStr{
			"used_percent": dev.usedPercent,
		}
		if dev.usedPercent > maxDiskIO.usedPercent {
			maxDiskIO = dev
		}
	}
	if len(diskIOMap) > 0 {
		fields["disk_io.devices"] = diskIOMap
		fields["disk_io.used_percent"] = maxDiskIO.usedPercent
		fields["disk_io.bottleneck_device"] = maxDiskIO.name
	}

	// --- Per-network interface utilization ---
	netMap := util.MapStr{}
	var maxNet deviceUtilization
	for _, dev := range netDevices {
		bw := m.netBandwidth[dev.name]
		if bw <= 0 {
			bw = 1000 // Default 1 Gbps if unknown
		}
		netMap[dev.name] = util.MapStr{
			"used_percent":  dev.usedPercent,
			"bandwidth_mbps": bw,
		}
		if dev.usedPercent > maxNet.usedPercent {
			maxNet = dev
		}
	}
	if len(netMap) > 0 {
		fields["network.devices"] = netMap
		fields["network.used_percent"] = maxNet.usedPercent
		fields["network.bottleneck_device"] = maxNet.name
	}

	// --- Calculate overall status and bottleneck ---
	status, bottleneck := m.calculateStatus(cpuPercent, memPercent, diskPercent, maxDiskIO, maxNet)
	fields["status"] = status
	fields["bottleneck"] = bottleneck

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

// collectDiskIO returns per-disk I/O utilization percentages (0-100) based on io time deltas.
// Returns empty slice if data is not yet available (first call).
func (m *Metric) collectDiskIO() []deviceUtilization {
	ret, err := disk.IOCounters()
	if err != nil {
		log.Debugf("overall: failed to get disk io counters: %v", err)
		return nil
	}
	if len(ret) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var results []deviceUtilization

	for name, io := range ret {
		// Skip certain device types
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}

		prev, exists := m.prevDiskIO[name]
		if !exists {
			// First time seeing this device, store initial values
			m.prevDiskIO[name] = &diskIOSnapshot{
				readTime:  io.ReadTime,
				writeTime: io.WriteTime,
			}
			continue
		}

		// Calculate IO busy time delta
		deltaRead := io.ReadTime - prev.readTime
		deltaWrite := io.WriteTime - prev.writeTime
		deltaIO := deltaRead + deltaWrite

		// Update stored values
		prev.readTime = io.ReadTime
		prev.writeTime = io.WriteTime

		// IO busy time delta in ms over the collection interval
		intervalMs := m.IntervalSeconds * 1000.0
		busy := float64(deltaIO) / intervalMs * 100.0
		if busy > 100.0 {
			busy = 100.0
		}
		if busy < 0 {
			busy = 0
		}

		results = append(results, deviceUtilization{
			name:        name,
			usedPercent: busy,
		})
	}

	return results
}

// collectNetwork returns per-interface network utilization percentages (0-100)
// based on throughput relative to each interface's detected bandwidth.
// Returns empty slice if data is not yet available (first call).
func (m *Metric) collectNetwork() []deviceUtilization {
	stats, err := net.IOCounters(true) // true = per-interface
	if err != nil {
		log.Debugf("overall: failed to get network io counters: %v", err)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var results []deviceUtilization

	for _, stat := range stats {
		name := stat.Name

		// Skip loopback and virtual interfaces
		if isVirtualInterface(name) {
			continue
		}

		prev, exists := m.prevNetIO[name]
		if !exists {
			// First time seeing this interface, store initial values
			m.prevNetIO[name] = &netIOSnapshot{
				bytesRecv: stat.BytesRecv,
				bytesSent: stat.BytesSent,
			}
			continue
		}

		// Calculate deltas
		deltaRecv := stat.BytesRecv - prev.bytesRecv
		deltaSent := stat.BytesSent - prev.bytesSent

		// Update stored values
		prev.bytesRecv = stat.BytesRecv
		prev.bytesSent = stat.BytesSent

		// Use the higher of in/out throughput for utilization
		deltaMax := deltaRecv
		if deltaSent > deltaMax {
			deltaMax = deltaSent
		}

		// Get bandwidth for this interface
		bandwidth := m.netBandwidth[name]
		if bandwidth <= 0 {
			bandwidth = 1000 // Default 1 Gbps if unknown
		}

		// Convert bandwidth from Mbps to bytes/sec: Mbps * 1_000_000 / 8
		bandwidthBytesPerSec := bandwidth * 1000000.0 / 8.0

		throughputBytesPerSec := float64(deltaMax) / m.IntervalSeconds
		percent := throughputBytesPerSec / bandwidthBytesPerSec * 100.0
		if percent > 100.0 {
			percent = 100.0
		}
		if percent < 0 {
			percent = 0
		}

		results = append(results, deviceUtilization{
			name:        name,
			usedPercent: percent,
		})
	}

	return results
}

// isVirtualInterface returns true if the interface name looks like a virtual/loopback interface
func isVirtualInterface(name string) bool {
	// Common virtual interface prefixes across platforms
	virtualPrefixes := []string{
		"lo", "lo0",                                // Loopback
		"veth", "docker", "br-",                    // Docker/containers
		"virbr", "vnet",                            // Libvirt/KVM
		"utun", "awdl", "bridge", "llw", "ap", "XHC", // macOS virtual
		"vmnet",                                    // VMware
		"Loopback",                                 // Windows loopback
	}

	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) || name == prefix {
			return true
		}
	}
	return false
}

// calculateStatus determines the overall system status (green/yellow/red) and
// identifies the bottleneck subsystem (if any) based on configured thresholds.
// For disk_io and network, it includes the specific device name in the bottleneck.
func (m *Metric) calculateStatus(cpuPct, memPct, diskPct float64, maxDiskIO, maxNet deviceUtilization) (status, bottleneck string) {
	status = "green"
	bottleneck = ""

	// Subsystems to check with their utilization percentages
	type subsystem struct {
		name    string
		percent float64
		device  string // Optional device name for disk_io and network
	}

	subsystems := []subsystem{
		{"cpu", cpuPct, ""},
		{"memory", memPct, ""},
		{"disk", diskPct, ""},
	}

	// Add disk_io if we have data
	if maxDiskIO.name != "" {
		subsystems = append(subsystems, subsystem{"disk_io", maxDiskIO.usedPercent, maxDiskIO.name})
	}

	// Add network if we have data
	if maxNet.name != "" {
		subsystems = append(subsystems, subsystem{"network", maxNet.usedPercent, maxNet.name})
	}

	// Find the highest utilization and determine status
	var maxPercent float64
	var maxSubsystem subsystem
	for _, s := range subsystems {
		if s.percent > maxPercent {
			maxPercent = s.percent
			maxSubsystem = s
		}
	}

	// Determine status based on thresholds
	if maxPercent >= m.RedThreshold {
		status = "red"
		if maxSubsystem.device != "" {
			bottleneck = maxSubsystem.name + ":" + maxSubsystem.device
		} else {
			bottleneck = maxSubsystem.name
		}
	} else if maxPercent >= m.YellowThreshold {
		status = "yellow"
		if maxSubsystem.device != "" {
			bottleneck = maxSubsystem.name + ":" + maxSubsystem.device
		} else {
			bottleneck = maxSubsystem.name
		}
	}

	return status, bottleneck
}

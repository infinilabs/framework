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

// DefaultBandwidthMbps is the default network bandwidth in Mbps when auto-detection fails
const DefaultBandwidthMbps = 1000

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

	// Previous aggregate CPU times, used to compute the steal-time percentage
	// between collections. nil before the first sample is taken.
	prevCPUTimes *cpu.TimesStat

	// Previous TCP RetransSegs counter, used to compute retransmits per second.
	// hasPrevTCPRetrans guards against emitting bogus values on the first call
	// (when the counter is unknown) and after a counter reset.
	prevTCPRetrans    int64
	hasPrevTCPRetrans bool
}

// diskIOSnapshot stores previous I/O counters for a disk device.
// weightedIO is gopsutil's WeightedIO field (in milliseconds), used to derive
// the average queue depth (iostat "aqu-sz") across the collection interval.
type diskIOSnapshot struct {
	readTime   uint64
	writeTime  uint64
	weightedIO uint64
}

// netIOSnapshot stores previous I/O counters for a network interface
type netIOSnapshot struct {
	bytesRecv uint64
	bytesSent uint64
}

// deviceUtilization represents utilization info for a single device.
// queueDepth is only populated for disk I/O devices (avg outstanding requests
// over the collection interval, derived from WeightedIO).
type deviceUtilization struct {
	name        string
	usedPercent float64
	queueDepth  float64
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
	cpuPercent, cpuStealPercent := m.collectCPU()
	memPercent := m.collectMemory()
	diskPercent, diskInodePercent := m.collectDiskUsage()
	diskIODevices := m.collectDiskIO()
	netDevices := m.collectNetwork()
	tcpRetransPerSec, hasTCPRetrans := m.collectTCPRetrans()

	// --- CPU utilization ---
	fields["cpu.used_percent"] = cpuPercent
	fields["cpu.steal_percent"] = cpuStealPercent

	// --- Memory utilization ---
	fields["memory.used_percent"] = memPercent

	// --- Disk capacity utilization ---
	fields["disk.used_percent"] = diskPercent
	fields["disk.inodes_used_percent"] = diskInodePercent

	// --- Per-disk I/O utilization ---
	diskIOMap := util.MapStr{}
	var maxDiskIO deviceUtilization
	for _, dev := range diskIODevices {
		diskIOMap[dev.name] = util.MapStr{
			"used_percent": dev.usedPercent,
			"queue_depth":  dev.queueDepth,
		}
		if dev.usedPercent > maxDiskIO.usedPercent {
			maxDiskIO = dev
		}
	}
	if len(diskIOMap) > 0 {
		fields["disk_io.devices"] = diskIOMap
		fields["disk_io.used_percent"] = maxDiskIO.usedPercent
		fields["disk_io.queue_depth"] = maxDiskIO.queueDepth
		fields["disk_io.bottleneck_device"] = maxDiskIO.name
	}

	// --- Per-network interface utilization ---
	netMap := util.MapStr{}
	var maxNet deviceUtilization
	for _, dev := range netDevices {
		bw := m.netBandwidth[dev.name]
		if bw <= 0 {
			bw = DefaultBandwidthMbps // Default if unknown
		}
		netMap[dev.name] = util.MapStr{
			"used_percent":   dev.usedPercent,
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

	// --- TCP retransmits ---
	// Emitted as a raw quality signal for the front layer to interpret; it is
	// intentionally not folded into status/bottleneck because retransmits are
	// not directly comparable to capacity-based utilization percentages.
	// Only emit once we have a valid delta (skips first call and counter resets).
	if hasTCPRetrans {
		fields["network.tcp_retrans_per_sec"] = tcpRetransPerSec
	}

	// --- Calculate overall status and bottleneck ---
	// CPU stress can come from either user/system load or hypervisor steal;
	// disk pressure can come from either capacity or inode exhaustion. Pick
	// the worst signal in each subsystem so the bottleneck reflects reality.
	cpuStatus := cpuPercent
	if cpuStealPercent > cpuStatus {
		cpuStatus = cpuStealPercent
	}
	diskStatus := diskPercent
	if diskInodePercent > diskStatus {
		diskStatus = diskInodePercent
	}
	status, bottleneck := m.calculateStatus(cpuStatus, memPercent, diskStatus, maxDiskIO, maxNet)
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

// collectCPU returns the current overall CPU utilization percentage and the
// hypervisor steal-time percentage (both 0-100). The steal percentage is
// derived from the delta of cpu.Times() across collections; it is reported as
// 0 on the first call (no baseline) and on platforms where Steal is unavailable.
func (m *Metric) collectCPU() (usedPercent, stealPercent float64) {
	percents, err := cpu.Percent(0, false)
	if err != nil {
		log.Errorf("overall: failed to get cpu percent: %v", err)
	}
	if len(percents) > 0 {
		usedPercent = percents[0]
	}

	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		if err != nil {
			log.Debugf("overall: failed to get cpu times: %v", err)
		}
		return usedPercent, 0
	}
	cur := times[0]

	m.mu.Lock()
	prev := m.prevCPUTimes
	m.prevCPUTimes = &cur
	m.mu.Unlock()

	if prev == nil {
		// First sample; no delta available yet.
		return usedPercent, 0
	}

	deltaTotal := cur.Total() - prev.Total()
	deltaSteal := cur.Steal - prev.Steal
	if deltaTotal <= 0 || deltaSteal < 0 {
		// Counter reset or non-monotonic reading; skip this sample.
		return usedPercent, 0
	}
	stealPercent = deltaSteal / deltaTotal * 100.0
	if stealPercent > 100.0 {
		stealPercent = 100.0
	}
	return usedPercent, stealPercent
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

// collectDiskUsage returns disk capacity and inode utilization percentages
// (both 0-100). Inode usage is aggregated across all partitions (sum of used
// inodes over sum of total inodes); filesystems without inodes (e.g. some
// pseudo-filesystems, certain Windows volumes) are skipped.
func (m *Metric) collectDiskUsage() (usedPercent, inodesUsedPercent float64) {
	if runtime.GOOS == "darwin" {
		v, err := disk.Usage("/")
		if err != nil {
			log.Errorf("overall: failed to get disk usage: %v", err)
			return 0, 0
		}
		return v.UsedPercent, v.InodesUsedPercent
	}

	partitions, err := disk.Partitions(false)
	if err != nil || len(partitions) == 0 {
		log.Errorf("overall: failed to get disk partitions: %v", err)
		return 0, 0
	}
	var total, used, inodesTotal, inodesUsed uint64
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
		inodesTotal += v.InodesTotal
		inodesUsed += v.InodesUsed
	}
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100.0
	}
	if inodesTotal > 0 {
		inodesUsedPercent = float64(inodesUsed) / float64(inodesTotal) * 100.0
	}
	return usedPercent, inodesUsedPercent
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
				readTime:   io.ReadTime,
				writeTime:  io.WriteTime,
				weightedIO: io.WeightedIO,
			}
			continue
		}

		// Calculate IO busy time delta
		deltaRead := io.ReadTime - prev.readTime
		deltaWrite := io.WriteTime - prev.writeTime
		deltaIO := deltaRead + deltaWrite
		deltaWeighted := io.WeightedIO - prev.weightedIO

		// Update stored values
		prev.readTime = io.ReadTime
		prev.writeTime = io.WriteTime
		prev.weightedIO = io.WeightedIO

		// IO busy time delta in ms over the collection interval
		intervalMs := m.IntervalSeconds * 1000.0
		busy := float64(deltaIO) / intervalMs * 100.0
		if busy > 100.0 {
			busy = 100.0
		}
		if busy < 0 {
			busy = 0
		}

		// Average queue depth over the interval (iostat "aqu-sz"):
		// WeightedIO is the cumulative weighted time spent doing I/Os in ms,
		// so dividing the delta by the interval in ms yields the average
		// number of in-flight requests. WeightedIO is always 0 on platforms
		// that don't populate it (e.g. macOS), which correctly yields 0.
		var queueDepth float64
		if intervalMs > 0 {
			queueDepth = float64(deltaWeighted) / intervalMs
		}
		if queueDepth < 0 {
			queueDepth = 0
		}

		results = append(results, deviceUtilization{
			name:        name,
			usedPercent: busy,
			queueDepth:  queueDepth,
		})
	}

	return results
}

// collectTCPRetrans returns the TCP retransmits-per-second rate computed from
// the delta of the kernel's RetransSegs counter. The boolean return is false
// on the first call (no baseline), on platforms where ProtoCounters is not
// supported (currently non-Linux), or when the counter is unavailable / has
// been reset (negative delta).
func (m *Metric) collectTCPRetrans() (float64, bool) {
	if m.IntervalSeconds <= 0 {
		return 0, false
	}
	counters, err := net.ProtoCounters([]string{"tcp"})
	if err != nil || len(counters) == 0 {
		// Platforms without ProtoCounters support (darwin, windows, ...) end
		// up here; log at debug level to avoid spamming production logs.
		if err != nil {
			log.Debugf("overall: failed to get tcp proto counters: %v", err)
		}
		return 0, false
	}
	cur, ok := counters[0].Stats["RetransSegs"]
	if !ok {
		return 0, false
	}

	m.mu.Lock()
	prev := m.prevTCPRetrans
	hadPrev := m.hasPrevTCPRetrans
	m.prevTCPRetrans = cur
	m.hasPrevTCPRetrans = true
	m.mu.Unlock()

	if !hadPrev {
		return 0, false
	}
	delta := cur - prev
	if delta < 0 {
		// Counter reset (e.g. kernel restart, namespace change); skip.
		return 0, false
	}
	return float64(delta) / m.IntervalSeconds, true
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
			bandwidth = DefaultBandwidthMbps // Default if unknown
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
		"lo", "lo0", // Loopback
		"veth", "docker", "br-", // Docker/containers
		"virbr", "vnet", // Libvirt/KVM
		"utun", "awdl", "bridge", "llw", "ap", "XHC", // macOS virtual
		"vmnet",    // VMware
		"Loopback", // Windows loopback
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

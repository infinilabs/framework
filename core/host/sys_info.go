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

package host

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"infini.sh/framework/core/errors"
	"runtime"
	"time"
)

var CollectNetIOLastTime time.Time
var NetIOUsageLast *NetIOUsageInfo
var CollectDiskIOLastTime time.Time
var DiskIOUsageLast *DiskIOUsageInfo

func GetCPUInfo() (physicalCnt int, logicalCnt int, totalPercent float64, modelName string, err error) {
	physicalCnt, err = cpu.Counts(false) //物理内核数
	if err != nil {
		return 0, 0, 0, "", err
	}
	logicalCnt, err = cpu.Counts(true) //逻辑内核数
	if err != nil {
		return 0, 0, 0, "", err
	}
	totalPercents, err := cpu.Percent(time.Millisecond * 200, false) //过去200毫秒cpu使用率
	if len(totalPercents) > 0 {
		totalPercent = totalPercents[0] //这个使用率
	}
	if err != nil {
		return 0, 0, 0, "", err
	}
	cpuInfos, _ := cpu.Info()
	for _, info := range cpuInfos {
		modelName = info.ModelName
	}
	return physicalCnt, logicalCnt, totalPercent, modelName, nil
}

func GetDiskInfo() (total uint64, free uint64, used uint64, usedPercent float64, err error) {

	if runtime.GOOS == "darwin" {
		statMac, err := diskUsage("/")
		if err != nil {
			return 0, 0, 0, 0, err
		}
		return statMac.Total, statMac.Free, statMac.Total - statMac.Free, float64(statMac.Total - statMac.Free) / float64(statMac.Total) * 100.00, nil
	}

	partitions, err := disk.Partitions(false)
	if err != nil || len(partitions) == 0 {
		return 0, 0, 0, 0, err
	}

	var stat *disk.UsageStat
	for _, disk := range partitions {
		if disk.Device == "" {
			log.Errorf("Could not get device info %v", disk)
			continue
		}
		stat, err = diskUsage(disk.Mountpoint)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		total += stat.Total
		free += stat.Free
		used += stat.Used
	}
	usedPercent = float64(total - free) / float64(total) * 100.00
	return total, free, used, usedPercent, nil
}

func diskUsage(mountPoint string) (*disk.UsageStat, error) {
	path := mountPoint
	v, err := disk.Usage(path)
	if err != nil {
		return nil, err
	}
	if v.Path != path {
		return nil, errors.New(fmt.Sprintf("get disk usage, target path: %s, result path: %s", path, v.Path))
	}
	return v, nil
}

func GetOSInfo() (hostName string, bootTime uint64, platform string, platformVersion string, kernelVersion string, kernelArch string, err error) {
	v, err := host.Info()
	if err != nil {
		return "", 0, "", "", "", "", err
	}
	empty := &host.InfoStat{}
	if v == empty {
		return "", 0, "", "", "", "", errors.New(fmt.Sprintf("Could not get hostinfo %v", v))
	}
	if v.Procs == 0 {
		return "", 0, "", "", "", "", errors.New("Could not determine the number of host processes")
	}
	hostName = v.Hostname
	bootTime = v.BootTime
	platform = v.Platform
	platformVersion = v.PlatformVersion
	kernelVersion = v.KernelVersion
	kernelArch = v.KernelArch
	return hostName, bootTime, platform, platformVersion, kernelVersion, kernelArch, nil
}

func GetMemoryInfo() (total uint64, available uint64, used uint64, usedPercent float64, err error) {
	if runtime.GOOS == "solaris" {
		return 0, 0, 0, 0, errors.New("Only .Total is supported on Solaris")
	}

	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	empty := &mem.VirtualMemoryStat{}
	if v == empty {
		return 0, 0, 0, 0, errors.New("computer.memoryInfo: mem.VirtualMemoryStat is empty")
	}

	total = v.Used + v.Free + v.Buffers + v.Cached
	available = v.Available
	used = v.Used
	usedPercent = v.UsedPercent
	return total, available, used, usedPercent, nil
}

func GetSwapInfo() (total uint64, used uint64, free uint64, usedPercent float64, err error) {
	v, err := mem.SwapMemory()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	empty := &mem.SwapMemoryStat{}
	if v == empty {
		return 0, 0, 0, 0, errors.New("computer.swapInfo: mem.SwapMemoryStat is empty")
	}
	total = v.Total
	used = v.Used
	free = v.Free
	usedPercent = v.UsedPercent
	return total, used, free, usedPercent, nil
}

func GetAllUsageInfo() (*Usage, error) {
	usage := &Usage{}
	var err error
	usage.NetIOUsage, err = GetNetIOUsage()
	if err != nil {
		return nil, errors.Wrap(err,"get usage.NetIOUsage err")
	}
	usage.DiskUsage, err = GetDiskUsage()
	if err != nil {
		return nil, errors.Wrap(err,"get usage.DiskUsage err")
	}
	usage.DiskIOUsage, err = GetDiskIOUsageInfo()
	if err != nil {
		return nil, errors.Wrap(err,"get usage.DiskIOUsage err")
	}
	usage.MemoryUsage, usage.SwapMemoryUsage, err = GetMemoryUsage()
	if err != nil {
		return nil, errors.Wrap(err,"get usage.MemoryUsage err")
	}
	usage.CPUPercent = GetCPUUsageInfo()
	return usage, nil
}

func GetCPUUsageInfo() float64 {
	_, _, cupPercent, _, err := GetCPUInfo()
	if err != nil {
		log.Errorf("get GetCPUUsageInfo err: %v",err)
		return 0
	}
	return cupPercent
}

func GetDiskUsage() (*DiskUsageInfo, error) {
	diskUsage := &DiskUsageInfo{}
	var err error
	diskUsage.Total, diskUsage.Free, diskUsage.Used, diskUsage.UsedPercent, err = GetDiskInfo()
	return diskUsage, err
}

func GetDiskIOUsageInfo() (*DiskIOUsageInfo, error) {
	ret, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, errors.New("instance.GetDiskIOUsageInfo: failed, result is empty")
	}
	empty := disk.IOCountersStat{}
	var readBytes uint64 = 0
	var writeBytes uint64 = 0
	var writeTimeCost uint64 = 0
	var readTimeCost uint64 = 0
	for _, io := range ret {
		if io != empty {
			readBytes += io.ReadBytes
			writeBytes += io.WriteBytes
			readTimeCost += io.ReadTime
			writeTimeCost += io.WriteTime
		}
	}
	if DiskIOUsageLast == nil {
		return &DiskIOUsageInfo{
			ReadBytes:     readBytes,
			WriteBytes:    writeBytes,
			ReadTimeCost:  readTimeCost,
			WriteTimeCost: writeTimeCost,
		}, nil
	}
	diskIOUsage := &DiskIOUsageInfo{}
	var timePeriod uint64 = 1
	timePassed := uint64(time.Now().Unix() - CollectDiskIOLastTime.Unix())
	if timePassed == 0 {
		timePassed = 1
	}
	var toByte uint64 = 1
	//var toKb uint64 = 1000
	//var toMB uint64 = 1000 * 1000
	diskIOUsage.ReadBytes = (readBytes - DiskIOUsageLast.ReadBytes) / toByte * timePeriod / timePassed    //byte
	diskIOUsage.WriteBytes = (writeBytes - DiskIOUsageLast.WriteBytes) / toByte * timePeriod / timePassed //byte
	diskIOUsage.WriteTimeCost = (writeTimeCost - DiskIOUsageLast.WriteTimeCost) * timePeriod / timePassed //ms
	diskIOUsage.ReadTimeCost = (readTimeCost - DiskIOUsageLast.ReadTimeCost) * timePeriod / timePassed    //ms

	CollectDiskIOLastTime = time.Now()
	DiskIOUsageLast.WriteBytes = writeBytes
	DiskIOUsageLast.ReadBytes = readBytes
	DiskIOUsageLast.ReadTimeCost = readTimeCost
	DiskIOUsageLast.WriteTimeCost = writeTimeCost
	return diskIOUsage, nil
}

func GetNetIOUsage() (*NetIOUsageInfo, error) {
	stats, err := net.IOCounters(false)
	if err != nil {
		log.Errorf("Could not get GetNetIOUsage: %v", err)
	}
	if len(stats) < 1 {
		return nil, errors.New("instance.GetNetIOUsage: failed")
	}
	stat := stats[0]
	usage := &NetIOUsageInfo{}
	if NetIOUsageLast == nil {
		usage.BytesSent = stat.BytesSent
		usage.BytesRecv = stat.BytesRecv
		usage.PacketsRecv = stat.PacketsRecv
		usage.PacketsSent = stat.PacketsSent
		return usage, nil
	}
	var timePeriod uint64 = 1
	timePassed := uint64(time.Now().Unix() - CollectNetIOLastTime.Unix())
	if timePassed == 0 {
		timePassed = 1
	}
	var toByte uint64 = 1
	//var toKb uint64 = 1000
	usage.BytesRecv = (stat.BytesRecv - NetIOUsageLast.BytesRecv) / toByte * timePeriod / timePassed //byte
	usage.BytesSent = (stat.BytesSent - NetIOUsageLast.BytesSent) / toByte * timePeriod / timePassed //byte
	usage.PacketsRecv = (stat.PacketsRecv - NetIOUsageLast.PacketsRecv) * timePeriod / timePassed
	usage.PacketsSent = (stat.PacketsSent - NetIOUsageLast.PacketsSent) * timePeriod / timePassed
	CollectNetIOLastTime = time.Now()
	NetIOUsageLast.BytesRecv = stat.BytesRecv
	NetIOUsageLast.BytesSent = stat.BytesSent
	NetIOUsageLast.PacketsRecv = stat.PacketsRecv
	NetIOUsageLast.PacketsSent = stat.PacketsSent
	return usage, nil
}

func GetMemoryUsage() (*MemoryUsageInfo, *SwapMemoryUsageInfo, error) {

	memoryUsage := &MemoryUsageInfo{}
	swapUsage := &SwapMemoryUsageInfo{}
	var err error
	memoryUsage.Total, memoryUsage.Available, memoryUsage.Used, memoryUsage.UsedPercent, err = GetMemoryInfo()
	if err != nil {
		return nil, nil, err
	}
	swapUsage.Total, swapUsage.Used, swapUsage.Free, swapUsage.UsedPercent, err = GetSwapInfo()
	if err != nil {
		return nil, nil, err
	}
	return memoryUsage, swapUsage, nil
}

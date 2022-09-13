/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package disk

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/disk"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"runtime"
	"strings"
)

type MetricType string

const typeUsage = MetricType("usage")
const typeIOQS = MetricType("iops")

type Metric struct {
	Enabled     bool     `config:"enabled"`
	Metrics     []string `config:"metrics"`
	prevCounter ioCounter
}

type ioCounter struct {
	prevReadBytes     uint64
	prevWriteBytes    uint64
	prevReadTimeCost  uint64
	prevWriteTimeCost uint64
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled:     true,
		prevCounter: ioCounter{},
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	log.Debugf("disk stats will be included for %v", me.Metrics)
	return me, nil
}

func (m *Metric) Collect() error {

	var err error
	for _, v := range m.Metrics {
		switch MetricType(strings.ToLower(v)) {
		case typeUsage:
			err = m.collectUsage()
			if err != nil {
				log.Error(err)
			}
		case typeIOQS:
			err = m.collectIO()
			if err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func (m *Metric) collectUsage() error {
	partitions, err := disk.Partitions(false)
	if err != nil || len(partitions) == 0 {
		return nil
	}

	var total, free, used uint64
	var stat *disk.UsageStat
	for _, disk := range partitions {
		if disk.Device == "" {
			log.Errorf("Could not get device info %v", disk)
			continue
		}
		stat, err = diskUsage(disk.Mountpoint)
		if err != nil {
			return err
		}
		event.Save(event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "disk_partition_usage",
				Datatype: "gauge",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"disk_partition_usage": util.MapStr{
						"partition":      disk.Mountpoint,
						"total_in_bytes": stat.Total,
						"free_in_bytes":  stat.Free,
						"used_in_bytes":  stat.Used,
						"used_percent":   stat.UsedPercent,
					},
				},
			},
		})
		total += stat.Total
		free += stat.Free
		used += stat.Used
	}

	if runtime.GOOS == "darwin" {
		statMac, err := diskUsage("/")
		if err != nil {
			return err
		}
		event.Save(event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "disk_usage_summary",
				Datatype: "accumulate",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"disk_usage_summary": util.MapStr{
						"partition":      "all",
						"total_in_bytes": statMac.Total,
						"free_in_bytes":  statMac.Free,
						"used_in_bytes":  statMac.Total - statMac.Free,
						"used_percent":  float64(statMac.Total - statMac.Free) / float64(statMac.Total) * 100.00,
					},
				},
			},
		})
		return nil
	}

	event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "disk_usage_summary",
			Datatype: "accumulate",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"disk_usage_summary": util.MapStr{
					"partition":      "all",
					"total_in_bytes": total,
					"free_in_bytes":  free,
					"used_in_bytes":  used,
					"used_percent":   float64(used) / float64(total) * 100.00,
				},
			},
		},
	})
	return nil
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

func diskPartition() []disk.PartitionStat {

	ret, err := disk.Partitions(false)
	if err != nil || len(ret) == 0 {
		log.Error(err)
	}
	return ret
}

func (m *Metric) collectIO() error {
	ret, err := disk.IOCounters()
	if err != nil {
		return err
	}

	var readBytes, writeBytes, writeTimeCost, readTimeCost uint64

	for _, io := range ret {
		if io != (disk.IOCountersStat{}) {
			readBytes += io.ReadBytes
			writeBytes += io.WriteBytes
			readTimeCost += io.ReadTime
			writeTimeCost += io.WriteTime
		}
	}

	event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "disk_io_summary",
			Datatype: "accumulate",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"disk_io_summary": util.MapStr{
					"read_in_bytes":    readBytes,
					"read_time_in_ms":  readTimeCost,
					"write_in_bytes":   writeBytes,
					"write_time_in_ms": writeTimeCost,
				},
			},
		},
	})

	if m.prevCounter != (ioCounter{}) {
		event.Save(event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "disk_iops",
				Datatype: "gauge",
				Labels: util.MapStr{
					"ip": util.GetLocalIPs(),
				},
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"disk_iops": util.MapStr{
						"read_in_bytes":    readBytes - m.prevCounter.prevReadBytes,
						"read_time_in_ms":  readTimeCost - m.prevCounter.prevReadTimeCost,
						"write_in_bytes":   writeBytes - m.prevCounter.prevWriteBytes,
						"write_time_in_ms": writeTimeCost - m.prevCounter.prevWriteTimeCost,
					},
				},
			},
		})
	}

	m.prevCounter.prevReadBytes = readBytes
	m.prevCounter.prevReadTimeCost = readTimeCost
	m.prevCounter.prevWriteBytes = writeBytes
	m.prevCounter.prevWriteTimeCost = writeTimeCost
	return nil
}

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

package disk

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v3/disk"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
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

	if !m.Enabled{
		return nil
	}

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
				if util.ContainStr(err.Error(), "not implemented yet") {
					m.Enabled = false
				}
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
		event.Save(&event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "filesystem",
				Datatype: "gauge",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"filesystem": util.MapStr{
						"mount_point":  disk.Mountpoint,
						"total.bytes":  stat.Total,
						"free.bytes":   stat.Free,
						"used.bytes":   stat.Used,
						"used.percent": stat.UsedPercent,
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
		event.Save(&event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "filesystem_summary",
				Datatype: "accumulate",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"filesystem_summary": util.MapStr{
						"mount_point":  "/",
						"total.bytes":  statMac.Total,
						"free.bytes":   statMac.Free,
						"used.bytes":   statMac.Total - statMac.Free,
						"used.percent": float64(statMac.Total-statMac.Free) / float64(statMac.Total) * 100.00,
					},
				},
			},
		})
		return nil
	} else {
		event.Save(&event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "filesystem_summary",
				Datatype: "accumulate",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"filesystem_summary": util.MapStr{
						"mount_point":  "/",
						"total.bytes":  total,
						"free.bytes":   free,
						"used.bytes":   used,
						"used.percent": float64(used) / float64(total) * 100.00,
					},
				},
			},
		})
	}

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

	event.Save(&event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "diskio_summary",
			Datatype: "accumulate",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"diskio_summary": util.MapStr{
					"read.bytes":       readBytes,
					"read.time_in_ms":  readTimeCost,
					"write.bytes":      writeBytes,
					"write.time_in_ms": writeTimeCost,
				},
			},
		},
	})

	if m.prevCounter != (ioCounter{}) {
		event.Save(&event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "diskio",
				Datatype: "gauge",
				Labels: util.MapStr{
					"ip": util.GetLocalIPs(),
				},
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"diskio": util.MapStr{
						"read.bytes":       readBytes - m.prevCounter.prevReadBytes,
						"read.time_in_ms":  readTimeCost - m.prevCounter.prevReadTimeCost,
						"write.bytes":      writeBytes - m.prevCounter.prevWriteBytes,
						"write.time_in_ms": writeTimeCost - m.prevCounter.prevWriteTimeCost,
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

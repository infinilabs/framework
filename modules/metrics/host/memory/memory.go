/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package memory

import (
	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/mem"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"runtime"
	"strings"
)

type MetricType string

const typeSwap = MetricType("swap")
const typeMemory = MetricType("memory")

type Metric struct {
	Enabled bool     `config:"enabled"`
	Metrics []string `config:"metrics"`
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled: true,
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	log.Debugf("memory stats will be included for %v", me.Metrics)
	return me, nil
}

func (m *Metric) Collect() error {

	var err error
	for _, v := range m.Metrics {
		switch MetricType(strings.ToLower(v)) {
		case typeSwap:
			err = m.collectSwap()
			if err != nil {
				log.Error(err)
			}
		case typeMemory:
			err = m.collectMemory()
			if err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func (m *Metric) collectSwap() error {
	v, err := mem.SwapMemory()
	if err != nil {
		return err
	}
	empty := &mem.SwapMemoryStat{}
	if v == empty {
		return errors.New("computer.swapInfo: mem.SwapMemoryStat is empty")
	}
	return event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "memory_swap",
			Datatype: "accumulate",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"memory_swap": util.MapStr{
					"total_in_bytes": v.Total,
					"free_in_bytes":  v.Free,
					"used_in_bytes":  v.Used,
					"used_percent":   v.UsedPercent,
				},
			},
		},
	})
}

func (m *Metric) collectMemory() error {
	if runtime.GOOS == "solaris" {
		return errors.New("Only .Total is supported on Solaris. skipped")
	}
	v, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	empty := &mem.VirtualMemoryStat{}
	if v == empty {
		return errors.New("computer.memoryInfo: mem.VirtualMemoryStat is empty")
	}

	total := v.Used + v.Free + v.Buffers + v.Cached
	switch runtime.GOOS {
	case "windows":
		total = v.Used + v.Available
	case "darwin", "openbsd":
		total = v.Used + v.Free + v.Cached + v.Inactive
	case "freebsd":
		total = v.Used + v.Free + v.Cached + v.Inactive + v.Laundry
	}
	return event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "memory",
			Datatype: "accumulate",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"memory": util.MapStr{
					"total_in_bytes":     total,
					"available_in_bytes": v.Available,
					"used_in_bytes":      v.Used,
					"used_percent":       v.UsedPercent,
				},
			},
		},
	})
}

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package cpu

import (
	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"time"
)

type MetricType string

const TypeIdle = MetricType("idle")
const TypeSystem = MetricType("system")
const TypeUser = MetricType("user")
const TypeIoWait = MetricType("iowait")
const TypeLoad = MetricType("load")

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

	log.Debugf("cpu stats will be included for %v", me.Metrics)
	return me, nil
}

func (m *Metric) Collect() error {

	cpuTotal, err := cpu.Times(false)
	if err != nil {
		log.Errorf("error %v", err)
	}
	if len(cpuTotal) == 0 {
		log.Error("could not get CPUs", err)
	}
	cpuStats := cpuTotal[0]

	mapStr := util.MapStr{}
	for _, metric := range m.Metrics {
		switch MetricType(metric) {
		case TypeIoWait:
			mapStr.Put(string(TypeIoWait), cpuStats.Iowait)
		case TypeLoad:
			v, err := load.Avg()
			if err != nil {
				return err
			}
			mapStr.Put(string(TypeLoad), util.MapStr{
				"load1":  v.Load1,
				"load5":  v.Load5,
				"load15": v.Load15,
			})
		case TypeSystem:
			mapStr.Put(string(TypeSystem), cpuStats.System)
		case TypeIdle:
			mapStr.Put(string(TypeIdle), cpuStats.Idle)
		case TypeUser:
			mapStr.Put(string(TypeUser), cpuStats.User)
		}
	}
	percents, err := cpu.Percent(time.Millisecond*100, false) //cpu use percent in 100ms
	if err != nil {
		log.Error("get cpu use percent failed: %v", err)
		return nil
	}
	var percent float64
	if len(percents) > 0 {
		percent = percents[0]
	}
	mapStr.Put("used_percent", percent) //cpu使用率
	return event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "host",
			Name:     "cpu_usage",
			Datatype: "gauge",
		},
		Fields: util.MapStr{
			"host": util.MapStr{
				"cpu_usage": mapStr,
			},
		},
	})
}

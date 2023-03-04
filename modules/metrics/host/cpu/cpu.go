/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package cpu

import (
	"strconv"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

type MetricType string

const TypeIdle = MetricType("idle")
const TypeSystem = MetricType("system")
const TypeUser = MetricType("user")
const TypeIoWait = MetricType("iowait")
const TypeLoad = MetricType("load")

type Metric struct {
	Enabled     bool     `config:"enabled"`
	Metrics     []string `config:"metrics"`
	prevCounter cpuCounter
}

type cpuCounter struct {
	preIOWait float64
	preSystem float64
	preIdle   float64
	preUser   float64
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled:     true,
		prevCounter: cpuCounter{},
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
		return nil
	}
	if len(cpuTotal) == 0 {
		log.Error("could not get CPUs")
		return nil
	}
	cpuStats := cpuTotal[0]
	if m.prevCounter != (cpuCounter{}) {
		log.Debugf("Collect CPU metrics, origin: %v", util.MustToJSON(cpuStats))
		mapStr := util.MapStr{}
		for _, metric := range m.Metrics {
			switch MetricType(metric) {
			case TypeIoWait:
				iowait := (cpuStats.Iowait - m.prevCounter.preIOWait) / 10.00 * 100.00 //interval: 10s. convert to %
				retIOWait := KeepZero(iowait)
				mapStr.Put(string(TypeIoWait), retIOWait)
			case TypeSystem:
				system := (cpuStats.System - m.prevCounter.preSystem) / 10.00 * 100.00 //interval: 10s. convert to %
				retSystem := KeepZero(system)
				mapStr.Put(string(TypeSystem), retSystem)
			case TypeIdle:
				idle := 100.00 - (cpuStats.Idle-m.prevCounter.preIdle)/10.00 //interval: 10s. convert to %
				retIdle := KeepZero(idle)
				mapStr.Put(string(TypeIdle), retIdle)
			case TypeUser:
				user := (cpuStats.User - m.prevCounter.preUser) / 10.00 * 100.00
				retIdle := KeepZero(user)
				mapStr.Put(string(TypeUser), retIdle) //interval: 10s. convert to %
			case TypeLoad:
				v, err := load.Avg()
				if err != nil {
					log.Error(err)
					continue
				}
				retL1 := KeepZero(v.Load1)
				retL5 := KeepZero(v.Load5)
				retL15 := KeepZero(v.Load15)
				mapStr.Put(string(TypeLoad), util.MapStr{
					"load1":  retL1,
					"load5":  retL5,
					"load15": retL15,
				})
			}
		}
		percents, err := cpu.Percent(0, false) //cpu use percent in 100ms
		if err != nil {
			log.Error("get cpu used percent failed: %v", err)
		}
		var percent float64
		if len(percents) > 0 {
			percent = percents[0]
		}
		mapStr.Put("used_percent", percent)
		log.Debugf("Collect CPU metrics: %v", util.MustToJSON(mapStr))
		err = event.Save(event.Event{
			Metadata: event.EventMetadata{
				Category: "host",
				Name:     "cpu",
				Datatype: "gauge",
			},
			Fields: util.MapStr{
				"host": util.MapStr{
					"cpu": mapStr,
				},
			},
		})
		if err != nil {
			log.Error(err)
		}
	}

	m.prevCounter.preIdle = cpuStats.Idle
	m.prevCounter.preSystem = cpuStats.System
	m.prevCounter.preIOWait = cpuStats.Iowait
	m.prevCounter.preUser = cpuStats.User
	return nil
}

type KeepZero float64

func (f KeepZero) MarshalJSON() ([]byte, error) {
	if float64(f) == float64(int(f)) {
		return []byte(strconv.FormatFloat(float64(f), 'f', 1, 32)), nil
	}
	return []byte(strconv.FormatFloat(float64(f), 'f', -1, 32)), nil
}

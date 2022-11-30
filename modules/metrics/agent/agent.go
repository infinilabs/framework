/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"github.com/shirou/gopsutil/process"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"os"
	"time"
)

type Metric struct {
	Enabled       bool `config:"enabled"`
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled: true,
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	return me, nil
}

func (m *Metric) Collect() error {

	checkPid := os.Getpid()
	p, err := process.NewProcess(int32(checkPid))
	if err != nil {
		return err
	}
	cupPercent, err := p.CPUPercent()
	if err != nil {
		return err
	}
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return err
	}
	event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "agent",
			Name:     "agent_basic",
			Datatype: "gauge",
		},
		Fields: util.MapStr{
			"agent": util.MapStr{
				"agent_basic": util.MapStr{
					"uptime_in_ms":    time.Since(env.GetStartTime()).Milliseconds(),
					"cpu":             util.FormatNumber(cupPercent),
					"memory_in_bytes": memInfo.RSS,
					"memory":          util.ByteSize(memInfo.RSS),
				},
			},
		},
	})
	return nil
}
/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package instance

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

type Metric struct {
	Enabled bool `config:"enabled"`
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

	data, err := stats.StatsMap()
	if err != nil {
		panic(err)
	}

	return event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "instance",
			Name:     global.Env().GetAppLowercaseName(),
			Version:  global.Env().GetVersion(),
			Datatype: "gauge",
			Labels: util.MapStr{
				"id":   global.Env().SystemConfig.NodeConfig.ID,
				"name": global.Env().SystemConfig.NodeConfig.Name,
				"ip":   global.Env().SystemConfig.NodeConfig.IP,
			},
		},
		Fields: util.MapStr{
			"instance": data,
		},
	})
}

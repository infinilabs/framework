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

package instance

import (
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/event"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/stats"
	"github.com/rubyniu105/framework/core/util"
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

	return event.Save(&event.Event{
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

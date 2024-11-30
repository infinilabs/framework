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

package statsd

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/quipo/statsd"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/stats"
	"sync"
	"time"
)

type StatsDConfig struct {
	Enabled           bool          `config:"enabled"`
	Host              string        `config:"host"`
	Port              int           `config:"port"`
	Namespace         string        `config:"namespace"`
	Protocol          string        `config:"protocol"`
	IntervalInSeconds int `config:"interval_in_seconds"`
	BufferSize        int           `config:"buffer_size"`
}
type StatsDModule struct {
	 statsdInited bool
	 statsdclient *statsd.StatsdClient
	 buffer *statsd.StatsdBuffer
	 l1 sync.RWMutex
}


func (module *StatsDModule) Setup() {
}


var defaultStatsdConfig = StatsDConfig{
	Enabled:           false,
	Host:              "localhost",
	Port:              8125,
	Namespace:         "app.",
	Protocol:          "udp",
	IntervalInSeconds: 1,
}

func (module *StatsDModule) Name() string {
	return "statsd"
}

func (module *StatsDModule) Start() error {
	if module.statsdInited {
		panic(errors.New("statsd not inited"))
	}

	config := defaultStatsdConfig
	//cfg.Unpack(&config)
	env.ParseConfig("statsd", &config)
	if !config.Enabled {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	module.l1.Lock()
	defer module.l1.Unlock()
	module.statsdclient = statsd.NewStatsdClient(addr, config.Namespace)

	log.Debug("statsd connec to, ", addr, ",prefix:", config.Namespace)

	var err error
	if config.Protocol == "tcp" {
		err = module.statsdclient.CreateTCPSocket()
	} else {
		err = module.statsdclient.CreateSocket()
	}
	if nil != err {
		log.Warn(err)
		return err
	}

	interval := time.Second * time.Duration(config.IntervalInSeconds) // aggregate stats and flush every 2 seconds
	if config.BufferSize<=0{
		config.BufferSize=100
	}
	module.buffer = statsd.NewStatsdBuffer(interval,config.BufferSize, module.statsdclient)

	module.statsdInited = true

	stats.Register(module)
	return nil
}

func (module *StatsDModule) Stop() error {
	if module.statsdclient != nil {
		module.statsdclient.Close()
	}
	return nil
}

func (module *StatsDModule) Absolute(category, key string, value int64) {

	if !module.statsdInited {
		return
	}
	module.buffer.Absolute(category+"."+key, value)
}

func (module *StatsDModule) Increment(category, key string) {

	module.IncrementBy(category, key, 1)
}

func (module *StatsDModule) IncrementBy(category, key string, value int64) {
	if !module.statsdInited {
		return
	}
	module.buffer.Incr(category+"."+key, value)
}

func (module *StatsDModule) Decrement(category, key string) {
	module.DecrementBy(category, key, 1)
}

func (module *StatsDModule) DecrementBy(category, key string, value int64) {
	if !module.statsdInited {
		return
	}
	module.buffer.Decr(category+"."+key, value)
}

func (module *StatsDModule) Timing(category, key string, v int64) {
	if !module.statsdInited {
		return
	}
	module.buffer.Timing(category+"."+key, v)

}

func (module *StatsDModule) GetTimestamp(category, key string)(time.Time, error) {
	return time.Now(), errors.New("not support")
}

func (module *StatsDModule) RecordTimestamp(category, key string, value time.Time) {

}

func (module *StatsDModule) Gauge(category, key string, v int64) {
	if !module.statsdInited {
		return
	}
	module.buffer.Gauge(category+"."+key, v)
}

func (module *StatsDModule) Stat(category, key string) int64 {
	return 0
}

func (module *StatsDModule) StatsAll() string {
	return ""
}

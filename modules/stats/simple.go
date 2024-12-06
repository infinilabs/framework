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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package stats

import (
	"errors"
	"github.com/rubyniu105/framework/lib/status"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/api"
	httprouter "github.com/rubyniu105/framework/core/api/router"
	"github.com/rubyniu105/framework/core/env"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/stats"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/lib/bytebufferpool"
	"github.com/rubyniu105/framework/lib/lock_free/queue"
	"github.com/segmentio/encoding/json"
	"github.com/shirou/gopsutil/v3/process"
)

func (module SimpleStatsModule) Name() string {
	return "simple_stats"
}

type SimpleStatsConfig struct {
	Enabled                  bool `config:"enabled"`
	Persist                  bool `config:"persist"`
	NoBuffer                 bool `config:"no_buffer"`
	IncludeStorageStatsInAPI bool `config:"include_storage_stats_in_api"`
	BufferSize               int  `config:"buffer_size"`
	FlushIntervalInMs        int  `config:"flush_interval_ms"`
}

func (module *SimpleStatsModule) Setup() {

	module.config = &SimpleStatsConfig{
		Enabled:                  true,
		Persist:                  true,
		NoBuffer:                 true,
		BufferSize:               1000,
		IncludeStorageStatsInAPI: true,
		FlushIntervalInMs:        1000,
	}
	env.ParseConfig("stats", module.config)

	if !module.config.Enabled {
		return
	}

	if module.config.Persist {
		module.dataPath = path.Join(global.Env().GetDataDir(), "stats")
		os.MkdirAll(module.dataPath, 0755)
	}

	module.data = &Stats{
		raw: module.config.NoBuffer,
		cfg: module.config,
	}
	module.initStats("simple")

	if !module.config.NoBuffer {
		module.data.q = queue.NewQueue(uint32(module.config.BufferSize))
	}

	stats.Register(module.data)

	//register api
	api.HandleAPIMethod(api.GET, "/stats", module.StatsAction)
	api.HandleAPIMethod(api.GET, "/stats/prometheus", module.PrometheusStatsAction)
	api.HandleAPIMethod(api.GET, "/debug/goroutines", module.GoroutinesAction)

	//if global.Env().IsDebug{
	api.HandleAPIMethod(api.GET, "/debug/pool/bytes", module.BufferItemStatsAction)
	//}

	api.HandleAPIMethod(api.GET, "/_local/files/_list", module.ListDirFs)
	api.HandleAPIMethod(api.GET, "/_local/files/:file/_list", module.ListDirFs)
	api.HandleAPIMethod(api.DELETE, "/_local/files/:file", module.DeleteDataFile)
}

func (module *SimpleStatsModule) Start() error {
	if !module.config.Enabled {
		return nil
	}

	if !module.config.NoBuffer {
		go func() {
			defer func() {
				if !global.Env().IsDebug {
					if r := recover(); r != nil {
						var v string
						switch r.(type) {
						case error:
							v = r.(error).Error()
						case runtime.Error:
							v = r.(runtime.Error).Error()
						case string:
							v = r.(string)
						}
						log.Error(v)
					}
				}
			}()
			for {
				v, ok, n := module.data.q.Get()
				if ok {
					x, ok := v.(StatItem)
					if ok {
						module.data.initData(x.Category, x.Key)
						switch x.Op {
						case Incr:
							module.data.l.Lock()
							(*module.data.Data)[x.Category][x.Key] += x.Value
							module.data.l.Unlock()
							break
						case Decr:
							module.data.l.Lock()
							(*module.data.Data)[x.Category][x.Key] -= x.Value
							module.data.l.Unlock()
							break
						}
					}
				}
				if n == 0 {
					if module.config.FlushIntervalInMs < 100 {
						module.config.FlushIntervalInMs = 1000
					}
					time.Sleep(time.Duration(module.config.FlushIntervalInMs) * time.Millisecond)
				}
			}
		}()
	}
	return nil
}

func (module *SimpleStatsModule) Stop() error {
	if module.config == nil {
		return nil
	}

	if !module.config.Enabled {
		return nil
	}

	module.data.closed = true
	if module.config.Persist {
		module.data.l.Lock()
		defer module.data.l.Unlock()
		v, _ := json.Marshal(module.data.Data)
		_, err := util.FilePutContentWithByte(path.Join(module.dataPath, strings.ToLower(module.data.ID)), v)
		if err != nil {
			log.Error(err)
		}
		log.Trace("save stats db,", module.data.ID)
	}
	return nil
}

type SimpleStatsModule struct {
	api.Handler
	config   *SimpleStatsConfig
	data     *Stats
	dataPath string
}

const Incr = "incr"
const Decr = "decr"

type StatItem struct {
	Op       string
	Category string
	Key      string
	Value    int64
}

type Stats struct {
	l         sync.RWMutex
	ID        string                       `storm:"id,unique" json:"id" gorm:"not null;unique;primary_key"`
	Data      *map[string]map[string]int64 `storm:"inline" json:"data,omitempty"`
	timestamp map[string]time.Time         //for last timestamps, no need persist
	closed    bool
	raw       bool
	q         *queue.EsQueue
	cfg       *SimpleStatsConfig
}

func (s *Stats) initData(category, key string) {

	s.l.Lock()
	defer s.l.Unlock()
	_, ok := (*s.Data)[category]
	if !ok {
		(*s.Data)[category] = make(map[string]int64)
	}
	_, ok1 := (*s.Data)[category][key]
	if !ok1 {
		(*s.Data)[category][key] = 0
	}
	runtime.Gosched()
}

func (s *Stats) Increment(category, key string) {
	s.IncrementBy(category, key, 1)
}

func (s *Stats) IncrementBy(category, key string, value int64) {
	if s.closed {
		return
	}

	if s.raw {
		s.initData(category, key)
		s.l.Lock()
		(*s.Data)[category][key] += value
		s.l.Unlock()
	} else {
		s.q.Put(StatItem{Op: Incr, Category: category, Key: key, Value: value})
	}

	runtime.Gosched()
}

func (s *Stats) Absolute(category, key string, value int64) {
	s.initData(category, key)
	s.l.Lock()
	(*s.Data)[category][key] = value
	s.l.Unlock()
	runtime.Gosched()
}

func (s *Stats) Decrement(category, key string) {
	s.DecrementBy(category, key, 1)
}

func (s *Stats) DecrementBy(category, key string, value int64) {
	if s.closed {
		return
	}

	if s.raw {
		s.initData(category, key)
		s.l.Lock()
		(*s.Data)[category][key] -= value
		s.l.Unlock()
	} else {
		s.q.Put(StatItem{Op: Decr, Category: category, Key: key, Value: value})
	}
	runtime.Gosched()
}

func (s *Stats) Timing(category, key string, v int64) {

}

func (s *Stats) GetTimestamp(category, key string) (time.Time, error) {
	s.l.RLock()
	defer s.l.RUnlock()
	v, ok := (s.timestamp)[category+"."+key]
	if !ok {
		return time.Time{}, errors.New("not found")
	}
	return v, nil
}

func (s *Stats) RecordTimestamp(category, key string, v time.Time) {
	s.l.Lock()
	(s.timestamp)[category+"."+key] = v
	s.l.Unlock()
	runtime.Gosched()
}

func (s *Stats) Gauge(category, key string, v int64) {
	s.initData(category, key)
	s.l.Lock()
	(*s.Data)[category][key] = v
	s.l.Unlock()
	runtime.Gosched()
}

func (s *Stats) Stat(category, key string) int64 {
	s.initData(category, key)
	s.l.RLock()
	v := ((*s.Data)[category][key])
	s.l.RUnlock()
	return v
}

var m runtime.MemStats

func (s *Stats) StatsAll() string {
	s.l.RLock()
	defer s.l.RUnlock()

	result := util.MapStr{}
	result["stats"] = *s.Data

	result["pool"] = bytebufferpool.BuffStats()

	//update system metrics
	checkPid := os.Getpid()
	p, _ := process.NewProcess(int32(checkPid))
	mem, err := p.MemoryInfo()
	if err != nil {
		log.Error(err)
		return util.ToJson(result, false)
	}
	empty := process.MemoryInfoStat{}
	if mem == nil || *mem == empty {
		log.Errorf("could not get memory info %v\n", mem)
		return util.ToJson(result, false)
	}

	cpuPercent, err := p.CPUPercent()
	if err != nil {
		log.Error(err)
		return util.ToJson(result, false)
	}
	times, err := p.Times()
	if err != nil {
		log.Error(err)
		return util.ToJson(result, false)
	}

	runtime.ReadMemStats(&m)

	systemStats := map[string]int64{
		"uptime_in_ms": time.Since(env.GetStartTime()).Milliseconds(),
		"cpu":          int64(cpuPercent),
		"user_in_ms":   int64(times.User * 1000),
		"sys_in_ms":    int64(times.System * 1000),
		"mem":          int64(mem.RSS),
		"goroutines":   int64(runtime.NumGoroutine()),
		"objects":      int64(m.HeapObjects),
		"stack":        int64(m.StackInuse),
		"mspan":        int64(m.MSpanInuse),
		"gc":           int64(m.NumGC),
		"cgo_calls":    int64(runtime.NumCgoCall()),
	}

	if s != nil && s.cfg.IncludeStorageStatsInAPI {
		diskStats := status.DiskPartitionUsage(global.Env().GetDataDir())
		result["disk"] = diskStats
		storage, _ := status.DirSize(global.Env().GetDataDir())
		systemStats["store"] = int64(storage) //maybe too heavy to call frequently
	}

	result["system"] = systemStats

	return util.ToJson(result, false)
}

func (module *SimpleStatsModule) initStats(id string) {
	module.data.l.Lock()
	defer module.data.l.Unlock()

	module.data.ID = id

	if module.config.Persist {
		v, err := util.FileGetContent(path.Join(module.dataPath, strings.ToLower(module.data.ID)))

		if err == nil && v != nil {
			d := map[string]map[string]int64{}
			err = json.Unmarshal(v, &d)
			if err != nil {
				log.Error(err)
			}
			module.data.Data = &d
		}
	}

	if module.data.Data == nil {
		module.data.Data = &map[string]map[string]int64{}
		log.Trace("inited stats map")
	}

	module.data.timestamp = map[string]time.Time{}
}

func (handler SimpleStatsModule) BufferItemStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj := bytebufferpool.DumpBufferItemSize()
	handler.WriteJSON(w, obj, 200)
}

func (handler SimpleStatsModule) ListDirFs(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	file := handler.GetParameter(req, "file")
	files, err := status.ListDiskFs(file)
	result := util.MapStr{}
	status := 200
	if err != nil {
		result["error"] = err.Error()
		status = 500
	}
	result["disk"] = files
	handler.WriteJSON(w, result, status)
}

func (handler SimpleStatsModule) DeleteDataFile(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	file := ps.ByName("file")
	err := status.DeleteDataFile(file)
	result := util.MapStr{}
	status := 200
	if err != nil {
		result["error"] = err.Error()
		status = 500
	}
	handler.WriteJSON(w, result, status)
}

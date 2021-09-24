package stats

import (
	"github.com/segmentio/encoding/json"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
)

func (module SimpleStatsModule) Name() string {
	return "Stats"
}

var data *Stats
var dataPath string
var config *SimpleStatsConfig

type SimpleStatsConfig struct {
	Persist bool `config:"persist"`
}

func (module *SimpleStatsModule) Setup(cfg *Config) {

	config = &SimpleStatsConfig{
		Persist: true,
	}
	env.ParseConfig("stats", config)

	if config.Persist {
		dataPath = path.Join(global.Env().GetDataDir(), "stats")
		os.MkdirAll(dataPath, 0755)
	}

	data = &Stats{}
	data.initStats("simple")
	stats.Register(data)

	//register api
	api.HandleAPIFunc("/stats", module.StatsAction)
}

func (module *SimpleStatsModule) Start() error {

	return nil
}

func (module *SimpleStatsModule) Stop() error {

	if config.Persist {
		data.l.Lock()
		defer data.l.Unlock()
		v, _ := json.Marshal(data.Data)
		_, err := util.FilePutContentWithByte(path.Join(dataPath, strings.ToLower(data.ID)), v)
		if err != nil {
			log.Error(err)
		}
		log.Trace("save stats db,", data.ID)
	}

	return nil
}

type SimpleStatsModule struct {
	config *SimpleStatsConfig
	api.Handler
}

type Stats struct {
	l    sync.RWMutex
	ID   string                       `storm:"id,unique" json:"id" gorm:"not null;unique;primary_key"`
	Data *map[string]map[string]int64 `storm:"inline" json:"data,omitempty"`
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
	s.initData(category, key)
	s.l.Lock()
	(*s.Data)[category][key] += value
	s.l.Unlock()
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
	s.initData(category, key)
	s.l.Lock()
	(*s.Data)[category][key] -= value
	s.l.Unlock()
	runtime.Gosched()
}

func (s *Stats) Timing(category, key string, v int64) {

}

func (s *Stats) Gauge(category, key string, v int64) {

}

func (s *Stats) Stat(category, key string) int64 {
	s.initData(category, key)
	s.l.RLock()
	v := ((*s.Data)[category][key])
	s.l.RUnlock()
	return v
}

func (s *Stats) StatsAll() *[]byte {
	s.l.RLock()
	defer s.l.RUnlock()
	b, _ := json.MarshalIndent((*s.Data), "", " ")
	return &b
}

func (s *Stats) initStats(id string) {
	s.l.Lock()
	defer s.l.Unlock()

	s.ID = id

	if config.Persist {
		v, err := util.FileGetContent(path.Join(dataPath, strings.ToLower(data.ID)))

		if err == nil && v != nil {
			d := map[string]map[string]int64{}
			err = json.Unmarshal(v, &d)
			if err != nil {
				log.Error(err)
			}
			s.Data = &d
		}
	}

	if s.Data == nil {
		s.Data = &map[string]map[string]int64{}
		log.Trace("inited stats map")
	}
}

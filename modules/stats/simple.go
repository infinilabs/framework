package stats

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/persist"
	"github.com/infinitbyte/framework/core/stats"
	"runtime"
	"sync"
)

func (module SimpleStatsModule) Name() string {
	return "Stats"
}

const id = "stats"

var data *Stats

const KVBucketKey string = "kv"

func (module SimpleStatsModule) Start(cfg *Config) {

	data = &Stats{}
	data.initStats()
	stats.Register(data)
}

func (module SimpleStatsModule) Stop() error {
	data.l.Lock()
	defer data.l.Unlock()
	v, _ := json.Marshal(data.Data)
	data.ID = id
	err := persist.AddValue(string(KVBucketKey), []byte(data.ID), v)
	if err != nil {
		log.Error(err)
	}
	log.Trace("save stats db,", data.ID)
	return nil
}

type SimpleStatsModule struct {
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

func (s *Stats) initStats() {
	s.l.Lock()
	defer s.l.Unlock()

	s.ID = id
	v, err := persist.GetValue(string(KVBucketKey), []byte(s.ID))

	if err == nil && v != nil {
		d := map[string]map[string]int64{}
		err = json.Unmarshal(v, &d)
		if err != nil {
			log.Error(err)
		}
		s.Data = &d
	}

	if s.Data == nil {
		s.Data = &map[string]map[string]int64{}
		log.Trace("inited stats map")
	}
}

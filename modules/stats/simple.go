package stats

import (
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"github.com/struCoder/pidusage"
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
	"time"
)

func (module SimpleStatsModule) Name() string {
	return "Stats"
}



type SimpleStatsConfig struct {
	Enabled bool `config:"enabled"`
	Persist bool `config:"persist"`
	BufferSize int `config:"buffer_size"`
}

func (module *SimpleStatsModule) Setup(cfg *Config) {

	module.config = &SimpleStatsConfig{
		Enabled: true,
		Persist: true,
		BufferSize: 1000,
	}
	env.ParseConfig("stats", module.config)

	if !module.config.Enabled{
		return
	}

	if module.config.Persist {
		module.dataPath = path.Join(global.Env().GetDataDir(), "stats")
		os.MkdirAll(module.dataPath, 0755)
	}

	module.data = &Stats{}
	module.initStats("simple")
	module.data.buffer=make(chan StatItem,module.config.BufferSize)
	stats.Register(module.data)

	//register api
	api.HandleAPIMethod(api.GET,"/stats", module.StatsAction)
}

func (module *SimpleStatsModule) Start() error {
	if !module.config.Enabled{
		return nil
	}

	go func() {
		for x := range module.data.buffer{
			module.data.initData(x.Category,x.Key)
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
	}()
	return nil
}

func (module *SimpleStatsModule) Stop() error {
	if !module.config.Enabled{
		return nil
	}

	module.data.closed=true
	close(module.data.buffer)
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
	config *SimpleStatsConfig
	data *Stats
	dataPath string
}

const Incr ="incr"
const Decr ="decr"

type StatItem struct {
	Op string
	Category string
	Key string
	Value int64
}



type Stats struct {
	l    sync.RWMutex
	ID   string                       `storm:"id,unique" json:"id" gorm:"not null;unique;primary_key"`
	Data *map[string]map[string]int64 `storm:"inline" json:"data,omitempty"`
	closed bool
	buffer chan StatItem
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
	if s.closed{
		return
	}

	s.buffer <- StatItem{Op: Incr,Category: category,Key: key,Value: value}
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
	if s.closed{
		return
	}

	s.buffer <- StatItem{Op: Decr,Category: category,Key: key,Value: value}
}

func (s *Stats) Timing(category, key string, v int64) {

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

func (s *Stats) StatsAll() *[]byte {
	s.l.Lock()
	defer s.l.Unlock()

	//update system metrics
	sysInfo, err := pidusage.GetStat(os.Getpid())
	if err==nil{
		(*s.Data)["system"]=map[string]int64{
			"uptime_in_ms":time.Since(env.GetStartTime()).Milliseconds(),
			"cpu":int64(sysInfo.CPU),
			"mem":int64(sysInfo.Memory),
		}
	}

	b, _ := json.MarshalIndent((*s.Data), "", " ")
	return &b
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
}

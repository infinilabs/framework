package filter

import (
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/modules/filter/nutsdb"
	nuts	"github.com/xujiajun/nutsdb"
	"time"

	"path"
)

type FilterModule struct {
	Sync bool  `config:"sync"`
	RWMode string  `config:"rw_mode"`
	StartFileLoadingMode string  `config:"start_file_loading_mode"`
	SegmentSize int64  `config:"segment_size"`

	handler *nutsdb.NutsdbKVFilter
}

func (module *FilterModule) Name() string {
	return "Filter"
}


func (module *FilterModule) Setup(cfg *Config) {
	module.RWMode="file_io"
	module.StartFileLoadingMode="file_io"
	module.SegmentSize=8 * 1024 * 1024
	ok,err:=env.ParseConfig("filter", module)
	if ok&&err!=nil{
		panic(err)
	}

	opt := nuts.Options{
		EntryIdxMode:         nuts.HintKeyValAndRAMIdxMode,
		SegmentSize:          8 * 1024 * 1024,
		NodeNum:              1,
		RWMode:               nuts.FileIO,
		SyncEnable:           module.Sync,
		StartFileLoadingMode: nuts.FileIO,
	}

	if module.RWMode=="fileio"{
		opt.RWMode=nuts.FileIO
	}else{
		opt.RWMode=nuts.MMap
	}

	if module.StartFileLoadingMode=="fileio"{
		opt.StartFileLoadingMode=nuts.FileIO
	}else{
		opt.StartFileLoadingMode=nuts.MMap
	}

	if module.SegmentSize>0{
		opt.SegmentSize=module.SegmentSize
	}

	opt.Dir = path.Join(global.Env().GetDataDir(),"kvdb")

	module.handler= &nutsdb.NutsdbKVFilter{
		Options: opt,
	}
	filter.Register("kv", module.handler)
	kv.Register("kv", module.handler)

}

func (module *FilterModule) Start() error {
	if module.handler != nil {
		t := time.Now()
		module.handler.Open()
		log.Tracef("open kv db elapsed: %s", time.Since(t))
		Init(module.handler)
	}
	return nil
}

func (module *FilterModule) Stop() error {
	if module.handler != nil {
		err := module.handler.Close()
		if err != nil {
			log.Error(err)
		}
	}
	return nil

}

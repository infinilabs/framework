/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package nutsdb

import (
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	nuts	"github.com/xujiajun/nutsdb"
	"time"

	"path"
)

type Module struct {
	Sync bool  `config:"sync"`
	RWMode string  `config:"rw_mode"`
	StartFileLoadingMode string  `config:"start_file_loading_mode"`
	SegmentSize int64  `config:"segment_size"`

	handler *NutsdbFilter
}

func (module *Module) Name() string {
	return "Nutsdb"
}

func (module *Module) Setup(cfg *Config) {
	module.RWMode="file_io"
	module.StartFileLoadingMode="file_io"
	module.SegmentSize=8 * 1024 * 1024
	ok,err:=env.ParseConfig("nutsdb", module)
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

	opt.Dir = path.Join(global.Env().GetDataDir(),"nutsdb")

	module.handler= &NutsdbFilter{
		Options: opt,
	}
	filter.Register("nutsdb", module.handler)
	kv.Register("nutsdb", module.handler)

}

func (module *Module) Start() error {
	if module.handler != nil {
		t := time.Now()
		module.handler.Open()
		log.Tracef("open nutsdb elapsed: %s", time.Since(t))
		Init(module.handler)
	}
	return nil
}

func (module *Module) Stop() error {
	if module.handler != nil {
		err := module.handler.Close()
		if err != nil {
			log.Error(err)
		}
	}
	return nil

}

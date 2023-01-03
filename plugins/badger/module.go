/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/module"
	"path"
	"github.com/dgraph-io/badger/v3"
)

type Module struct {
	SingleBucketMode bool   `config:"single_bucket_mode"`
	Path             string `config:"path"`
	InMemoryMode     bool   `config:"memory_mode"`
	SyncWrites     bool   `config:"sync_writes"`
	MemTableSize     int64   `config:"mem_table_size"`
	ValueLogFileSize     int64   `config:"value_log_file_size"`
	ValueThreshold     int64   `config:"value_threshold"`
	ValueLogMaxEntries     uint32   `config:"value_log_max_entries"`
	NumMemtables     int   `config:"num_mem_tables"`

	NumLevelZeroTables     int   `config:"num_level0_tables"`
	NumLevelZeroTablesStall     int   `config:"num_level0_tables_stall"`
	bucket           *badger.DB
	closed bool
}

func (module *Module) Name() string {
	return "Badger"
}

func (module *Module) Setup() {
	ok,err:=env.ParseConfig("badger", module)
	if ok&&err!=nil{
		panic(err)
	}
	if module.Path==""{
		module.Path = path.Join(global.Env().GetDataDir(),"badger")
	}
	filter.Register("badger", module)
	kv.Register("badger", module)

}

func (module *Module) Start() error {
	module.closed=false
	return module.Open()
}

func (module *Module) Stop() error {
	module.closed=true
	return module.Close()
}

func init() {
	module.RegisterSystemModule(&Module{
		MemTableSize: 1*1024*1024,
		ValueLogFileSize: 1*1024*1024,
		ValueLogMaxEntries: 1000,
		NumMemtables: 1,
		NumLevelZeroTables: 1,
		NumLevelZeroTablesStall: 2,
		SingleBucketMode: true,
	})
}
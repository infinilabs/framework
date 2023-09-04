/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"github.com/dgraph-io/badger/v4"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/module"
	"path"
)

type Config struct {
	Enabled bool `config:"enabled"`

	SingleBucketMode   bool   `config:"single_bucket_mode"`
	Path               string `config:"path"`
	InMemoryMode       bool   `config:"memory_mode"`
	SyncWrites         bool   `config:"sync_writes"`
	MemTableSize       int64  `config:"mem_table_size"`
	ValueLogFileSize   int64  `config:"value_log_file_size"`
	ValueThreshold     int64  `config:"value_threshold"`
	ValueLogMaxEntries uint32 `config:"value_log_max_entries"`
	NumMemtables       int    `config:"num_mem_tables"`

	NumLevelZeroTables      int `config:"num_level0_tables"`
	NumLevelZeroTablesStall int `config:"num_level0_tables_stall"`
}

type Module struct {
	cfg    *Config
	bucket *badger.DB
	closed bool
}

func (module *Module) Name() string {
	return "badger"
}

func (module *Module) Setup() {
	module.cfg = &Config{
		Enabled:                 true,
		MemTableSize:            10 * 1024 * 1024,
		ValueLogFileSize:        1<<30 - 1, //1g
		ValueThreshold:          1048576,   //1m
		ValueLogMaxEntries:      1000000,   //1million
		NumMemtables:            1,
		NumLevelZeroTables:      1,
		NumLevelZeroTablesStall: 2,
		SingleBucketMode:        true,
	}
	ok, err := env.ParseConfig("badger", module.cfg)
	if ok && err != nil {
		panic(err)
	}
	if module.cfg.Path == "" {
		module.cfg.Path = path.Join(global.Env().GetDataDir(), "badger")
	}

	if module.cfg.Enabled {
		filter.Register("badger", module)
		kv.Register("badger", module)
	}

}

func (module *Module) Start() error {
	if module.cfg == nil {
		return nil
	}

	if module.cfg.Enabled {
		module.closed = false
		return module.Open()
	}

	return nil
}

func (module *Module) Stop() error {

	if module.cfg == nil {
		return nil
	}

	if module.cfg != nil && module.cfg.Enabled {
		module.closed = true
		return module.Close()
	}

	return nil

}

func init() {
	module.RegisterModuleWithPriority(&Module{}, -100)
}

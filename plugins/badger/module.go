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

	ValueLogGCEnabled           bool    `config:"value_log_gc_enabled"`
	ValueLogDiscardRatio        float64 `config:"value_log_gc_discard_ratio"`
	ValueLogGCIntervalInSeconds int     `config:"value_log_gc_interval_in_seconds"`
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
		Enabled:                     true,
		MemTableSize:                10 * 1024 * 1024,
		ValueLogFileSize:            1<<30 - 1, //1g
		ValueThreshold:              1048576,   //1m
		ValueLogMaxEntries:          1000000,   //1million
		ValueLogGCEnabled:           true,
		ValueLogGCIntervalInSeconds: 120,
		ValueLogDiscardRatio:        0.5,

		NumMemtables:            1,
		NumLevelZeroTables:      1,
		NumLevelZeroTablesStall: 2,
		SingleBucketMode:        true,
	}
	ok, err := env.ParseConfig("badger", module.cfg)
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
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

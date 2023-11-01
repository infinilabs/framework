/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package simple_kv

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
	"os"
	"path"
)

type Config struct {
	Enabled bool `config:"enabled"`
	Path               string `config:"path"`
	SyncWrites         bool   `config:"sync_writes"`
}

type SimpleKV struct {
	cfg    *Config
	closed bool
	kvstore *KVStore
}

func (module *SimpleKV) Name() string {
	return "simple_kv"
}

func (module *SimpleKV) Setup() {
	module.cfg = &Config{
		Enabled:                 true,
	}
	ok, err := env.ParseConfig("simple_kv", module.cfg)
	if ok && err != nil  &&global.Env().SystemConfig.Configs.PanicOnConfigError{
		panic(err)
	}
	if module.cfg.Path == "" {
		module.cfg.Path = path.Join(global.Env().GetDataDir(), "simple_kv")
	}

	if !util.FileExists(module.cfg.Path){
		os.MkdirAll(module.cfg.Path, 0755)
	}


	if module.cfg.Enabled {
		filter.Register("simple_kv", module)
		kv.Register("simple_kv", module)
	}

	module.kvstore=NewKVStore(path.Join(module.cfg.Path,"last_state"), path.Join(module.cfg.Path,"wal"))
}

func (module *SimpleKV) Start() error {
	if module.cfg == nil {
		return nil
	}

	if module.cfg.Enabled {
		module.closed = false
	}

	return nil
}

func (module *SimpleKV) Stop() error {

	if module.cfg == nil {
		return nil
	}

	if module.cfg != nil && module.cfg.Enabled {
		module.closed = true
	}

	return nil

}

//func init() {
//	module.RegisterModuleWithPriority(&SimpleKV{}, -100)
//}

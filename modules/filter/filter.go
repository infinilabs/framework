package filter

import (
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/modules/filter/nutsdb"
)

type FilterModule struct {
	handler *nutsdb.NutsdbKVFilter
}

type FilterConfig struct {
	KV *KVFilterConfig
}

type KVFilterConfig struct{}

func (module *FilterModule) Name() string {
	return "Filter"
}


func (module *FilterModule) Setup(cfg *Config) {

	module.handler= &nutsdb.NutsdbKVFilter{}
	filter.Register("kv", module.handler)
	kv.Register("kv", module.handler)

}

func (module *FilterModule) Start() error {
	if module.handler != nil {
		module.handler.Open()
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

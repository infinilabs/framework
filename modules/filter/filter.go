package filter

import (
	log "github.com/cihub/seelog"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/filter"
	"github.com/infinitbyte/framework/modules/filter/kv"
)

type FilterModule struct {
}

type FilterConfig struct {
	KV *KVFilterConfig
}

type KVFilterConfig struct{}

var (
	defaultConfig = FilterConfig{
		KV: &KVFilterConfig{},
	}
)

func (module FilterModule) Name() string {
	return "Filter"
}

var handler filter.Filter

func (module FilterModule) Setup(cfg *Config) {

	//init config
	cfg.Unpack(&defaultConfig)

	handler = kv.KVFilter{}
	filter.Register("kv", handler)
}

func (module FilterModule) Start() error {
	return nil
}

func (module FilterModule) Stop() error {
	if handler != nil {
		err := handler.Close()
		if err != nil {
			log.Error(err)
		}
	}
	return nil

}

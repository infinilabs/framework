package filter

import (
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/modules/filter/kv"
)

type FilterModule struct {
}

type FilterConfig struct {
	KV *KVFilterConfig
}

type KVFilterConfig struct{}

func (module FilterModule) Name() string {
	return "Filter"
}

var handler filter.Filter

func (module FilterModule) Setup(cfg *Config) {

	handler = kv.KVFilter{}
	filter.Register("kv", handler)
}

func (module FilterModule) Start() error {
	handler.Open()
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

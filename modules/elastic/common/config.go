package common

import (
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"strings"
)


type ORMConfig struct {
	Enabled      bool   `config:"enabled"`
	InitTemplate bool   `config:"init_template"`
	TemplateName string `config:"template_name"`
	IndexPrefix  string `config:"index_prefix"`
}

type StoreConfig struct {
	Enabled      bool   `config:"enabled"`
	IndexName  string `config:"index_name"`
}

type MonitoringConfig struct {
	Enabled       bool     `config:"enabled"`
	Interval      string   `config:"interval,omitempty"`
}

type ModuleConfig struct {
	Elasticsearch string      `config:"elasticsearch"`
	ORMConfig     ORMConfig   `config:"orm"`
	StoreConfig   StoreConfig `config:"store"`
	MonitoringConfig   MonitoringConfig `config:"monitoring"`

}

func InitClientWithConfig(esConfig elastic.ElasticsearchConfig)(client elastic.API) {

	var ver string
	if esConfig.Version == "" || esConfig.Version == "auto" {
		esVersion, err := adapter.ClusterVersion(&esConfig)
		if err != nil {
			panic(err)
			return
		}
		ver = esVersion.Version.Number
		esConfig.Version = ver
	} else {
		ver = esConfig.Version
	}

	if util.SuffixStr(esConfig.Endpoint,"/"){
		esConfig.Endpoint=esConfig.Endpoint[:len(esConfig.Endpoint)-1]
	}

	if strings.HasPrefix(ver, "8.") {
		api := new(adapter.ESAPIV8)
		api.Config = esConfig
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "7.") {
		api := new(adapter.ESAPIV7)
		api.Config = esConfig
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "6.") {
		api := new(adapter.ESAPIV6)
		api.Config = esConfig
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "5.") {
		api := new(adapter.ESAPIV5)
		api.Config = esConfig
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "2.") {
		api := new(adapter.ESAPIV2)
		api.Config = esConfig
		api.Version = ver
		client = api
	} else {
		api := new(adapter.ESAPIV0)
		api.Config = esConfig
		api.Version = ver
		client = api
	}

	return client
}


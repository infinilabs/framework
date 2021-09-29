package common

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
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
	Enabled   bool   `config:"enabled"`
	IndexName string `config:"index_name"`
}

type MonitoringConfig struct {
	Enabled  bool   `config:"enabled"`
	Interval string `config:"interval,omitempty"`
}

type HealthCheckConfig struct {
	Enabled  bool   `config:"enabled"`
	Interval string `config:"interval,omitempty"`
}

type ModuleConfig struct {
	Elasticsearch       string    `config:"elasticsearch"`
	RemoteConfigEnabled bool      `config:"remote_configs"`
	ORMConfig           ORMConfig `config:"orm"`
	StoreConfig                    StoreConfig      `config:"store"`
	MonitoringConfig               MonitoringConfig `config:"monitoring"`
	HealthCheckConfig              HealthCheckConfig `config:"health_check"`
}

func InitClientWithConfig(esConfig elastic.ElasticsearchConfig) (client elastic.API, err error) {

	var (
		ver string
	)
	if esConfig.Version == "" || esConfig.Version == "auto" {
		esConfig.Version, _ = adapter.GetMajorVersion(elastic.GetOrInitMetadata(&esConfig))
	} else {
		ver = esConfig.Version
	}

	if ver==""&&esConfig.Version!=""{
		ver = esConfig.Version
	}

	if strings.HasPrefix(ver, "8.") {
		api := new(adapter.ESAPIV8)
		api.Elasticsearch = esConfig.ID
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "7.") {
		api := new(adapter.ESAPIV7)
		api.Elasticsearch = esConfig.ID
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "6.") {
		api := new(adapter.ESAPIV6)
		api.Elasticsearch = esConfig.ID
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "5.") {
		api := new(adapter.ESAPIV5)
		api.Elasticsearch = esConfig.ID
		api.Version = ver
		client = api
	} else if strings.HasPrefix(ver, "2.") {
		api := new(adapter.ESAPIV2)
		api.Elasticsearch = esConfig.ID
		api.Version = ver
		client = api
	} else {
		api := new(adapter.ESAPIV0)
		api.Elasticsearch = esConfig.Name
		api.Version = ver
		client = api
	}

	return client, nil
}

func InitElasticInstance(esConfig elastic.ElasticsearchConfig) (elastic.API, error) {
	if !esConfig.Enabled {
		log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
		return nil, nil
	}
	client, err := InitClientWithConfig(esConfig)
	if err != nil {
		log.Error("elasticsearch ", esConfig.Name, err)
		return client, err
	}
	elastic.RegisterInstance(esConfig.ID, esConfig, client)
	return client, err
}

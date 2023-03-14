package common

import (
	"fmt"
	"strings"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/credential"
	elastic "infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"infini.sh/framework/modules/elastic/adapter/easysearch"
	"infini.sh/framework/modules/elastic/adapter/elasticsearch"
	"infini.sh/framework/modules/elastic/adapter/opensearch"
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

type CheckConfig struct {
	Enabled  bool   `config:"enabled"`
	Interval string `config:"interval,omitempty"`
}

type ModuleConfig struct {
	Elasticsearch               string      `config:"elasticsearch"`
	RemoteConfigEnabled         bool        `config:"remote_configs"`
	ORMConfig                   ORMConfig   `config:"orm"`
	StoreConfig                 StoreConfig `config:"store"`
	HealthCheckConfig           CheckConfig `config:"health_check"`
	NodeAvailabilityCheckConfig CheckConfig `config:"availability_check"`
	MetadataRefresh             CheckConfig `config:"metadata_refresh"`
	ClusterSettingsCheckConfig  CheckConfig `config:"cluster_settings_check"`
	ClientTimeout               string      `config:"client_timeout"`
}

func InitClientWithConfig(esConfig elastic.ElasticsearchConfig) (client elastic.API, err error) {

	var (
		ver string
	)
	if esConfig.Version == "" || esConfig.Version == "auto" {
		verInfo, _ := adapter.ClusterVersion(elastic.GetOrInitMetadata(&esConfig))
		if verInfo != nil {
			esConfig.Version = verInfo.Version.Number
			esConfig.Distribution = verInfo.Version.Distribution
		}
	} else {
		ver = esConfig.Version
	}

	if ver == "" && esConfig.Version != "" {
		ver = esConfig.Version
	}

	if ver == "" {
		//can't fetch any version
		ver = "1.0.0"
	}

	apiVer := elastic.Version{
		Number:       ver,
		Distribution: esConfig.Distribution,
	}

	if esConfig.Distribution == elastic.Easysearch {
		return newEasysearchClient(esConfig.ID, apiVer)
	} else if esConfig.Distribution == elastic.Opensearch {
		return newOpensearchClient(esConfig.ID, apiVer)
	}
	sem, err := util.ParseSemantic(ver)
	if err != nil {
		return nil, err
	}

	major, minor := sem.Major(), sem.Minor()
	if major >= 8 {
		api := new(elasticsearch.ESAPIV8)
		api.Elasticsearch = esConfig.ID
		api.Version = apiVer
		client = api
	} else if major == 7 {
		if minor >= 7 {
			api := new(elasticsearch.ESAPIV7_7)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		} else if minor >= 3 {
			api := new(elasticsearch.ESAPIV7_3)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		} else {
			api := new(elasticsearch.ESAPIV7)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		}
	} else if major == 6 {
		if minor >= 6 {
			api := new(elasticsearch.ESAPIV6_6)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		} else {
			api := new(elasticsearch.ESAPIV6)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		}
	} else if major == 5 {
		if minor >= 6 {
			api := new(elasticsearch.ESAPIV5_6)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		} else if minor >= 4 {
			api := new(elasticsearch.ESAPIV5_4)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		} else {
			api := new(elasticsearch.ESAPIV5)
			api.Elasticsearch = esConfig.ID
			api.Version = apiVer
			client = api
		}
	} else if major == 2 {
		api := new(elasticsearch.ESAPIV2)
		api.Elasticsearch = esConfig.ID
		api.Version = apiVer
		client = api
	} else {
		api := new(elasticsearch.ESAPIV0)
		api.Elasticsearch = esConfig.ID
		api.Version = apiVer
		client = api
	}

	return client, nil
}

func InitElasticInstanceWithoutMetadata(esConfig elastic.ElasticsearchConfig) (elastic.API, error) {
	if esConfig.ID == "" && esConfig.Name != "" {
		esConfig.ID = esConfig.Name
	}
	if !esConfig.Enabled {
		log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
		return nil, nil
	}
	client, err := InitClientWithConfig(esConfig)
	if err != nil {
		log.Error("elasticsearch ", esConfig.Name, err)
		return nil, err
	}
	elastic.RegisterInstance(esConfig, client)

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
	elastic.RegisterInstance(esConfig, client)

	originMeta := elastic.GetMetadata(esConfig.ID)
	initHealth := true
	if originMeta != nil {
		initHealth = originMeta.IsAvailable()
	}

	v := elastic.InitMetadata(&esConfig, initHealth)
	if v.Health == nil && originMeta != nil {
		v.Health = originMeta.Health
	}
	elastic.SetMetadata(esConfig.ID, v)
	return client, err
}

func GetBasicAuth(esConfig *elastic.ElasticsearchConfig) (basicAuth elastic.BasicAuth, err error) {
	if esConfig.BasicAuth != nil && esConfig.BasicAuth.Username != "" {
		basicAuth = *esConfig.BasicAuth
		return
	}
	if esConfig.CredentialID != "" {
		cred := credential.Credential{}
		cred.ID = esConfig.CredentialID
		_, err = orm.Get(&cred)
		if err != nil {
			return
		}
		var dv interface{}
		dv, err = cred.Decode()
		if err != nil {
			return
		}
		if auth, ok := dv.(elastic.BasicAuth); ok {
			basicAuth = auth
		}
	}
	return
}

func GetElasticClient(clusterID string) (elastic.API, error) {
	client := elastic.GetClientNoPanic(clusterID)
	if client != nil {
		return client, nil
	}
	conf := &elastic.ElasticsearchConfig{}
	conf.ID = clusterID
	exists, err := orm.Get(conf)
	if err != nil {
		return nil, err
	}
	if exists {
		return InitElasticInstance(*conf)
	}
	return nil, fmt.Errorf("cluster [%s] was not found", clusterID)
}

func GetClusterDocType(clusterID string) string {
	client := elastic.GetClient(clusterID)
	verInfo := client.GetVersion()
	switch verInfo.Distribution {
	case elastic.Easysearch:
		return "_doc"
	case elastic.Opensearch:
		return ""
	default:
		majorVersion := client.GetMajorVersion()
		if majorVersion >= 8 {
			return ""
		}
		if majorVersion < 7 {
			return "doc"
		} else {
			return "_doc"
		}
	}
}

func newOpensearchClient(clusterID string, version elastic.Version) (elastic.API, error) {
	if strings.HasPrefix(version.Number, "2.") {
		api := new(opensearch.APIV2)
		api.Elasticsearch = clusterID
		api.Version = version
		return api, nil
	}
	if strings.HasPrefix(version.Number, "1.") {
		api := new(opensearch.APIV1)
		api.Elasticsearch = clusterID
		api.Version = version
		return api, nil
	}
	return nil, fmt.Errorf("unsupport opensearch version [%v]", version.Number)
}

func newEasysearchClient(clusterID string, version elastic.Version) (elastic.API, error) {
	api := new(easysearch.APIV1)
	api.Elasticsearch = clusterID
	api.Version = version
	return api, nil
}

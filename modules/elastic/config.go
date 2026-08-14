package elastic

import "infini.sh/framework/modules/elastic/common"

type ModuleConfig struct {
	Enabled                           bool               `config:"enabled"`
	Elasticsearch                     string             `config:"elasticsearch"`
	RemoteConfigEnabled               bool               `config:"remote_configs"`
	ORMConfig                         common.ORMConfig   `config:"orm"`
	StoreConfig                       common.StoreConfig `config:"store"`
	HealthCheckConfig                 common.CheckConfig `config:"health_check"`
	NodeAvailabilityCheckConfig       common.CheckConfig `config:"availability_check"`
	MetadataRefresh                   common.CheckConfig `config:"metadata_refresh"`
	ClusterSettingsCheckConfig        common.CheckConfig `config:"cluster_settings_check"`
	ClientTimeout                     string             `config:"client_timeout"`
	DeadNodeAvailabilityCheckInterval string             `config:"dead_node_availability_check_interval,omitempty"`
	SkipInitMetadataOnStart           bool               `config:"skip_init_metadata_on_start"`
}

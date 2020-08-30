package config

import (
	"fmt"
	"infini.sh/framework/core/errors"
	"strings"
)

// ClusterConfig stores cluster settings
type ClusterConfig struct {
	Enabled                        bool          `config:"enabled"`
	Name                           string        `config:"name"`
	MinimumNodes                   int           `config:"minimum_nodes"`
	Seeds                          []string      `config:"seeds"`
	RPCConfig                      RPCConfig     `config:"rpc"`
	BoradcastConfig                NetworkConfig `config:"broadcast"`
	DiscoveryTimeoutInMilliseconds int64         `config:"discovery_timeout_ms"`
	HealthCheckInMilliseconds      int64         `config:"health_check_ms"`
}

func (cfg ClusterConfig) GetSeeds() []string {
	if (len(cfg.Seeds)) == 0 {
		return cfg.Seeds
	}
	newSeeds := []string{}
	for _, v := range cfg.Seeds {
		if v != cfg.RPCConfig.NetworkConfig.GetBindingAddr() {
			newSeeds = append(newSeeds, v)
		}
	}
	return newSeeds
}

type RPCConfig struct {
	TLSConfig     TLSConfig     `config:"tls"`
	NetworkConfig NetworkConfig `config:"network"`
}

// NetworkConfig stores network settings
type NetworkConfig struct {
	Host             string `config:"host"`
	Port             string `config:"port"`
	Binding          string `config:"binding"`
	SkipOccupiedPort bool   `config:"skip_occupied_port"`
	ReusePort        bool   `config:"reuse_port"`
}

func (cfg NetworkConfig) GetBindingPort() string {
	if cfg.Port != "" {
		return cfg.Port
	}
	if cfg.Binding != "" {
		array := strings.Split(cfg.Binding, ":")
		return array[1]
	}
	panic("error on get binding port")
}

func (cfg NetworkConfig) GetBindingAddr() string {
	if cfg.Binding != "" {
		array := strings.Split(cfg.Binding, ":")
		cfg.Host = array[0]
		cfg.Port = array[1]
		return cfg.Binding
	}
	if cfg.Host != "" && cfg.Port != "" {
		return fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	}
	panic(errors.Errorf("invalid network config, %v", cfg))
}

// NodeConfig stores node settings
type NodeConfig struct {
	Name string `config:"name"`
}

// PathConfig stores path settings
type PathConfig struct {
	Plugin string `config:"plugins"`
	Data   string `config:"data"`
	Log    string `config:"logs"`
	Cert   string `config:"certs"`
}

// SystemConfig is a high priority config, init from the environment or startup, can't be changed on the fly, need to restart to make config apply
type SystemConfig struct {
	ClusterConfig ClusterConfig `config:"cluster"`

	APIConfig APIConfig `config:"api"`

	NodeConfig NodeConfig `config:"node"`

	PathConfig PathConfig `config:"path"`

	CookieSecret string `config:"cookie_secret"`

	AllowMultiInstance bool `config:"allow_multi_instance"`

	MaxNumOfInstance int `config:"max_num_of_instances"`

	Modules []*Config `config:"modules"`

	Plugins []*Config `config:"plugins"`
}

type APIConfig struct {
	Enabled       bool          `config:"enabled"`
	TLSConfig     TLSConfig     `config:"tls"`
	NetworkConfig NetworkConfig `config:"network"`
}

func (config *APIConfig) GetSchema() string {
	if config.TLSConfig.TLSEnabled {
		return "https"
	}
	return "http"
}

type TLSConfig struct {
	TLSEnabled            bool   `config:"enabled"`
	TLSCertFile           string `config:"cert_file"`
	TLSKeyFile            string `config:"key_file"`
	TLSInsecureSkipVerify bool   `config:"skip_insecure_verify"`
}

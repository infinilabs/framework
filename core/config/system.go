package config

import (
	"fmt"
	"github.com/infinitbyte/framework/core/errors"
)

// ClusterConfig stores cluster settings
type ClusterConfig struct {
	Enabled         bool          `config:"enabled"`
	Name            string        `config:"name"`
	MinimumNodes    int           `config:"minimum_nodes"`
	Seeds           []string      `config:"seeds"`
	RPCConfig       RPCConfig     `config:"rpc"`
	BoradcastConfig NetworkConfig `config:"broadcast"`
}

type RPCConfig struct {
	TLSConfig     TLSConfig     `config:"tls"`
	NetworkConfig NetworkConfig `config:"network"`
}

// NetworkConfig stores network settings
type NetworkConfig struct {
	Host    string `config:"host"`
	Port    string `config:"port"`
	Binding string `config:"binding"`
}

func (cfg NetworkConfig) GetBindingAddr() string {
	if cfg.Binding != "" {
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
	Data string `config:"data"`
	Log  string `config:"logs"`
	Cert string `config:"certs"`
}

// SystemConfig is a high priority config, init from the environment or startup, can't be changed on the fly, need to restart to make config apply
type SystemConfig struct {
	ClusterConfig ClusterConfig `config:"cluster"`

	NodeConfig NodeConfig `config:"node"`

	PathConfig PathConfig `config:"path"`

	CookieSecret string `config:"cookie_secret"`

	AllowMultiInstance bool `config:"allow_multi_instance"`

	MaxNumOfInstance int `config:"max_num_of_instances"`

	Modules []*Config `config:"modules"`

	Plugins []*Config `config:"plugins"`
}

type TLSConfig struct {
	TLSEnabled            bool   `config:"enabled"`
	TLSCertFile           string `config:"cert_file"`
	TLSKeyFile            string `config:"key_file"`
	TLSInsecureSkipVerify bool   `config:"skip_insecure_verify"`
}

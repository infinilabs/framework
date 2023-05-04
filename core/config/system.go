package config

import (
	"fmt"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
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
	Host             string `config:"host" json:"host,omitempty" elastic_mapping:"host: { type: keyword }"`
	Port             int `config:"port" json:"port,omitempty" elastic_mapping:"port: { type: keyword }"`
	Binding          string `config:"binding" json:"binding,omitempty" elastic_mapping:"binding: { type: keyword }"`
	Publish          string `config:"publish" json:"publish,omitempty" elastic_mapping:"publish: { type: keyword }"`
	SkipOccupiedPort bool   `config:"skip_occupied_port" json:"skip_occupied_port,omitempty" elastic_mapping:"skip_occupied_port: { type: boolean }"`
	ReusePort        bool   `config:"reuse_port" json:"reuse_port,omitempty" elastic_mapping:"reuse_port: { type: boolean }"`
}

func (cfg NetworkConfig) GetPublishAddr() string {
	if cfg.Publish != "" {
		return util.GetSafetyInternalAddress(cfg.Publish)
	}
	return util.GetSafetyInternalAddress(cfg.GetBindingAddr())
}

func (cfg NetworkConfig) GetBindingPort() int {
	if cfg.Port >0 {
		return cfg.Port
	}
	if cfg.Binding != "" {
		array := strings.Split(strings.TrimSpace(cfg.Binding), ":")
		port,err:=util.ToInt(array[1])
		if err!=nil{
			panic(err)
		}
		cfg.Port = port
		return port
	}
	panic("error on get binding port")
}

func (cfg NetworkConfig) GetBindingAddr() string {
	if cfg.Binding != "" {

		//skip auto detect for ipv6 family
		if strings.Contains(cfg.Binding,"::"){
			return cfg.Binding
		}

		array := strings.Split(strings.TrimSpace(cfg.Binding), ":")
		cfg.Host = array[0]
		port,err:=util.ToInt(array[1])
		if err!=nil{
			panic(err)
		}
		cfg.Port = port
		return cfg.Binding
	}
	if cfg.Host != "" && cfg.Port >0 {
		return fmt.Sprintf("%s:%v", cfg.Host, cfg.Port)
	}
	panic(errors.Errorf("invalid network config, %v", cfg))
}

// NodeConfig stores node settings
type NodeConfig struct {
	ID   string `json:"id,omitempty" config:"id"`
	Name string `json:"name,omitempty" config:"name"`
	IP   string `json:"ip,omitempty" config:"ip"`
}

func (config *NodeConfig) ToString() string {
	return fmt.Sprintf("%s-%s", config.IP, config.Name)
}

// PathConfig stores path settings
type PathConfig struct {
	Plugin string `config:"plugins"`
	Config   string `config:"configs"`
	Data   string `config:"data"`
	Log    string `config:"logs"`
	Cert   string `config:"certs"`
}

// SystemConfig is a high priority config, init from the environment or startup, can't be changed on the fly, need to restart to make config apply
type SystemConfig struct {

	//reserved config
	ClusterConfig ClusterConfig `config:"cluster"`

	APIConfig APIConfig `config:"api"`

	NodeConfig NodeConfig `config:"node"`

	PathConfig PathConfig `config:"path"`

	LoggingConfig LoggingConfig `config:"log"`

	CookieSecret string `config:"cookie_secret"`

	AllowMultiInstance bool `config:"allow_multi_instance"`
	SkipInstanceDetect bool `config:"skip_instance_detect"`

	MaxNumOfInstance int `config:"max_num_of_instances"`

	MaxMemoryInBytes int `config:"max_memory_in_bytes"`

	Configs struct {
		AutoReload bool             `config:"auto_reload"`
	} `config:"configs"`

	//dynamic config enabled
	Modules []*Config `config:"modules"`

	Plugins []*Config `config:"plugins"`
}

type APIConfig struct {
	Enabled       bool          `config:"enabled"`
	AuthConfig    AuthConfig    `config:"auth"`
	TLSConfig     TLSConfig     `config:"tls"`
	NetworkConfig NetworkConfig `config:"network"`

	CrossDomain struct {
		AllowedOrigins []string `config:"allowed_origins"`
	} `config:"cors"`
	WebsocketConfig WebsocketConfig `config:"websocket"`
}

func (config *APIConfig) GetSchema() string {
	if config.TLSConfig.TLSEnabled {
		return "https"
	}
	return "http"
}

type TLSConfig struct {
	TLSEnabled            bool   `config:"enabled" json:"enabled,omitempty" elastic_mapping:"enabled: { type: boolean }"`
	TLSCertFile           string `config:"cert_file" json:"cert_file,omitempty" elastic_mapping:"cert_file: { type: keyword }"`
	TLSKeyFile            string `config:"key_file" json:"key_file,omitempty" elastic_mapping:"key_file: { type: keyword }"`
	TLSCAFile            string `config:"ca_file" json:"ca_file,omitempty" elastic_mapping:"ca_file: { type: keyword }"`
	TLSInsecureSkipVerify bool   `config:"skip_insecure_verify" json:"skip_insecure_verify,omitempty" elastic_mapping:"skip_insecure_verify: { type: boolean }"`

	//use for auto generate cert
	DefaultDomain string `config:"default_domain" json:"default_domain,omitempty" elastic_mapping:"default_domain: { type: keyword }"`
	SkipDomainVerify bool `config:"skip_domain_verify" json:"skip_domain_verify,omitempty" elastic_mapping:"skip_domain_verify: { type: boolean }"`

	ClientSessionCacheSize int `config:"client_session_cache_size" json:"client_session_cache_size,omitempty"`
}

type AuthConfig struct {
	Enabled           bool     `config:"enabled"`
	OAuthProvider     string   `config:"oauth_provider"`
	oauthAuthorizeUrl string   `config:"oauth_authorize_url"`
	oauthTokenUrl     string   `config:"oauth_token_url"`
	oauthRedirectUrl  string   `config:"oauth_redirect_url"`
	AuthorizedAdmins  []string `config:"authorized_admin"`
	ClientSecret      string   `config:"client_secret"`
	ClientID          string   `config:"client_id"`
}

type GzipConfig struct {
	Enabled bool `config:"enabled"`
	Level   int  `config:"level"`
}

type WebsocketConfig struct {
	Enabled bool `config:"enabled"`
	PermittedHosts   []string `config:"permitted_hosts"`
	SkipHostVerify bool     `config:"skip_host_verify"`
}
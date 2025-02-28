// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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
	Port             int    `config:"port" json:"port,omitempty" elastic_mapping:"port: { type: keyword }"`
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
	if cfg.Port > 0 {
		return cfg.Port
	}
	if cfg.Binding != "" {
		array := strings.Split(strings.TrimSpace(cfg.Binding), ":")
		port, err := util.ToInt(array[1])
		if err != nil {
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
		if strings.Contains(cfg.Binding, "::") {
			return cfg.Binding
		}

		array := strings.Split(strings.TrimSpace(cfg.Binding), ":")
		cfg.Host = array[0]
		port, err := util.ToInt(array[1])
		if err != nil {
			panic(err)
		}
		cfg.Port = port
		return cfg.Binding
	}
	if cfg.Host != "" && cfg.Port > 0 {
		return fmt.Sprintf("%s:%v", cfg.Host, cfg.Port)
	}
	panic(errors.Errorf("invalid network config, %v", cfg))
}

// NodeConfig stores node settings
type NodeConfig struct {
	ID   string `json:"id,omitempty" config:"id"`
	Name string `json:"name,omitempty" config:"name"`
	IP   string `json:"ip,omitempty" config:"ip"`

	//tagging for node
	MajorIpPattern string            `config:"major_ip_pattern"`
	Labels         map[string]string `config:"labels"`
	Tags           []string          `config:"tags"`
}

func (config *NodeConfig) ToString() string {
	return fmt.Sprintf("%s-%s", config.IP, config.Name)
}

// PathConfig stores path settings
type PathConfig struct {
	Plugin string `config:"plugins"`
	Config string `config:"configs"`
	Data   string `config:"data"`
	Log    string `config:"logs"`
}

// SystemConfig is a high priority config, init from the environment or startup, can't be changed on the fly, need to restart to make config apply
type SystemConfig struct {

	//reserved config
	ClusterConfig ClusterConfig `config:"cluster"`

	APIConfig APIConfig `config:"api"`

	WebAppConfig WebAppConfig `config:"web"`

	NodeConfig NodeConfig `config:"node"`

	PathConfig PathConfig `config:"path"`

	LoggingConfig LoggingConfig `config:"log"`

	Cookie CookieConfig `config:"cookie"`

	AllowMultiInstance bool `config:"allow_multi_instance"`
	SkipInstanceDetect bool `config:"skip_instance_detect"`

	MaxNumOfInstance int `config:"max_num_of_instances"`

	ResourceLimit *ResourceLimit `config:"resource_limit"`

	Configs ConfigsConfig `config:"configs"`

	//dynamic config enabled
	Modules []*Config `config:"modules"`

	Plugins []*Config `config:"plugins"`

	HTTPClientConfig map[string]HTTPClientConfig `config:"http_client"`
}

type CookieConfig struct {
	Secret string `config:"secret"`
	Domain string `config:"domain"`
}

type ProxyConfig struct {
	HTTPProxy                     string `config:"http_proxy"` //export HTTP_PROXY=http://username:password@proxy-url:port
	Socket5Proxy                  string `config:"socket5_proxy"`
	UsingEnvironmentProxySettings bool   `config:"using_proxy_env"` //using the the env(HTTP_PROXY, HTTPS_PROXY and NO_PROXY) configured HTTP proxy
}

func (c *HTTPClientConfig) init() {
	if !c.initTempMap {
		tempMap := map[string]bool{}
		for _, v := range c.Proxy.Denied {
			c.checkDomainMap[v] = false
		}

		for _, v := range c.Proxy.Permitted {
			c.checkDomainMap[v] = true
		}

		c.checkDomainMap = tempMap
		c.initTempMap = true
	}
}

func (c *HTTPClientConfig) ValidateProxy(addr string) (bool, *ProxyConfig) { //allow to visit the proxy, the proxy setting

	//init configs
	c.init()

	//check proxy rule for specify domain
	if len(c.Proxy.Domains) > 0 {
		//port are part of the domain, need exact match{
		if v, ok := c.Proxy.Domains[addr]; ok {
			return true, &v
		}
	}

	if len(c.checkDomainMap) > 0 {

		//any hit will be return
		if v, ok := c.checkDomainMap[addr]; ok {
			if v == false {
				return false, nil
			} else {
				return true, &c.Proxy.DefaultProxyConfig
			}
		}

		//only defined denied, the rest should permitted
		if len(c.Proxy.Denied) > 0 && len(c.Proxy.Permitted) == 0 {
			if v, ok := c.checkDomainMap[addr]; ok && v == false {
				return false, nil
			} else {
				return true, &c.Proxy.DefaultProxyConfig
			}
		}

		//only defined permitted, the rest should be consider denied
		if len(c.Proxy.Permitted) > 0 && len(c.Proxy.Denied) == 0 {
			if v, ok := c.checkDomainMap[addr]; ok && v == true {
				return true, &c.Proxy.DefaultProxyConfig
			} else {
				return false, nil
			}
		}
	}

	return true, &c.Proxy.DefaultProxyConfig
}

type HTTPClientConfig struct {
	Proxy struct {
		Enabled             bool                   `config:"enabled"`
		DefaultProxyConfig  ProxyConfig            `config:"default_config"`
		Permitted           []string               `config:"permitted"`
		Denied              []string               `config:"denied"`
		Domains             map[string]ProxyConfig `config:"domains"` //proxy settings per domain
		OverrideSystemProxy bool                   `config:"override_system_proxy_env"`
	} `config:"proxy"`

	Timeout              string    `config:"timeout"`
	DialTimeout          string    `config:"dial_timeout"`
	ReadTimeout          string    `config:"read_timeout"`
	WriteTimeout         string    `config:"write_timeout"`
	ReadBufferSize       int       `config:"read_buffer_size"`
	WriteBufferSize      int       `config:"write_buffer_size"`
	TLSConfig            TLSConfig `config:"tls"` //server or client's certs
	MaxConnectionPerHost int       `config:"max_connection_per_host"`

	//temp data structure
	initTempMap    bool
	checkDomainMap map[string]bool
}

type ConfigsConfig struct {
	AutoReload                 bool      `config:"auto_reload"`                    //auto reload local files
	Managed                    bool      `config:"managed"`                        //managed by remote config center
	ConfigFileManagedByDefault bool      `config:"config_file_managed_by_default"` //config file managed by default
	Servers                    []string  `config:"servers"`                        //remote config center servers
	ScheduledTask              bool      `config:"scheduled_task"`                 //use dedicated schedule task or background, use background task will save one goroutine
	Interval                   string    `config:"interval"`                       //sync interval in seconds
	SoftDelete                 bool      `config:"soft_delete"`                    //soft delete config
	PanicOnConfigError         bool      `config:"panic_on_config_error"`          //panic on config error
	MaxBackupFiles             int       `config:"max_backup_files"`               //keep max num of file backup
	ValidConfigsExtensions     []string  `config:"valid_config_extensions"`
	TLSConfig                  TLSConfig `config:"tls"` //server or client's certs
	ManagerConfig              struct {
		LocalConfigsRepoPath string `config:"local_configs_repo_path"`
	} `config:"manager"`
	AlwaysRegisterAfterRestart bool     `config:"always_register_after_restart"`
	AllowGeneratedMetricsTasks bool     `config:"allow_generated_metrics_tasks"`
	IgnoredPath                []string `config:"ignored_path"`
}

type ResourceLimit struct {
	CPU struct {
		CPUAffinityList    string `config:"affinity_list"`
		MaxCPUPercentUsage int    `config:"max_percent_usage"`
		MaxNumOfCPUs       int    `config:"max_num_of_cpus"`
	} `config:"cpu"`

	Mem struct {
		MaxMemoryInBytes int `config:"max_in_bytes"`
	} `config:"memory"`
}

type APISecurityConfig struct {
	Enabled  bool   `config:"enabled"`
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
}

type WebAppConfig struct {

	//same with API Config
	Enabled       bool              `config:"enabled"`
	TLSConfig     TLSConfig         `config:"tls"`
	NetworkConfig NetworkConfig     `config:"network"`
	Security      APISecurityConfig `config:"security"`
	CrossDomain   struct {
		AllowedOrigins []string `config:"allowed_origins"`
	} `config:"cors"`
	WebsocketConfig WebsocketConfig `config:"websocket"`
	//same with API Config

	AuthConfig   AuthConfig     `config:"auth"` //enable access control for UI or not
	UI           UIConfig       `config:"ui"`
	BasePath     string         `config:"base_path"`
	Domain       string         `config:"domain"`
	EmbeddingAPI bool           `config:"embedding_api"`
	Gzip         GzipConfig     `config:"gzip"`
	S3Config     S3BucketConfig `config:"s3"`
}

type S3Config struct {
	Endpoint           string `config:"endpoint" json:"endpoint,omitempty"`
	AccessKey          string `config:"access_key" json:"access_key,omitempty"`
	AccessSecret       string `config:"access_secret" json:"access_secret,omitempty"`
	Token              string `config:"token" json:"token,omitempty"`
	SSL                bool   `config:"ssl" json:"ssl,omitempty"`
	SkipInsecureVerify bool   `config:"skip_insecure_verify" json:"skip_insecure_verify,omitempty"`
}

type S3BucketConfig struct {
	Async    bool   `config:"async"`
	Server   string `config:"server"`
	Location string `config:"location"`
	Bucket   string `config:"bucket"`
}

func (config *WebAppConfig) GetEndpoint() string {
	return fmt.Sprintf("%s://%s", config.GetSchema(), config.NetworkConfig.GetPublishAddr())
}

func (config *WebAppConfig) GetSchema() string {
	if config.TLSConfig.TLSEnabled {
		return "https"
	}
	return "http"
}

type UIConfig struct {
	LocalPath    string `config:"path"`
	LocalEnabled bool   `config:"local"`
	VFSEnabled   bool   `config:"vfs"`
}

type APIConfig struct {
	Enabled       bool          `config:"enabled"`
	TLSConfig     TLSConfig     `config:"tls"`
	NetworkConfig NetworkConfig `config:"network"`

	Security APISecurityConfig `config:"security"`

	CrossDomain struct {
		AllowedOrigins []string `config:"allowed_origins"`
	} `config:"cors"`
	WebsocketConfig WebsocketConfig `config:"websocket"`

	VerboseErrorRootCause bool   `config:"verbose_error_root_cause"` //return root_cause in api response
	APIDirectoryPath      string `config:"api_directory_path"`
	DisableAPIDirectory   bool   `config:"disable_api_directory"`
}

func (config *APIConfig) GetEndpoint() string {
	return fmt.Sprintf("%s://%s", config.GetSchema(), config.NetworkConfig.GetPublishAddr())
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
	TLSCACertFile         string `config:"ca_file" json:"ca_file,omitempty" elastic_mapping:"ca_file: { type: keyword }"`
	TLSInsecureSkipVerify bool   `config:"skip_insecure_verify" json:"skip_insecure_verify,omitempty" elastic_mapping:"skip_insecure_verify: { type: boolean }"`

	//use for auto generate cert
	DefaultDomain    string `config:"default_domain" json:"default_domain,omitempty" elastic_mapping:"default_domain: { type: keyword }"`
	SkipDomainVerify bool   `config:"skip_domain_verify" json:"skip_domain_verify,omitempty" elastic_mapping:"skip_domain_verify: { type: boolean }"`

	//auto issue public TLS cert
	AutoIssue AutoIssue `config:"auto_issue" json:"auto_issue,omitempty" elastic_mapping:"auto_issue: { type: object }"`

	ClientSessionCacheSize int `config:"client_session_cache_size" json:"client_session_cache_size,omitempty"`
}

type AutoIssue struct {
	Enabled              bool     `config:"enabled" json:"enabled,omitempty" elastic_mapping:"enabled: { type: boolean }"`
	Email                string   `config:"email" json:"email,omitempty" elastic_mapping:"email: { type: keyword }"`
	Path                 string   `config:"path" json:"path,omitempty" elastic_mapping:"path: { type: keyword }"`
	IncludeDefaultDomain bool     `config:"include_default_domain" json:"include_default_domain,omitempty" elastic_mapping:"include_default_domain: { type: boolean }"`
	SkipInvalidDomain    bool     `config:"skip_invalid_domain" json:"skip_invalid_domain,omitempty" elastic_mapping:"skip_invalid_domain: { type: boolean }"`
	Domains              []string `config:"domains" json:"domains,omitempty" elastic_mapping:"domains: { type: keyword }"`

	Provider struct {
		TencentDNS struct {
			SecretID  string `config:"secret_id" json:"secret_id,omitempty" elastic_mapping:"secret_id: { type: keyword }"`
			SecretKey string `config:"secret_key" json:"secret_key,omitempty" elastic_mapping:"secret_key: { type: keyword }"`
		} `config:"tencent_dns" json:"tencent_dns,omitempty" elastic_mapping:"tencent_dns: { type: object }"`
	} `config:"provider" json:"provider,omitempty" elastic_mapping:"provider: { type: object }"`
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
	Enabled                     bool     `config:"enabled"`
	EchoWelcomeMessageOnConnect bool     `config:"echo_welcome_message_on_connect"`
	EchoLoggingConfigOnConnect  bool     `config:"echo_logging_config_on_connect"`
	BasePath                    string   `config:"base_path"`
	PermittedHosts              []string `config:"permitted_hosts"`
	SkipHostVerify              bool     `config:"skip_host_verify"`
}

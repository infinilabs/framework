/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package env

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/util"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Env is environment object of app
type Env struct {
	name          string
	uppercaseName string
	desc          string
	version       string
	commit        string
	buildDate     string

	terminalHeader string
	terminalFooter string

	// static configs
	SystemConfig *config.SystemConfig

	// dynamic configs
	RuntimeConfig *config.RuntimeConfig

	IsDebug      bool
	IsDaemonMode bool

	LoggingLevel string
}

// GetLastCommitLog returns last commit information of source code
func (env *Env) GetLastCommitLog() string {
	return env.commit
}

func (env *Env) GetLastCommitHash() string {
	log := env.GetLastCommitLog()
	array := strings.Split(log, ",")
	if len(array) == 0 {
		return "N/A"
	}
	return array[0]
}

// GetBuildDate returns the build datetime of current package
func (env *Env) GetBuildDate() string {
	return env.buildDate
}

// GetVersion returns the version of this build
func (env *Env) GetVersion() string {
	return env.version
}

func (env *Env) GetAppName() string {
	return env.name
}

func (env *Env) GetAppCapitalName() string {
	return env.uppercaseName
}
func (env *Env) GetAppDesc() string {
	return env.desc
}

func (env *Env) GetWelcomeMessage() string {
	s := env.terminalHeader

	commitLog := ""
	if env.GetLastCommitLog() != "" {
		commitLog = " " + env.GetLastCommitLog()
	}
	s += ("[" + env.GetAppCapitalName() + "] " + env.GetAppDesc() + "\n")
	s += env.GetVersion() + ", " + commitLog + "\n"
	return s
}

func (env *Env) GetGoodbyeMessage() string {
	s := env.terminalFooter

	if env.IsDaemonMode {
		return s
	}

	s += fmt.Sprintf("[%s] %s, uptime:%s\n", env.GetAppCapitalName(), env.GetVersion(), time.Since(GetStartTime()))
	return s
}

// Environment create a new env instance from a config
func (env *Env) Load(configFile string) *Env {
	sysConfig := loadSystemConfig(configFile)
	env.SystemConfig = &sysConfig

	var err error
	env.RuntimeConfig, err = env.loadRuntimeConfig()
	if err != nil {
		log.Error(err)
		panic(err)
	}

	if env.IsDebug {
		log.Debug(util.ToJson(env, true))
	}

	return env
}

var moduleConfig map[string]*config.Config
var pluginConfig map[string]*config.Config
var startTime = time.Now().UTC()

var (
	defaultSystemConfig = config.SystemConfig{
		ClusterConfig: config.ClusterConfig{
			Name: "APP",
		},
		NetworkConfig: config.NetworkConfig{
			Host: "127.0.0.1",
		},
		NodeConfig: config.NodeConfig{
			Name: util.PickRandomName(),
		},
		PathConfig: config.PathConfig{
			Data: "data",
			Log:  "log",
			Cert: "cert",
		},

		APIBinding:         "127.0.0.1:8001",
		HTTPBinding:        "127.0.0.1:9001",
		ClusterBinding:     "127.0.0.1:13001",
		AllowMultiInstance: true,
		MaxNumOfInstance:   5,
		TLSEnabled:         false,
	}
)

func loadSystemConfig(cfgFile string) config.SystemConfig {
	cfg := defaultSystemConfig
	cfg.ConfigFile = cfgFile
	if util.IsExist(cfgFile) {
		config, err := yaml.NewConfigWithFile(cfgFile, ucfg.PathSep("."))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = config.Unpack(&cfg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	os.MkdirAll(cfg.GetWorkingDir(), 0777)
	os.MkdirAll(cfg.PathConfig.Log, 0777)
	return cfg
}

var (
	defaultRuntimeConfig = config.RuntimeConfig{}
)

func (env *Env) loadRuntimeConfig() (*config.RuntimeConfig, error) {

	var configFile = "./app.yml"
	if env.SystemConfig != nil && len(env.SystemConfig.ConfigFile) > 0 {
		configFile = env.SystemConfig.ConfigFile
	}

	filename, _ := filepath.Abs(configFile)
	var cfg config.RuntimeConfig

	if util.FileExists(filename) {
		log.Debug("load configFile:", filename)
		cfg, err := config.LoadFile(filename)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		config := defaultRuntimeConfig

		if err := cfg.Unpack(&config); err != nil {
			log.Error(err)
			return nil, err
		}

		pluginConfig = parseModuleConfig(config.Plugins)
		moduleConfig = parseModuleConfig(config.Modules)

	} else {
		log.Debug("no config file was found")

		cfg = defaultRuntimeConfig
	}

	return &cfg, nil
}

func parseModuleConfig(cfgs []*config.Config) map[string]*config.Config {
	result := map[string]*config.Config{}

	for _, cfg := range cfgs {
		log.Trace(getModuleName(cfg), ",", cfg.Enabled(true))
		name := getModuleName(cfg)
		result[name] = cfg
	}

	return result
}

//GetModuleConfig return specify module's config
func GetModuleConfig(name string) *config.Config {
	cfg := moduleConfig[strings.ToLower(name)]
	return cfg
}

//GetPluginConfig return specify plugin's config
func GetPluginConfig(name string) *config.Config {
	cfg := pluginConfig[strings.ToLower(name)]
	return cfg
}

func getModuleName(c *config.Config) string {
	cfgObj := struct {
		Module string `config:"name"`
	}{}

	if c == nil {
		return ""
	}
	if err := c.Unpack(&cfgObj); err != nil {
		return ""
	}

	return cfgObj.Module
}

// EmptyEnv return a empty env instance
func NewEnv(name, desc, ver, commit, buildDate, terminalHeader, terminalFooter string) *Env {
	return &Env{
		name:           strings.TrimSpace(name),
		uppercaseName:  strings.ToUpper(strings.TrimSpace(name)),
		desc:           strings.TrimSpace(desc),
		version:        strings.TrimSpace(ver),
		commit:         strings.TrimSpace(commit),
		buildDate:      buildDate,
		terminalHeader: terminalHeader,
		terminalFooter: terminalFooter,
	}
}

func EmptyEnv() *Env {
	system := defaultSystemConfig
	return &Env{SystemConfig: &system, RuntimeConfig: &config.RuntimeConfig{}}
}

func GetStartTime() time.Time {
	return startTime
}

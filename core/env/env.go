/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package env

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

//TODO storage adaptor should config in env

// Env is environment object of app
type Env struct {
	name          string
	uppercaseName string
	lowercaseName string
	desc          string

	//generated
	version     string
	commit      string
	buildDate   string
	buildNumber string
	eolDate     string
	//generated

	configFile string

	terminalHeader string
	terminalFooter string

	// static configs
	SystemConfig *config.SystemConfig

	IsDebug       bool
	IsDaemonMode  bool
	ISServiceMode bool

	LoggingLevel string

	init bool

	workingDataDir string
	workingLogDir  string
	pluginDir      string

	allowSetup            bool
	setupRequired         bool
	IgnoreOnConfigMissing bool

	//atomic status
	state int32 //0 means running, 1 means stopping, 2 means stopped
}

func (env *Env) CheckSetup() error {
	if !env.allowSetup {
		return nil
	}

	env.setupRequired = false
	//check is required
	setupLock := path.Join(env.GetDataDir(), ".setup_lock")
	if !util.FileExists(setupLock) {
		env.setupRequired = true
	}
	return nil
}

func (env *Env) EnableSetup(b bool) {
	env.allowSetup = b
	if b {
		env.CheckSetup()
	}
}

func (env *Env) SetupRequired() bool {
	return env.setupRequired
}

func (env *Env) GetLastCommitHash() string {
	return util.TrimSpaces(env.commit)
}

// GetBuildDate returns the build datetime of current package
func (env *Env) GetBuildDate() time.Time {
	t, err := time.Parse(time.RFC3339, util.TrimSpaces(env.buildDate))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (env *Env) GetBuildNumber() string {
	return util.TrimSpaces(env.buildNumber)
}

// GetVersion returns the version of this build
func (env *Env) GetVersion() string {
	return util.TrimSpaces(env.version)
}

func (env *Env) GetEOLDate() time.Time {
	t, err := time.Parse(time.RFC3339, util.TrimSpaces(env.eolDate))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (env *Env) GetAppName() string {
	return env.name
}

func (env *Env) GetAppCapitalName() string {
	return env.uppercaseName
}

func (env *Env) GetAppLowercaseName() string {
	return env.lowercaseName
}

func (env *Env) GetAppDesc() string {
	return env.desc
}

func (env *Env) GetWelcomeMessage() string {
	s := env.terminalHeader

	message := ""
	message = fmt.Sprintf("%s, %s, %s", util.FormatTime(env.GetBuildDate()), util.FormatTime(env.GetEOLDate()), env.GetLastCommitHash())
	s += ("[" + env.GetAppCapitalName() + "] " + env.GetAppDesc() + "\n")
	s += "[" + env.GetAppCapitalName() + "] " + env.GetVersion() + "#" + env.GetBuildNumber() + ", " + message + ""
	return s
}

func (env *Env) GetGoodbyeMessage() string {
	s := fmt.Sprintf("[%s] %s, uptime: %s\n\n", env.GetAppCapitalName(), env.GetVersion(), time.Since(GetStartTime()))

	if env.IsDaemonMode {
		return s
	}

	s += env.terminalFooter
	return s
}

// Environment create a new env instance from a config
func (env *Env) Init() *Env {
	if env.init {
		return env
	}

	err := env.loadConfig()
	if err != nil {
		if env.SystemConfig.Configs.PanicOnConfigError {
			panic(err)
		}
	}

	//init config watcher
	if env.SystemConfig.Configs.AutoReload {

		if !env.ISServiceMode {
			log.Info("configuration auto reload enabled")
		}

		absConfigPath, _ := filepath.Abs(env.SystemConfig.PathConfig.Config)
		if util.FileExists(absConfigPath) {
			if !env.ISServiceMode {
				log.Info("watching config: ", absConfigPath)
			}
			config.EnableWatcher(env.SystemConfig.PathConfig.Config)
		}

		//watching self
		config.EnableWatcher(env.GetConfigFile())
	}

	if len(env.SystemConfig.Configs.ValidConfigsExtensions) > 0 {
		config.SetValidExtension(env.SystemConfig.Configs.ValidConfigsExtensions)
	}

	if env.IsDebug {
		log.Debug(util.ToJson(env, true))
	}

	env.init = true
	return env
}

func (env *Env) InitPaths(cfgPath string) error {
	defaultConfig:=GetDefaultSystemConfig()
	_, defaultConfig.NodeConfig.IP, _, _ = util.GetPublishNetworkDeviceInfo("")
	env.SystemConfig = &defaultConfig
	env.SystemConfig.ClusterConfig.Name = env.GetAppLowercaseName()
	env.SystemConfig.Configs.PanicOnConfigError = !env.IgnoreOnConfigMissing //if ignore on config missing, then no panic on config error

	var (
		cfgObj *config.Config
		err    error
	)

	if util.FileExists(cfgPath) {
		if cfgObj, err = config.LoadFile(cfgPath); err != nil {
			return fmt.Errorf("error loading confiuration file: %v, %w", cfgPath, err)
		}
		return cfgObj.Unpack(&env.SystemConfig)
	} else {
		if !env.IgnoreOnConfigMissing {
			return errors.Errorf("config file %v not found", cfgPath)
		}
	}
	return nil
}

var moduleConfig map[string]*config.Config
var pluginConfig map[string]*config.Config
var startTime = time.Now().UTC()

func GetDefaultSystemConfig() config.SystemConfig  {
	return config.SystemConfig{
		APIConfig: config.APIConfig{
			Enabled: true,
			NetworkConfig: config.NetworkConfig{
				Binding:          "0.0.0.0:2900",
				SkipOccupiedPort: true,
			},
			WebsocketConfig: config.WebsocketConfig{
				Enabled:        true,
				BasePath:       "/ws",
				SkipHostVerify: true,
			},
		},
		WebAppConfig: config.WebAppConfig{
			UI: config.UIConfig{
				LocalPath:    ".public",
				VFSEnabled:   true,
				LocalEnabled: true,
			}, Gzip: config.GzipConfig{
				Enabled: true,
				Level:   gzip.BestCompression,
			},
			AuthConfig: config.AuthConfig{
				Enabled: true,
			},
			WebsocketConfig: config.WebsocketConfig{
				Enabled:        true,
				SkipHostVerify: true,
			}},
		LoggingConfig: config.LoggingConfig{
			DisableFileOutput: false,
		},
		ClusterConfig: config.ClusterConfig{
			Seeds:                          []string{},
			HealthCheckInMilliseconds:      10000,
			DiscoveryTimeoutInMilliseconds: 10000,
			MinimumNodes:                   1,
			BoradcastConfig: config.NetworkConfig{
				Binding: "224.3.2.2:9876",
			},
			RPCConfig: config.RPCConfig{
				NetworkConfig: config.NetworkConfig{
					Binding:          "0.0.0.0:10000",
					SkipOccupiedPort: true,
				},
			},
		},

		NodeConfig: config.NodeConfig{},

		PathConfig: config.PathConfig{
			Plugin: "plugin",
			Data:   "data",
			Log:    "log",
			Config: "config",
		},

		AllowMultiInstance: false,
		MaxNumOfInstance:   5,
		Configs: config.ConfigsConfig{
			Interval:                   "30s",
			AutoReload:                 true,
			SoftDelete:                 true,
			ConfigFileManagedByDefault: true,
			PanicOnConfigError:         true,
			MaxBackupFiles:             10,
			ValidConfigsExtensions:     []string{".tpl", ".json", ".yml", ".yaml"},
		},
		HTTPClientConfig: config.HTTPClientConfig{
			ReadBufferSize:       100 * 1024,
			WriteBufferSize:      100 * 1024,
			ReadTimeout:          "60s",
			WriteTimeout:         "60s",
			MaxConnectionPerHost: 1000,
			TLSConfig:            config.TLSConfig{SkipDomainVerify: true, TLSInsecureSkipVerify: true},
		},
	}
}

func (env *Env) loadConfig() error {

	if env.configFile == "" {
		env.configFile = "./" + env.GetAppLowercaseName() + ".yml"
	}

	filename, _ := filepath.Abs(env.configFile)

	//looking config from pwd
	pwd, _ := os.Getwd()
	if pwd != "" {
		pwd = path.Join(pwd, env.GetAppLowercaseName()+".yml")
	}
	ex, err := os.Executable()
	var exPath string
	if err == nil {
		exPath = filepath.Dir(ex)
	}
	if exPath != "" {
		exPath = path.Join(exPath, env.GetAppLowercaseName()+".yml")
	}

	if util.FileExists(filename) {
		err := env.loadEnvFromConfigFile(filename)
		if err != nil {
			return err
		}
	} else if util.FileExists(pwd) {
		log.Warnf("default config missing, but found in %v", pwd)
		err := env.loadEnvFromConfigFile(pwd)
		if err != nil {
			return err
		}
	} else if util.FileExists(exPath) {
		log.Warnf("default config missing, but found in %v", exPath)
		err := env.loadEnvFromConfigFile(exPath)
		if err != nil {
			return err
		}
	} else {
		if !env.IgnoreOnConfigMissing {
			return errors.Errorf("config not found: %s", filename)
		}
	}

	return nil
}

func (env *Env) RefreshConfig() error {
	return env.loadEnvFromConfigFile(env.GetConfigFile())
}

var configObject *config.Config
var refreshLock = sync.Mutex{}

func (env *Env) loadEnvFromConfigFile(filename string) error {
	refreshLock.Lock()
	defer refreshLock.Unlock()

	log.Debug("loading config file:", filename)
	tempConfig, err := config.LoadFile(filename)
	if err != nil {
		panic(err)
		return err
	}

	tempCfg:=GetDefaultSystemConfig()
	if err := tempConfig.Unpack(&tempCfg); err != nil {
		panic(err)
		return err
	}

	env.SystemConfig=&tempCfg

	env.SetConfigFile(filename)

	log.Trace(env.SystemConfig.PathConfig.Config)

	//load configs from config folder
	if env.SystemConfig.PathConfig.Config != "" {
		cfgPath := util.TryGetFileAbsPath(env.SystemConfig.PathConfig.Config, true)
		if len(env.SystemConfig.Configs.IgnoredPath) > 0 {
			pathFilter := config.GenerateWildcardPathFilter(env.SystemConfig.Configs.IgnoredPath)
			config.RegisterPathFilter(pathFilter)
		}
		log.Debug("loading configs from: ", cfgPath)
		if util.FileExists(cfgPath) {

			v, err := config.LoadPath(env.SystemConfig.PathConfig.Config)
			if err != nil {
				if env.SystemConfig.Configs.PanicOnConfigError {
					panic(err)
				}
				return err
			}
			if env.IsDebug {
				obj := map[string]interface{}{}
				v.Unpack(&obj)
				log.Trace(util.ToJson(obj, true))
			}

			err = tempConfig.Merge(v)
			if err != nil {
				if env.SystemConfig.Configs.PanicOnConfigError {
					panic(err)
				}
				return err
			}
		}

		if env.IsDebug {
			obj := map[string]interface{}{}
			tempConfig.Unpack(&obj)
			log.Trace(util.ToJson(obj, true))
		}

	}

	obj1 := map[string]interface{}{}
	if err := tempConfig.Unpack(obj1); err != nil {
		if env.SystemConfig.Configs.PanicOnConfigError {
			panic(err)
		}
		return err
	}

	configObject = tempConfig

	pluginConfig = parseModuleConfig(env.SystemConfig.Plugins)
	moduleConfig = parseModuleConfig(env.SystemConfig.Modules)

	return nil
}

func (env *Env) GetConfigFile() string {
	return env.configFile
}

func (env *Env) SetConfigFile(configFile string) *Env {
	env.configFile = configFile
	return env
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

// GetModuleConfig return specify module's config
func GetModuleConfig(name string) *config.Config {
	cfg := moduleConfig[strings.ToLower(name)]
	return cfg
}

// GetPluginConfig return specify plugin's config
func GetPluginConfig(name string) *config.Config {
	cfg := pluginConfig[strings.ToLower(name)]
	return cfg
}

// LoadConfigContents reads contents from the given path, and renders template
// variables if necessary.
func LoadConfigContents(path string) (contents string, err error) {
	bytes, err := util.FileGetContent(path)
	if err != nil {
		return
	}
	contents = string(bytes)

	if !util.ContainStr(contents, "$[[") {
		return
	}

	variables, err := config.NewTemplateVariablesFromConfig(configObject)
	if err != nil {
		return
	}
	contents = config.NestedRenderingTemplate(contents, variables)

	return
}

func ParseConfig(configKey string, configInstance interface{}) (exist bool, err error) {
	return ParseConfigSection(configObject, configKey, configInstance)
}

func ParseConfigSection(cfg *config.Config, configKey string, configInstance interface{}) (exist bool, err error) {
	if cfg != nil {
		childConfig, err := cfg.Child(configKey, -1)
		if err != nil {
			return exist, err
		}

		log.Tracef("found config: %s ", configKey)

		exist = true

		err = childConfig.Unpack(configInstance)
		log.Tracef("parsed config: %s, %v", configKey, configInstance)
		if err != nil {
			return exist, err
		}

		return exist, nil
	} else {
		log.Debugf("config: %s not found", configKey)
	}
	return exist, errors.Errorf("invalid config: %s", configKey)
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
func NewEnv(name, desc, ver, buildNumber, commit, buildDate, eolDate, terminalHeader, terminalFooter string) *Env {
	return &Env{
		name:           util.TrimSpaces(name),
		uppercaseName:  strings.ToUpper(util.TrimSpaces(name)),
		lowercaseName:  strings.ToLower(util.TrimSpaces(name)),
		desc:           util.TrimSpaces(desc),
		version:        util.TrimSpaces(ver),
		commit:         util.TrimSpaces(commit),
		buildDate:      buildDate,
		buildNumber:    buildNumber,
		eolDate:        eolDate,
		terminalHeader: terminalHeader,
		terminalFooter: terminalFooter,
	}
}

func EmptyEnv() *Env {
	system := GetDefaultSystemConfig()
	system.ClusterConfig.Name = "app"
	system.PathConfig.Data = os.TempDir()
	system.PathConfig.Log = os.TempDir()
	system.LoggingConfig.DisableFileOutput = true
	system.LoggingConfig.LogLevel = "info"
	system.Configs.PanicOnConfigError = false
	return &Env{SystemConfig: &system}
}

func GetStartTime() time.Time {
	return startTime
}

func (env *Env) GetConfigDir() string {
	cfgPath := util.TryGetFileAbsPath(env.SystemConfig.PathConfig.Config, true)
	if util.FileExists(cfgPath) {
		return cfgPath
	}
	return env.SystemConfig.PathConfig.Config
}

// GetDataDir returns root working dir of app instance
func (env *Env) GetDataDir() string {
	if env.workingDataDir != "" {
		return env.workingDataDir
	}
	env.workingDataDir, env.workingLogDir = env.findWorkingDir()
	return env.workingDataDir
}

func (env *Env) GetLogDir() string {
	if env.workingLogDir != "" {
		return env.workingLogDir
	}
	env.workingDataDir, env.workingLogDir = env.findWorkingDir()
	return env.workingLogDir
}

func (env *Env) findWorkingDir() (string, string) {

	//check data folder
	//check if lock file exists
	//if no lock, use it
	//have lock，check if it is a dead instance
	//dead instance, use it
	//alive instance, skip to next folder
	//no folder exists, generate new id and name
	//persist metadata to folder

	baseDir := path.Join(env.SystemConfig.PathConfig.Data, env.SystemConfig.ClusterConfig.Name, "nodes")

	if !util.FileExists(baseDir) {
		if env.SystemConfig.NodeConfig.ID == "" {
			env.SystemConfig.NodeConfig.ID = util.GetUUID()
		}
		return env.getNodeWorkingDir(env.SystemConfig.NodeConfig.ID)
	}

	//try load instance id from existing data folder
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	if env.IsDebug {
		log.Trace("finding files in working dir:", files)
	}

	var instance = 0
	for _, f := range files {
		if env.IsDebug {
			log.Trace("checking dir: ", f.Name(), ",", f.IsDir())
		}
		if f.IsDir() {
			instance++
			lockFile := path.Join(baseDir, f.Name(), ".lock")
			if env.IsDebug {
				log.Tracef("lock found [%v] in dir: %v", util.FileExists(lockFile), f.Name())
			}
			if !util.FileExists(lockFile) {
				return env.getNodeWorkingDir(f.Name())
			}

			//check if pid is alive
			b, err := ioutil.ReadFile(lockFile)
			if err != nil {
				err := util.FileDelete(lockFile)
				panic(errors.Errorf("invalid lock file: %v, deleting now", err))
			}
			pid, err := util.ToInt(string(b))
			if err != nil {
				err := util.FileDelete(lockFile)
				panic(errors.Errorf("invalid lock file: %v, deleting now", err))
			}
			if pid <= 0 {
				err := util.FileDelete(lockFile)
				panic(errors.Errorf("invalid lock file: %v, deleting now", err))
			}

			procExists := util.CheckProcessExists(pid)
			if env.IsDebug {
				log.Tracef("process [%v] exists: ", pid, procExists)
			}
			if !procExists {

				err := util.FileDelete(lockFile)
				if err != nil {
					panic(err)
				}
				if env.IsDebug {
					log.Debug("dead process with broken lock file, removed: ", lockFile)
				}
				return env.getNodeWorkingDir(f.Name())
			}
			if env.IsDebug {
				log.Tracef("data folder [%v] is in used by [%v], continue", f.Name(), pid)
			}
			//current folder is in use
			if !env.SystemConfig.AllowMultiInstance {
				env.SystemConfig.NodeConfig.ID = f.Name()
				break
			}

			if instance >= env.SystemConfig.MaxNumOfInstance {
				panic(fmt.Errorf("reach max num of instances on this node, max_num_of_instances is: %v", env.SystemConfig.MaxNumOfInstance))
			}
		}
	}

	//final check
	if env.SystemConfig.NodeConfig.ID == "" {
		env.SystemConfig.NodeConfig.ID = util.GetUUID()
	}

	return env.getNodeWorkingDir(env.SystemConfig.NodeConfig.ID)
}

func (env *Env) GetPluginDir() string {
	if env.pluginDir != "" {
		return env.pluginDir
	}

	if env.SystemConfig.PathConfig.Plugin == "" {
		env.pluginDir = "./plugins"
	} else {
		env.pluginDir = env.SystemConfig.PathConfig.Plugin
	}

	return env.pluginDir
}

// lowercase, get configs from defaults>env>config
func (env *Env) GetConfig(key string, defaultV string) (string, bool) {
tryEnvAgain:
	val, ok := os.LookupEnv(key)
	if !ok {

		val, ok = os.LookupEnv(strings.ToUpper(key))
		if ok {
			return val, true
		}

		val, ok = os.LookupEnv(strings.ToLower(key))
		if ok {
			return val, true
		}
		if strings.Contains(key, ".") {
			key = strings.ReplaceAll(key, ".", "_")
			goto tryEnvAgain
		}

		return defaultV, false
	} else {
		return val, true
	}

	//TODO check configs
	//TODO cache env for period time
}

func GetConfigAsJSON() string {
	o := util.MapStr{}
	configObject.Unpack(&o)
	return util.MustToJSON(o)
}

func (env *Env) getNodeWorkingDir(nodeID string) (string, string) {
	env.SystemConfig.NodeConfig.ID = nodeID

	dataDir := path.Join(env.SystemConfig.PathConfig.Data, env.SystemConfig.ClusterConfig.Name, "nodes", nodeID)
	logDir := path.Join(env.SystemConfig.PathConfig.Log, env.SystemConfig.ClusterConfig.Name, "nodes", nodeID)

	//try get node name from meta file
	metaFile := path.Join(dataDir, ".meta")
	if util.FileExists(metaFile) {
		data, err := util.FileGetContent(metaFile)
		if err != nil {
			panic(err)
		}
		str := string(data)
		arr := strings.Split(str, ",")
		if len(arr) == 2 {
			env.SystemConfig.NodeConfig.Name = arr[1]
		}
	}

	//meta was not exists or just in case meta file was broken
	if env.SystemConfig.NodeConfig.Name == "" {
		env.SystemConfig.NodeConfig.Name = util.PickRandomName()
	}

	if !util.FileExists(metaFile) {

		//persist meta, in case data was not found
		if !util.FileExists(dataDir) {
			err := os.MkdirAll(dataDir, 0755)
			if err != nil {
				panic(err)
			}
		}

		_, err := util.FilePutContent(metaFile, fmt.Sprintf("%v,%v", env.SystemConfig.NodeConfig.ID, env.SystemConfig.NodeConfig.Name))
		if err != nil {
			panic(err)
		}
	}

	return dataDir, logDir
}

func (env *Env) UpdateState(i int32) {
	atomic.StoreInt32(&env.state, i)
}

func (env *Env) GetState() int32 {
	return atomic.LoadInt32(&env.state)
}

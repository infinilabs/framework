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
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
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

	IsDebug      bool
	IsDaemonMode bool

	LoggingLevel string

	init bool

	workingDataDir string
	workingLogDir  string
	pluginDir      string
	SetupRequired  bool
}

func (env *Env) GetLastCommitHash() string {
	return util.TrimSpaces(env.commit)
}

// GetBuildDate returns the build datetime of current package
func (env *Env) GetBuildDate() time.Time {
	t,err:=time.Parse(time.RFC3339,util.TrimSpaces(env.buildDate))
	if err!=nil{
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
	t,err:=time.Parse(time.RFC3339,util.TrimSpaces(env.eolDate))
	if err!=nil{
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
	//if env.GetLastCommitLog() != "" {
	//	message = " " + env.GetLastCommitLog()
	//}

	message =fmt.Sprintf("%s, %s, %s",util.FormatTime(env.GetBuildDate()),util.FormatTime(env.GetEOLDate()),env.GetLastCommitHash())

	s += ("[" + env.GetAppCapitalName() + "] " + env.GetAppDesc() + "\n")
	s +=  "[" + env.GetAppCapitalName() + "] " + env.GetVersion() +"#"+ env.GetBuildNumber() + ", " + message + ""
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
		panic(err)
	}

	if env.IsDebug {
		log.Debug(util.ToJson(env, true))
	}

	env.init = true
	return env
}

var moduleConfig map[string]*config.Config
var pluginConfig map[string]*config.Config
var startTime = time.Now().UTC()

var (
	defaultSystemConfig = config.SystemConfig{
		APIConfig: config.APIConfig{
				Enabled: true,
				NetworkConfig: config.NetworkConfig{
					Binding:          "0.0.0.0:2900",
					SkipOccupiedPort: true,
				},
		},
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

		NodeConfig: config.NodeConfig{

		},

		PathConfig: config.PathConfig{
			Plugin: "plugins",
			Data:   "data",
			Log:    "log",
			Config:  "configs",
		},

		AllowMultiInstance: false,
		MaxNumOfInstance:   5,
	}
)

var configObject *config.Config

func (env *Env) loadConfig() error {

	var ignoreFileMissing =false
	if env.configFile == "" {
		env.configFile = "./" + env.GetAppLowercaseName() + ".yml"
		ignoreFileMissing = true
	}

	_,defaultSystemConfig.NodeConfig.IP,_,_ = util.GetPublishNetworkDeviceInfo("")

	env.SystemConfig = &defaultSystemConfig

	if env.SystemConfig.ClusterConfig.Name == "" {
		env.SystemConfig.ClusterConfig.Name = env.GetAppLowercaseName()
	}

	filename, _ := filepath.Abs(env.configFile)

	//looking config from pwd
	pwd,_:=os.Getwd()
	if pwd!=""{
		pwd=path.Join(pwd,env.GetAppLowercaseName() + ".yml")
	}
	ex,err:=os.Executable()
	var exPath string
	if err==nil{
		exPath=filepath.Dir(ex)
	}
	if exPath!=""{
		exPath=path.Join(exPath,env.GetAppLowercaseName() + ".yml")
	}

	log.Trace("pwd:",pwd,",process path:",exPath)

	if util.FileExists(filename) {
		err:=env.loadEnvFromConfigFile(filename)
		if err!=nil{
			return err
		}
	}else if util.FileExists(pwd){
		log.Warnf("default config missing, but found in %v",pwd)
		err:=env.loadEnvFromConfigFile(pwd)
		if err!=nil{
			return err
		}
	}else if util.FileExists(exPath){
		log.Warnf("default config missing, but found in %v",exPath)
		err:=env.loadEnvFromConfigFile(exPath)
		if err!=nil{
			return err
		}
	} else {
		if !ignoreFileMissing {
			return errors.Errorf("config not found: %s", filename)
		}
	}

	return nil
}

func (env *Env) loadEnvFromConfigFile(filename string) error {
	log.Debug("loading config file:", filename)
	var err error
	configObject, err = config.LoadFile(filename)
	if err != nil {
		return err
	}

	if err := configObject.Unpack(env.SystemConfig); err != nil {
		return err
	}

	env.SetConfigFile(filename)

	log.Trace(env.SystemConfig.PathConfig.Config)

	//load configs from config folder
	if env.SystemConfig.PathConfig.Config != "" {
		cfgPath := util.TryGetFileAbsPath(env.SystemConfig.PathConfig.Config, true)
		log.Debug("loading configs from:", cfgPath)
		if util.FileExists(cfgPath) {

			v, err := config.LoadPath(env.SystemConfig.PathConfig.Config)
			if err != nil {
				return err
			}
			if env.IsDebug {
				obj := map[string]interface{}{}
				v.Unpack(&obj)
				log.Trace(util.ToJson(obj, true))
			}

			err = configObject.Merge(v)
			if err != nil {
				return err
			}
		}

		if env.IsDebug {
			obj := map[string]interface{}{}
			configObject.Unpack(&obj)
			log.Trace(util.ToJson(obj, true))
		}

		if env.SystemConfig.Configs.AutoReload {
			log.Info("auto reload config, monitoring path:", env.SystemConfig.PathConfig.Config)
			config.EnableWatcher(env.SystemConfig.PathConfig.Config)
		}
	}

	obj := map[string]interface{}{}
	if err := configObject.Unpack(obj); err != nil {
		return err
	}
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

func ParseConfig(configKey string, configInstance interface{}) (exist bool, err error) {
	return ParseConfigSection(configObject,configKey,configInstance)
}

func ParseConfigSection(cfg *config.Config,configKey string, configInstance interface{}) (exist bool, err error) {
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
func NewEnv(name, desc, ver,buildNumber, commit, buildDate,eolDate, terminalHeader, terminalFooter string) *Env {
	return &Env{
		name:           util.TrimSpaces(name),
		uppercaseName:  strings.ToUpper(util.TrimSpaces(name)),
		lowercaseName:  strings.ToLower(util.TrimSpaces(name)),
		desc:           util.TrimSpaces(desc),
		version:        util.TrimSpaces(ver),
		commit:         util.TrimSpaces(commit),
		buildDate:      buildDate,
		buildNumber:    buildNumber,
		eolDate:      	eolDate,
		terminalHeader: terminalHeader,
		terminalFooter: terminalFooter,
	}
}

func EmptyEnv() *Env {
	system := defaultSystemConfig
	system.ClusterConfig.Name = "app"
	system.PathConfig.Data=os.TempDir()
	system.PathConfig.Log=os.TempDir()
	system.LoggingConfig.DisableFileOutput=true
	return &Env{SystemConfig: &system}
}

func GetStartTime() time.Time {
	return startTime
}

// GetDataDir returns root working dir of app instance
func (env *Env) GetDataDir() string {
	if env.workingDataDir!=""{
		return env.workingDataDir
	}
	env.workingDataDir,env.workingLogDir=env.findWorkingDir()
	return env.workingDataDir
}

func (env *Env) GetLogDir() string {
	if env.workingLogDir!=""{
		return env.workingLogDir
	}
	env.workingDataDir,env.workingLogDir=env.findWorkingDir()
	return env.workingLogDir
}

func (env *Env) findWorkingDir() (string,string) {

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
			env.SystemConfig.NodeConfig.Name = util.PickRandomName()
		}
		return env.getNodeWorkingDir(env.SystemConfig.NodeConfig.ID)
	}

	//try load instance id from existing data folder
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	log.Trace("finding files in working dir:",files)

	var instance = 0
	for _, f := range files {
		log.Trace("checking dir: ",f.Name(),",",f.IsDir())
		if f.IsDir() {
			instance++
			lockFile := path.Join(baseDir,f.Name(), ".lock")

			log.Tracef("lock found [%v] in dir: %v",util.FileExists(lockFile),f.Name())

			if !util.FileExists(lockFile) {
				env.SystemConfig.NodeConfig.ID = f.Name()
				env.SystemConfig.NodeConfig.Name = util.PickRandomName()
				return env.getNodeWorkingDir(env.SystemConfig.NodeConfig.ID)
			}

			//check if pid is alive
			b, err := ioutil.ReadFile(lockFile)
			if err != nil {
				panic(err)
			}
			pid, err := util.ToInt(string(b))
			if err != nil {
				panic(err)
			}
			if pid <= 0 {
				panic(errors.New("invalid pid"))
			}

			procExists := util.CheckProcessExists(pid)

			log.Tracef("process [%v] exists: ",pid, procExists)

			if !procExists {

				err := util.FileDelete(lockFile)
				if err != nil {
					panic(err)
				}
				log.Debug("dead process with broken lock file, removed: ", lockFile)
				env.SystemConfig.NodeConfig.ID = f.Name()
				env.SystemConfig.NodeConfig.Name = util.PickRandomName()
				return env.getNodeWorkingDir(f.Name())
			}

			log.Tracef("data folder [%v] is in used by [%v], continue",f.Name(),pid)

			//current folder is in use
			if !env.SystemConfig.AllowMultiInstance {
				env.SystemConfig.NodeConfig.ID = f.Name()
				env.SystemConfig.NodeConfig.Name = util.PickRandomName()
				break
			}

			if instance>=env.SystemConfig.MaxNumOfInstance{
				panic(fmt.Errorf("reach max num of instances on this node, limit is: %v", env.SystemConfig.MaxNumOfInstance))
			}
		}
	}

	//final check
	if env.SystemConfig.NodeConfig.ID == "" {
		env.SystemConfig.NodeConfig.ID = util.GetUUID()
		env.SystemConfig.NodeConfig.Name = util.PickRandomName()
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

//lowercase, get configs from defaults>env>config
func (env *Env)GetConfig(key string,defaultV string)(string,bool){
	tryEnvAgain:
	val, ok := os.LookupEnv(key)
	if !ok {

		val, ok = os.LookupEnv(strings.ToUpper(key))
		if ok{
			return val,true
		}

		val, ok = os.LookupEnv(strings.ToLower(key))
		if ok{
			return val,true
		}
		if strings.Contains(key,"."){
			key=strings.ReplaceAll(key,".","_")
			goto tryEnvAgain
		}

		return defaultV,false
	} else {
		return val,true
	}

	//TODO check configs
	//TODO cache env for period time
}

func (env *Env) getNodeWorkingDir(nodeID string)(string,string)  {
	dir1:= path.Join(env.SystemConfig.PathConfig.Data, env.SystemConfig.ClusterConfig.Name, "nodes", nodeID)
	dir2:= path.Join(env.SystemConfig.PathConfig.Log,  env.SystemConfig.ClusterConfig.Name, "nodes", nodeID)
	return dir1,dir2
}

// Package config , actually copied from github.com/elastic/beats
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"infini.sh/framework/lib/go-ucfg/parse"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/file"
	"infini.sh/framework/lib/fasttemplate"
	"infini.sh/framework/lib/go-ucfg"
	cfgflag "infini.sh/framework/lib/go-ucfg/flag"
	"infini.sh/framework/lib/go-ucfg/yaml"
)

// Config object to store hierarchical configurations into.
// See https://godoc.org/github.com/elastic/go-ucfg#Config
type Config ucfg.Config

// Namespace storing at most one configuration section by name and sub-section.
type Namespace struct {
	C map[string]*Config `config:",inline"`
}

type flagOverwrite struct {
	config *ucfg.Config
	path   string
	value  string
}

var configOpts = []ucfg.Option{
	ucfg.PathSep("."),
	ucfg.AppendValues,
	ucfg.VarExp,
	ucfg.ResolveNOOP,
	ucfg.DefaultParseConfig(parse.NoopConfig),
}

var customConfigOpts = map[string]ucfg.Option{}

func RegisterOption(name string, option ucfg.Option) {
	customConfigOpts[name] = option
}

// NewConfig create a pretty new config
func NewConfig() *Config {
	return fromConfig(ucfg.New())
}

// NewConfigFrom get config instance
func NewConfigFrom(from interface{}) (*Config, error) {
	c, err := ucfg.NewFrom(from, configOpts...)
	return fromConfig(c), err
}

// MergeConfigs just merge configs together
func MergeConfigs(cfgs ...*Config) (*Config, error) {
	config := NewConfig()
	for _, c := range cfgs {
		if err := config.Merge(c); err != nil {
			return nil, err
		}
	}
	return config, nil
}

// NewConfigWithYAML load config from yaml
func NewConfigWithYAML(in []byte, source string) (*Config, error) {
	opts := append(
		[]ucfg.Option{
			ucfg.MetaData(ucfg.Meta{Source: source}),
		},
		configOpts...,
	)
	c, err := yaml.NewConfig(in, opts...)
	return fromConfig(c), err
}

// NewFlagConfig will use flags
func NewFlagConfig(
	set *flag.FlagSet,
	def *Config,
	name string,
	usage string,
) *Config {
	opts := append(
		[]ucfg.Option{
			ucfg.MetaData(ucfg.Meta{Source: "command line flag"}),
		},
		configOpts...,
	)

	var to *ucfg.Config
	if def != nil {
		to = def.access()
	}

	config := cfgflag.ConfigVar(set, to, name, usage, opts...)
	return fromConfig(config)
}

// NewFlagOverwrite will use flags which specified
func NewFlagOverwrite(
	set *flag.FlagSet,
	config *Config,
	name, path, def, usage string,
) *string {
	if config == nil {
		panic("Missing configuration")
	}
	if path == "" {
		panic("empty path")
	}

	if def != "" {
		err := config.SetString(path, -1, def)
		if err != nil {
			panic(err)
		}
	}

	f := &flagOverwrite{
		config: config.access(),
		path:   path,
		value:  def,
	}

	if set == nil {
		flag.Var(f, name, usage)
	} else {
		set.Var(f, name, usage)
	}

	return &f.value
}

func LoadPath(folder string) (*ucfg.Config, error) {
	files := []string{}
	filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			if util.SuffixStr(path, ".yml") || util.SuffixStr(path, ".yaml") {
				files = append(files, path)
			}
		}
		return nil
	})
	return LoadFiles(files...)
}

type TemplateConfigs struct {
	Templates []ConfigTemplate `config:"configs.template"`
}

type ConfigTemplate struct {
	Name     string      `config:"name"`
	Path     string      `config:"path"`
	Variable util.MapStr `config:"variable"`
}

type EnvConfig struct {
	Environments map[string]interface{} `config:"env"`
}

func LoadFile(path string) (*Config, error) {
	var err error
	//check templated file
	cfgByes, err := util.FileGetContent(path)
	if err != nil {
		panic(err)
	}

	//if hash variable, apply and re-unpack
	bytesStr := util.UnsafeBytesToString(cfgByes)
	if util.ContainStr(bytesStr, "$[[") {
		obj, err := LoadEnvVariables(path)
		if err != nil {
			panic(err)
		}

		envObj := util.MapStr{}
		envObj.Put("env", obj)
		tempConfig := ConfigTemplate{
			Path:     path,
			Variable: envObj,
		}
		return NewConfigWithTemplate(tempConfig)
	}
	return internalLoadFile(path)
}

func LoadEnvVariables(path string) (map[string]interface{}, error) {
	env1 := EnvConfig{}
	var err error
	configObject, err := internalLoadFile(path)
	if err != nil {
		return nil, err
	}

	if err := configObject.Unpack(&env1); err != nil {
		return nil, err
	}

	log.Debugf("config contain variables, try to parse with environments")
	environs := os.Environ()
	obj := map[string]interface{}{}

	for k, v := range env1.Environments {
		obj[k] = v
	}

	for _, env := range environs {
		kv := strings.Split(env, "=")
		if len(kv) == 2 {
			obj[kv[0]] = kv[1]
		}
	}

	log.Trace("environments:", util.ToJson(obj, true))
	return obj, nil
}

// internalLoadFile will load config from specify file
func internalLoadFile(path string) (*Config, error) {

	c, err := yaml.NewConfigWithFile(path, configOpts...)
	if err != nil {
		return nil, err
	}

	pCfg := fromConfig(c)

	if pCfg.HasField("configs") {
		templates := TemplateConfigs{}
		pCfg.Unpack(&templates)
		log.Trace(templates)
		if len(templates.Templates) > 0 {
			for i, v := range templates.Templates {
				log.Tracef("processing #[%v] template: %v,%v", i, v.Name, v.Path)
				cfg, err := NewConfigWithTemplate(v)
				if err != nil {
					return pCfg, err
				} else {
					pCfg, err = MergeConfigs(pCfg, cfg)
					if err != nil {
						return pCfg, err
					}
					obj := map[string]interface{}{}
					if err := pCfg.Unpack(obj); err != nil {
						return pCfg, err
					}
				}
			}
		}

	}

	log.Debugf("load config file '%v'", path)
	return pCfg, err
}


func NestedRenderingTemplate(temp string, runKv util.MapStr) string {
	template, err := fasttemplate.NewTemplate(temp, "$[[", "]]")
	if err != nil {
		panic(err)
	}

	configStr := template.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		variable, ok := GetVariable(runKv, tag)
		if ok {
			return w.Write([]byte(variable))
		}
		return w.Write([]byte("$[[" + tag + "]]"))
	})

	if configStr!=temp&& strings.Contains(configStr, "$[[") && strings.Contains(configStr, "]]") &&strings.Index(configStr, "$[[") < strings.LastIndex(configStr, "]]") {
		newConfigStr:= NestedRenderingTemplate(configStr, runKv)
		if newConfigStr != configStr {
			configStr = newConfigStr
		}
		//fmt.Println("hit nested, and finished")
	}else{
		//fmt.Println("not hit nested template:",configStr!=temp,",",runKv,",",strings.Index(configStr, "$[[") < strings.LastIndex(configStr, "]]"))
	}
	return configStr
}

func GetVariable(runtimeKV util.MapStr, key string) (string, bool) {
	if runtimeKV != nil {

		if util.ContainStr(key,"$[[") {

			template, err := fasttemplate.NewTemplate(string(key), "$[[", "]]")
			if err != nil {
				//log.Error(key)
				panic(err)
			}

			key= template.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
				variable, ok := GetVariable(runtimeKV, tag)
				if ok {
					return w.Write([]byte(variable))
				}
				return w.Write([]byte("$[[" + tag + "]]"))
			})
		}

		x, err := runtimeKV.GetValue(key)
		if err == nil {
			str, ok := x.(string)
			if ok {
				return str, true
			} else {
				if util.TypeIsArray(x){
					y:=x.([]interface{})
					o:="['"+util.JoinInterfaceArray(y,"','",nil)+"']"
					return o, true
				}else{
					return util.ToString(x), true
				}
			}
		}
	}
	return "", false
}

func NewConfigWithTemplate(v ConfigTemplate) (*Config, error) {

	cfgFile := util.TryGetFileAbsPath(v.Path, false)
	if !util.FileExists(cfgFile) {
		return nil, errors.Errorf("template %v not exists", cfgFile)
	}

	tempBytes, err := util.FileGetContent(cfgFile)
	if err != nil {
		return nil, err
	}

	configStr:=NestedRenderingTemplate(string(tempBytes), v.Variable)

	//log.Error(configStr)

	log.Trace("rendering templated config:\n", configStr)
	return NewConfigWithYAML([]byte(configStr), "template")
}

// LoadFiles will load configs from specify files
func LoadFiles(paths ...string) (*ucfg.Config, error) {

	c := ucfg.New()
	opts := []ucfg.Option{
		ucfg.AppendValues,
	}

	var err error
	cfg := &Config{}

	for _, path := range paths {
		cfg, err = internalLoadFile(path)
		if err != nil {
			return c, err
		}

		err = c.Merge(cfg, opts...)
		if err != nil {
			return c, err
		}
	}

	return c, err
}

// Merge a map, a slice, a struct or another Config object into c.
func (c *Config) Merge(from interface{}) error {
	return c.access().Merge(from, configOpts...)
}

// Unpack unpacks c into a struct, a map, or a slice allocating maps, slices,
// and pointers as necessary.
func (c *Config) Unpack(to interface{}) error {
	var opts []ucfg.Option
	opts = append(opts, configOpts...)
	for _, opt := range customConfigOpts {
		opts = append(opts, opt)
	}
	return c.access().Unpack(to, opts...)
}

// Path gets the absolute path of c separated by sep. If c is a root-Config an
// empty string will be returned.
func (c *Config) Path() string {
	return c.access().Path(".")
}

// PathOf gets the absolute path of a potential setting field in c with name
// separated by sep.
func (c *Config) PathOf(field string) string {
	return c.access().PathOf(field, ".")
}

// HasField checks if c has a top-level named key name.
func (c *Config) HasField(name string) bool {
	return c.access().HasField(name)
}

// CountField returns number of entries in a table or 1 if entry is a primitive value.
// Primitives settings can be handled like a list with 1 entry.
func (c *Config) CountField(name string) (int, error) {
	return c.access().CountField(name)
}

// Bool reads a boolean setting returning an error if the setting has no
// boolean value.
func (c *Config) Bool(name string, idx int) (bool, error) {
	return c.access().Bool(name, idx, configOpts...)
}

// Strings reads a string setting returning an error if the setting has
// no string or primitive value convertible to string.
func (c *Config) String(name string, idx int) (string, error) {
	return c.access().String(name, idx, configOpts...)
}

// Int reads an int64 value returning an error if the setting is
// not integer value, the primitive value is not convertible to int or a conversion
// would create an integer overflow.
func (c *Config) Int(name string, idx int) (int64, error) {
	return c.access().Int(name, idx, configOpts...)
}

// Float reads a float64 value returning an error if the setting is
// not a float value or the primitive value is not convertible to float.
func (c *Config) Float(name string, idx int) (float64, error) {
	return c.access().Float(name, idx, configOpts...)
}

// Child returns a child configuration or an error if the setting requested is a
// primitive value only.
func (c *Config) Child(name string, idx int) (*Config, error) {
	sub, err := c.access().Child(name, idx, configOpts...)
	return fromConfig(sub), err
}

// SetBool sets a boolean primitive value. An error is returned if the new name
// is invalid.
func (c *Config) SetBool(name string, idx int, value bool) error {
	return c.access().SetBool(name, idx, value, configOpts...)
}

// SetInt sets an integer primitive value. An error is returned if the new
// name is invalid.
func (c *Config) SetInt(name string, idx int, value int64) error {
	return c.access().SetInt(name, idx, value, configOpts...)
}

// SetFloat sets an floating point primitive value. An error is returned if
// the name is invalid.
func (c *Config) SetFloat(name string, idx int, value float64) error {
	return c.access().SetFloat(name, idx, value, configOpts...)
}

// SetString sets string value. An error is returned if the name is invalid.
func (c *Config) SetString(name string, idx int, value string) error {
	return c.access().SetString(name, idx, value, configOpts...)
}

// SetChild adds a sub-configuration. An error is returned if the name is invalid.
func (c *Config) SetChild(name string, idx int, value *Config) error {
	return c.access().SetChild(name, idx, value.access(), configOpts...)
}

// IsDict checks if c has named keys.
func (c *Config) IsDict() bool {
	return c.access().IsDict()
}

// IsArray checks if c has index only accessible settings.
func (c *Config) IsArray() bool {
	return c.access().IsArray()
}

// Enabled was a predefined config, enabled by default if no config was found
func (c *Config) Enabled(defaultV bool) bool {
	testEnabled := struct {
		Enabled bool `config:"enabled"`
	}{defaultV}

	if c == nil {
		return defaultV
	}
	if err := c.Unpack(&testEnabled); err != nil {
		// if unpacking fails, expect 'enabled' being set to default value
		return defaultV
	}
	return testEnabled.Enabled
}

func FromConfig(in *ucfg.Config) *Config {
	return fromConfig(in)
}

func fromConfig(in *ucfg.Config) *Config {
	return (*Config)(in)
}

func (c *Config) access() *ucfg.Config {
	return (*ucfg.Config)(c)
}

// GetFields returns a list of all top-level named keys in c.
func (c *Config) GetFields() []string {
	return c.access().GetFields()
}

func (f *flagOverwrite) String() string {
	return f.value
}

func (f *flagOverwrite) Set(v string) error {
	opts := append(
		[]ucfg.Option{
			ucfg.MetaData(ucfg.Meta{Source: "command line flag"}),
		},
		configOpts...,
	)

	err := f.config.SetString(f.path, -1, v, opts...)
	if err != nil {
		return err
	}
	f.value = v
	return nil
}

func (f *flagOverwrite) Get() interface{} {
	return f.value
}

// Validate checks at most one sub-namespace being set.
func (ns *Namespace) Validate() error {
	if len(ns.C) > 1 {
		return errors.New("more then one namespace configured")
	}
	return nil
}

// Name returns the configuration sections it's name if a section has been set.
func (ns *Namespace) Name() string {
	for name := range ns.C {
		return name
	}
	return ""
}

// Config return the sub-configuration section if a section has been set.
func (ns *Namespace) Config() *Config {
	for _, cfg := range ns.C {
		return cfg
	}
	return nil
}

// IsSet returns true if a sub-configuration section has been set.
func (ns *Namespace) IsSet() bool {
	return len(ns.C) != 0
}

// OwnerHasExclusiveWritePerms asserts that the current user or root is the
// owner of the config file and that the config file is (at most) writable by
// the owner or root (e.g. group and other cannot have write access).
func OwnerHasExclusiveWritePerms(name string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := file.Stat(name)
	if err != nil {
		return err
	}

	euid := os.Geteuid()
	fileUID, _ := info.UID()
	perm := info.Mode().Perm()

	if fileUID != 0 && euid != fileUID {
		return fmt.Errorf(`config file ("%v") must be owned by the user identifier `+
			`(uid=%v) or root`, name, euid)
	}

	// Test if group or other have write permissions.
	if perm&0022 > 0 {
		nameAbs, err := filepath.Abs(name)
		if err != nil {
			nameAbs = name
		}
		return fmt.Errorf(`config file ("%v") can only be writable by the `+
			`owner but the permissions are "%v" (to fix the permissions use: `+
			`'chmod go-w %v')`,
			name, perm, nameAbs)
	}

	return nil
}

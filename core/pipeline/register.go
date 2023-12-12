package pipeline

import (
	"fmt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"strings"
)

type Namespace struct {
	processorReg map[string]processorPluginer
	filterReg    map[string]filterPluginer
}

type processorPlugin struct {
	name string
	c    ProcessorConstructor
}

func (p processorPlugin) ProcessorPlugin() ProcessorConstructor {
	return p.c
}

type filterPlugin struct {
	name string
	c    FilterConstructor
}

func (p filterPlugin) FilterPlugin() FilterConstructor {
	return p.c
}

type processorPluginer interface {
	ProcessorPlugin() ProcessorConstructor
}

type filterPluginer interface {
	FilterPlugin() FilterConstructor
}

func NewNamespace() *Namespace {
	return &Namespace{
		processorReg: map[string]processorPluginer{},
		filterReg:    map[string]filterPluginer{},
	}
}

func (ns *Namespace) RegisterProcessor(name string, factory ProcessorConstructor) error {
	p := processorPlugin{name, NewConditional(factory)}
	names := strings.Split(name, ".")
	if err := ns.addProcessor(names, p); err != nil {
		return fmt.Errorf("plugin %s registration fail %v", name, err)
	}
	return nil
}

func (ns *Namespace) RegisterFilter(name string, factory FilterConstructor) error {
	p := filterPlugin{name, NewFilterConditional(factory)}
	names := strings.Split(name, ".")
	if err := ns.addFilter(names, p); err != nil {
		return fmt.Errorf("plugin %s registration fail %v", name, err)
	}
	return nil
}

func (ns *Namespace) addProcessor(names []string, p processorPluginer) error {
	name := names[0]

	// register plugin if intermediate node in path being processed
	if len(names) == 1 {
		if _, found := ns.processorReg[name]; found {
			return errors.Errorf("%v exists already", name)
		}

		ns.processorReg[name] = p
		return nil
	}

	// check if namespace path already exists
	tmp, found := ns.processorReg[name]
	if found {
		ns, ok := tmp.(*Namespace)
		if !ok {
			return errors.New("non-namespace plugin already registered")
		}
		return ns.addProcessor(names[1:], p)
	}

	// register new namespace
	sub := NewNamespace()
	err := sub.addProcessor(names[1:], p)
	if err != nil {
		return err
	}
	ns.processorReg[name] = sub
	return nil
}

func (ns *Namespace) addFilter(names []string, p filterPluginer) error {
	name := names[0]

	// register plugin if intermediate node in path being processed
	if len(names) == 1 {
		if _, found := ns.filterReg[name]; found {
			return errors.Errorf("%v exists already", name)
		}

		ns.filterReg[name] = p
		return nil
	}

	// check if namespace path already exists
	tmp, found := ns.filterReg[name]
	if found {
		ns, ok := tmp.(*Namespace)
		if !ok {
			return errors.New("non-namespace plugin already registered")
		}
		return ns.addFilter(names[1:], p)
	}

	// register new namespace
	sub := NewNamespace()
	err := sub.addFilter(names[1:], p)
	if err != nil {
		return err
	}
	ns.filterReg[name] = sub
	return nil
}

func (ns *Namespace) ProcessorPlugin() ProcessorConstructor {
	return NewConditional(func(cfg *config.Config) (Processor, error) {
		var section string
		for _, name := range cfg.GetFields() {
			if name == "when" { // TODO: remove check for "when" once fields are filtered
				continue
			}

			if section != "" {
				return nil, errors.Errorf("too many lookup modules "+
					"configured (%v, %v)", section, name)
			}

			section = name
		}

		if section == "" {
			return nil, errors.New("no lookup module configured")
		}

		backend, found := ns.processorReg[section]
		if !found {
			return nil, errors.Errorf("unknown lookup module: %v", section)
		}

		config, err := cfg.Child(section, -1)
		if err != nil {
			return nil, err
		}

		constructor := backend.ProcessorPlugin()
		return constructor(config)
	})
}

func (ns *Namespace) FilterPlugin() FilterConstructor {
	return NewFilterConditional(func(cfg *config.Config) (Filter, error) {
		var section string
		for _, name := range cfg.GetFields() {
			if name == "when" { // TODO: remove check for "when" once fields are filtered
				continue
			}

			if section != "" {
				return nil, errors.Errorf("too many lookup modules "+
					"configured (%v, %v)", section, name)
			}

			section = name
		}

		if section == "" {
			return nil, errors.New("no lookup module configured")
		}

		backend, found := ns.filterReg[section]
		if !found {
			return nil, errors.Errorf("unknown lookup module: %v", section)
		}

		config, err := cfg.Child(section, -1)
		if err != nil {
			return nil, err
		}

		constructor := backend.FilterPlugin()
		return constructor(config)
	})
}

func (ns *Namespace) ProcessorConstructors() map[string]ProcessorConstructor {
	c := make(map[string]ProcessorConstructor, len(ns.processorReg))
	for name, p := range ns.processorReg {
		c[name] = p.ProcessorPlugin()
	}
	return c
}

func (p processorPlugin) Plugin() ProcessorConstructor { return p.c }

func (p filterPlugin) Plugin() FilterConstructor { return p.c }


type FilterConstructor func(config *config.Config) (Filter, error)

type ProcessorConstructor func(config *config.Config) (Processor, error)

type Constructor func(config *config.Config) (ProcessorBase, error)

var registry = NewNamespace()

func RegisterProcessorPlugin(name string, constructor ProcessorConstructor) {
	err := registry.RegisterProcessor(name, constructor)
	if err != nil {
		panic(err)
	}
}

func RegisterFilterPlugin(name string, constructor FilterConstructor) {
		err := registry.RegisterFilter(name, constructor)
	if err != nil {
		panic(err)
	}
}

type FilterProperty struct {
	Type         string      `config:"type" json:"type,omitempty"`
	SubType      string      `config:"sub_type" json:"sub_type,omitempty"`
	DefaultValue interface{} `config:"default_value" json:"default_value,omitempty"`
}

var filterMetadata=map[string]map[string]FilterProperty{}

func ExtractFilterMetadata(filter interface{})map[string]FilterProperty  {

	//extract interface to map[string]FilterProperty{}
	tags:=util.GetFieldAndTags(filter,[]string{"config","type","sub_type","default_value"})
	result:=map[string]FilterProperty{}
	for _,v:=range tags{
		field,ok:=v["config"]
		if ok{
			pro:=FilterProperty{}
			v1,ok:=v["type"]
			if v1!=""&&ok{
				pro.Type=v1
			}else{
				v1,ok:=v["TYPE"]
				if ok{
					pro.Type=v1
				}
			}
			v1,ok=v["sub_type"]
			if v1!=""&&ok{
				pro.SubType=v1
			}else{
				v1,ok:=v["SUB_TYPE"]
				if ok{
					pro.SubType=v1
				}
			}
			v1,ok=v["default_value"]
			if ok{
				switch pro.Type {
				case "bool":
					pro.DefaultValue=v1=="true"
					break
				default:
					pro.DefaultValue=v1
				}
			}
			result[field]=pro
		}
	}

	return result
}

func RegisterFilterConfigMetadata(name string,filter interface{})  {
	filterMetadata[name]=  ExtractFilterMetadata(filter)
}

func RegisterFilterPluginWithConfigMetadata(name string, constructor FilterConstructor,filter interface{}) {
	RegisterFilterPlugin(name,constructor)
	RegisterFilterConfigMetadata(name,filter)
}


func GetFilterMetadata()util.MapStr{
	result:=util.MapStr{}
	for v,_:=range registry.filterReg{
		x,_:=filterMetadata[v]
		result[v]=util.MapStr{
			"properties":x,
		}
	}
	return result
}
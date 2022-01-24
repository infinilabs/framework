package pipeline

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	p "infini.sh/framework/core/plugin"
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

var pluginKey = "basic.processor"

func ProcessorPlugin(name string, c ProcessorConstructor) map[string][]interface{} {
	return p.MakePlugin(pluginKey, processorPlugin{name, c})
}

func FilterPlugin(name string, c FilterConstructor) map[string][]interface{} {
	return p.MakePlugin(pluginKey, filterPlugin{name, c})
}

//func init() {
//	p.MustRegisterLoader(pluginKey, func(ifc interface{}) error {
//		p, ok := ifc.(processorPlugin)
//		if !ok {
//			return errors.New("plugin does not match processor plugin type")
//		}
//
//		return registry.RegisterProcessor(p.name, p.constr)
//	})
//}

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


func GetFilterMetadata(){
	for k,_:=range registry.filterReg{
		log.Error(k)
	}
}
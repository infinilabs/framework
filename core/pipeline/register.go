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

package pipeline

import (
	"fmt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"strings"
	"sync"
)

type Namespace struct {
	processorReg map[string]processorPluginer
	filterReg    map[string]filterPluginer
	sync.RWMutex
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
	ns.Lock()
	defer ns.Unlock()

	p := processorPlugin{name, NewConditional(factory)}
	names := strings.Split(name, ".")
	if err := ns.addProcessor(names, p); err != nil {
		return fmt.Errorf("plugin %s registration fail %v", name, err)
	}
	return nil
}

func (ns *Namespace) RegisterFilter(name string, factory FilterConstructor) error {
	ns.Lock()
	defer ns.Unlock()

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

// processorMetadata carries the extracted config schema of processors
// registered via RegisterProcessorPluginWithConfigMetadata, so that the
// discovery API (see modules/pipeline) can render configuration forms.
var processorMetadata = map[string]map[string]FilterProperty{}

// processorCategories carries the domain/category of processors registered
// via RegisterDomainProcessor (e.g. "event", "parsing", "connector"), so the
// discovery API can group the catalog and consumers can namespace lookups.
// Processors registered without a category default to "general".
var processorCategories = map[string]string{}

// RegisterProcessorPluginWithConfigMetadata registers a processor and
// records the schema of its config struct for discovery.
func RegisterProcessorPluginWithConfigMetadata(name string, constructor ProcessorConstructor, configStruct interface{}) {
	RegisterProcessorPlugin(name, constructor)
	processorMetadata[name] = ExtractFilterMetadata(configStruct)
}

// Domain-qualified processor registration.
//
// Processors keep their bare name as the YAML key (full backward
// compatibility with deployed configs), but gain a category for catalog
// grouping and namespaced lookup. The optional "domain:name" syntax in
// processor configs resolves to the same constructor as the bare name;
// registration conflicts are still detected on the bare name, so a
// same-named processor from a different domain must register explicitly
// qualified (see RegisterQualifiedProcessor).

// RegisterDomainProcessor registers a processor under its bare name with
// a category, plus the "category:name" alias for explicit YAML lookups.
func RegisterDomainProcessor(category, name string, constructor ProcessorConstructor) {
	RegisterProcessorPlugin(name, constructor)
	if category != "" {
		processorCategories[name] = category
		// category-qualified alias (distinct registry entry, same constructor)
		_ = registry.RegisterProcessor(category+":"+name, constructor)
	}
}

// RegisterDomainProcessorWithConfigMetadata is RegisterDomainProcessor
// plus config-schema extraction for the discovery API.
func RegisterDomainProcessorWithConfigMetadata(category, name string, constructor ProcessorConstructor, configStruct interface{}) {
	RegisterDomainProcessor(category, name, constructor)
	processorMetadata[name] = ExtractFilterMetadata(configStruct)
	processorMetadata[category+":"+name] = processorMetadata[name]
}

// ProcessorCategory returns the registered category of a processor
// ("general" when none was declared).
func ProcessorCategory(name string) string {
	if c, ok := processorCategories[name]; ok {
		return c
	}
	return "general"
}

// GetProcessorMetadata returns {name: {properties}} for every registered
// processor.
func GetProcessorMetadata() util.MapStr {
	result := util.MapStr{}
	for name := range registry.ProcessorConstructors() {
		x, _ := processorMetadata[name]
		result[name] = util.MapStr{
			"properties": x,
			"category":   ProcessorCategory(name),
		}
	}
	return result
}

// GetProcessorCatalog groups the processor registry by category for
// designer UIs: {category: {name: {properties, category}}}. Bare names
// only — the domain-qualified aliases are internal resolution detail.
func GetProcessorCatalog() util.MapStr {
	grouped := util.MapStr{}
	for name := range registry.ProcessorConstructors() {
		if strings.Contains(name, ":") {
			continue // qualified alias, not a catalog entry
		}
		cat := ProcessorCategory(name)
		if grouped[cat] == nil {
			grouped[cat] = util.MapStr{}
		}
		catMap := grouped[cat].(util.MapStr)
		x, _ := processorMetadata[name]
		catMap[name] = util.MapStr{
			"properties": x,
			"category":   cat,
		}
	}
	return grouped
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

var filterMetadata = map[string]map[string]FilterProperty{}

func ExtractFilterMetadata(filter interface{}) map[string]FilterProperty {

	//extract interface to map[string]FilterProperty{}
	tags := util.GetFieldAndTags(filter, []string{"config", "type", "sub_type", "default_value"})
	result := map[string]FilterProperty{}
	for _, v := range tags {
		field, ok := v["config"]
		if ok {
			pro := FilterProperty{}
			v1, ok := v["type"]
			if v1 != "" && ok {
				pro.Type = v1
			} else {
				v1, ok := v["TYPE"]
				if ok {
					pro.Type = v1
				}
			}
			v1, ok = v["sub_type"]
			if v1 != "" && ok {
				pro.SubType = v1
			} else {
				v1, ok := v["SUB_TYPE"]
				if ok {
					pro.SubType = v1
				}
			}
			v1, ok = v["default_value"]
			if ok {
				switch pro.Type {
				case "bool":
					pro.DefaultValue = v1 == "true"
					break
				default:
					pro.DefaultValue = v1
				}
			}
			result[field] = pro
		}
	}

	return result
}

func RegisterFilterConfigMetadata(name string, filter interface{}) {
	filterMetadata[name] = ExtractFilterMetadata(filter)
}

func RegisterFilterPluginWithConfigMetadata(name string, constructor FilterConstructor, filter interface{}) {
	RegisterFilterPlugin(name, constructor)
	RegisterFilterConfigMetadata(name, filter)
}

func GetFilterMetadata() util.MapStr {
	result := util.MapStr{}
	for v, _ := range registry.filterReg {
		x, _ := filterMetadata[v]
		result[v] = util.MapStr{
			"properties": x,
		}
	}
	return result
}

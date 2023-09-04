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

package module

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"sort"
)

type Modules struct {
	system  []ModuleItem
	user    []ModuleItem
	configs map[string]interface{}
}

func (receiver *Modules) Sort() {
	// Use sort.SliceStable with a custom sorting function to sort based on both value and priority
	sort.SliceStable(m.system, func(i, j int) bool {
		// Compare by priority first
		if m.system[i].Priority != m.system[j].Priority {
			return m.system[i].Priority < m.system[j].Priority
		}

		// If priority is the same, compare by name
		return m.system[i].Value.Name() < m.system[j].Value.Name()
	})

	sort.SliceStable(m.user, func(i, j int) bool {
		if m.user[i].Priority != m.user[j].Priority {
			return m.user[i].Priority < m.user[j].Priority
		}
		return m.user[i].Value.Name() < m.user[j].Value.Name()
	})
}

var m = &Modules{}

func RegisterModuleWithPriority(mod Module, priority int) {
	m.system = append(m.system, ModuleItem{Value: mod, Priority: priority})
}

func RegisterSystemModule(mod Module) {
	m.system = append(m.system, ModuleItem{Value: mod})
	log.Trace("system:", mod.Name(), ",", m.system)
}

func RegisterUserPlugin(mod Module) {
	m.user = append(m.user, ModuleItem{Value: mod})
	log.Trace("user:", mod.Name(), ",", m.user)
}

func RegisterPluginWithPriority(mod Module, priority int) {
	m.user = append(m.user, ModuleItem{Value: mod, Priority: priority})
	log.Trace("user:", mod.Name(), ",", m.user)
}

type ModuleItem struct {
	Value    Module
	Priority int
}

func checkModuleEnabled(name string) bool {
	cfg := env.GetModuleConfig(name)
	if cfg != nil {
		log.Trace("module: ", name, ", enabled: ", cfg.Enabled(true))
		return cfg.Enabled(true)
	}
	return true
}

func Start() {

	m.Sort()

	log.Trace("start to setup system modules")

	for _, v := range m.system {

		cfg := env.GetModuleConfig(v.Value.Name())

		log.Trace("module: ", v.Value.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("start to setup module: ", v.Value.Name())
			v.Value.Setup()
			log.Debug("setup module: ", v.Value.Name())
		}

	}
	log.Debug("all system module setup finished")

	log.Trace("start to setup user plugins")
	for _, v := range m.user {

		cfg := env.GetPluginConfig(v.Value.Name())

		log.Trace("plugin: ", v.Value.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("start to setup plugin: ", v.Value.Name())
			v.Value.Setup()
			log.Debug("setup plugin: ", v.Value.Name())
		}

	}
	log.Debug("all user plugin setup finished")

	log.Trace("start to start system modules")
	for _, v := range m.system {

		cfg := env.GetModuleConfig(v.Value.Name())

		log.Trace("module: ", v.Value.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("starting module: ", v.Value.Name())
			err := v.Value.Start()
			if err != nil {
				panic(err)
			}
			log.Info("started module: ", v.Value.Name())
		}

	}
	log.Debug("all system module are started")

	log.Trace("start to start user plugins")
	for _, v := range m.user {

		cfg := env.GetPluginConfig(v.Value.Name())

		log.Trace("plugin: ", v.Value.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("starting plugin: ", v.Value.Name())
			err := v.Value.Start()
			if err != nil {
				panic(err)
			}
			log.Info("started plugin: ", v.Value.Name())
		}

	}
	log.Debug("all user plugin are started")

	log.Info("all modules are started")
}

func Stop() {

	log.Trace("start to unload user")
	for i := len(m.user) - 1; i >= 0; i-- {
		v := m.user[i]
		cfg := env.GetPluginConfig(v.Value.Name())
		if cfg.Enabled(true) {
			log.Debug("stopping plugin: ", v.Value.Name())
			v.Value.Stop()
			log.Debug("stopped plugin: ", v.Value.Name())
		}
	}
	log.Debug("all user module are unloaded")

	log.Trace("start to stop system module")
	for i := len(m.system) - 1; i >= 0; i-- {
		v := m.system[i]
		cfg := env.GetModuleConfig(v.Value.Name())
		if cfg.Enabled(true) {
			log.Debug("stopping module: ", v.Value.Name())
			v.Value.Stop()
			log.Debug("stopped module: ", v.Value.Name())
		}
	}
	log.Debug("all system module are stopped")

	log.Info("all modules are stopped")
}

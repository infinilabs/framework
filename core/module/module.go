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
)

type Modules struct {
	system  []Module
	user    []Module
	configs map[string]interface{}
}

var m = &Modules{}

func New() {

}

func RegisterSystemModule(mod Module) {
	m.system = append(m.system, mod)
}

func RegisterUserPlugin(mod Module) {
	m.user = append(m.user, mod)
}

func Start() {

	log.Trace("start to setup system modules")
	for _, v := range m.system {

		cfg := env.GetModuleConfig(v.Name())

		log.Trace("module: ", v.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("start to setup module: ", v.Name())
			v.Setup(cfg)
			log.Debug("setup module: ", v.Name())
		}

	}
	log.Debug("all system module setup finished")

	log.Trace("start to setup user plugins")
	for _, v := range m.user {

		cfg := env.GetPluginConfig(v.Name())

		log.Trace("plugin: ", v.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("start to setup plugin: ", v.Name())
			v.Setup(cfg)
			log.Debug("setup plugin: ", v.Name())
		}

	}
	log.Debug("all user plugin setup finished")

	log.Trace("start to start system modules")
	for _, v := range m.system {

		cfg := env.GetModuleConfig(v.Name())

		log.Trace("module: ", v.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("starting module: ", v.Name())
			err:=v.Start()
			if err!=nil{
				panic(err)
			}
			log.Debug("started module: ", v.Name())
		}

	}
	log.Debug("all system module are started")

	log.Trace("start to start user plugins")
	for _, v := range m.user {

		cfg := env.GetPluginConfig(v.Name())

		log.Trace("plugin: ", v.Name(), ", enabled: ", cfg.Enabled(true))

		if cfg.Enabled(true) {
			log.Trace("starting plugin: ", v.Name())
			err:=v.Start()
			if err!=nil{
				panic(err)
			}
			log.Debug("started plugin: ", v.Name())
		}

	}
	log.Debug("all user plugin are started")

	log.Info("all modules are started")
}

func Stop() {

	log.Trace("start to unload user")
	for i := len(m.user) - 1; i >= 0; i-- {
		v := m.user[i]
		cfg := env.GetPluginConfig(v.Name())
		if cfg.Enabled(true) {
			log.Debug("stopping plugin: ", v.Name())
			v.Stop()
			log.Debug("stopped plugin: ", v.Name())
		}
	}
	log.Debug("all user module are unloaded")

	log.Trace("start to stop system module")
	for i := len(m.system) - 1; i >= 0; i-- {
		v := m.system[i]
		cfg := env.GetModuleConfig(v.Name())
		if cfg.Enabled(true) {
			log.Debug("stopping module: ", v.Name())
			v.Stop()
			log.Debug("stopped module: ", v.Name())
		}
	}
	log.Debug("all system module are stopped")

	log.Info("all modules are stopped")
}

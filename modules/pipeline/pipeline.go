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

package pipeline

import (
	log "github.com/cihub/seelog"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/errors"
	"github.com/infinitbyte/framework/core/global"
)

var started bool
var runners map[string]*PipeRunner

type PipeModule struct {
}

func (module PipeModule) Name() string {
	return "Pipeline"
}

func (module PipeModule) Start(cfg *Config) {

	if started {
		log.Error("pipeline framework already started, please stop it first.")
		return
	}

	var config = struct {
		APIEnabled bool               `config:"api_enabled"`
		Runners    []PipeRunnerConfig `config:"runners"`
	}{
		APIEnabled: true,
		//TODO load default pipe config
		//GetDefaultPipeConfig(),
	}

	cfg.Unpack(&config)

	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", config)
	}

	if config.APIEnabled {

		handler := API{}

		handler.Init()
	}

	runners = map[string]*PipeRunner{}
	for i, v := range config.Runners {
		if v.DefaultConfig == nil {
			panic(errors.Errorf("default pipeline config can't be null, %v, %v", i, v))
		}
		if (v.InputQueue) == "" {
			panic(errors.Errorf("input queue can't be null, %v, %v", i, v))
		}

		p := &PipeRunner{config: v}
		runners[v.Name] = p
	}

	log.Debug("starting up pipeline framework")
	for _, v := range runners {
		v.Start(v.config)
	}

	started = true
}

func (module PipeModule) Stop() error {
	if started {
		started = false
		log.Debug("shutting down pipeline framework")
		for _, v := range runners {
			v.Stop()
		}
	} else {
		log.Error("pipeline framework is not started")
	}

	return nil
}

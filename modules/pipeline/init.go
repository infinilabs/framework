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
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"runtime"
)

var started bool
var runners map[string]*PipeRunner

type PipeModule struct {
}

func (module PipeModule) Name() string {
	return "Pipeline"
}

var moduleCfg = struct {
	APIEnabled bool `config:"api_enabled"`
}{
	APIEnabled: true,
}

func (module PipeModule) Setup(cfg *config.Config) {

	cfg.Unpack(&moduleCfg)

	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	pipeline.RegisterPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterPlugin("echo", NewEchoProcessor)
	//pipeline.RegisterPlugin("processors",NewDAGProcessor)

	if moduleCfg.APIEnabled {
		handler := API{}
		handler.Init()
	}

}

type PipelineConfigV2 struct {
	Name       string                `config:"name"`
	Processors pipeline.PluginConfig `config:"processors"`
}

func (module PipeModule) Start() error {
	if started {
		return errors.New("pipeline framework already started, please stop it first.")
	}

	//TODO, each pipeline could be initialized
	var pipelines []PipelineConfigV2
	ok, err := env.ParseConfig("pipelines_v2", &pipelines)
	if ok && err != nil {
		panic(err)
	}
	if ok {
		for _, v := range pipelines {
			go func() {
				defer func() {
					if !global.Env().IsDebug {
						if r := recover(); r != nil {
							var v string
							switch r.(type) {
							case error:
								v = r.(error).Error()
							case runtime.Error:
								v = r.(runtime.Error).Error()
							case string:
								v = r.(string)
							}
							log.Error(r,v)
						}
					}
				}()
				log.Debug("init pipeline_v2:", v.Name)
				processor, err := pipeline.New(v.Processors)
				if err != nil {
					panic(err)
				}
				err = processor.Process(&pipeline.Context{})
				if err != nil {
					panic(err)
				}
			}()

		}
	}

	//return nil

	//TODO
	//orm.RegisterSchema(pipeline.PipelineConfig{})

	runners = map[string]*PipeRunner{}

	cfgs := pipeline.GetPipelineConfigs()
	for k, v := range cfgs {
		p := &PipeRunner{config: v}
		runners[k] = p
	}

	log.Debug("starting up pipeline framework")
	for _, v := range runners {
		v.Start(v.config)
	}

	started = true
	return nil
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

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
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	"net/http"
	"runtime"
	"time"
)



type PipeModule struct {
	api.Handler
	pipelines map[string]*pipeline.Processors
	contexts map[string]*pipeline.Context
	started bool
	runners map[string]*PipeRunner
}

func (module PipeModule) Name() string {
	return "Pipeline"
}

var moduleCfg = struct {
	APIEnabled bool `config:"api_enabled"`
}{
	APIEnabled: true,
}

func (module *PipeModule) Setup(cfg *config.Config) {

	cfg.Unpack(&moduleCfg)

	if global.Env().IsDebug {
		log.Debug("pipeline framework config: ", moduleCfg)
	}

	module.pipelines= map[string]*pipeline.Processors{}
	module.contexts= map[string]*pipeline.Context{}

	pipeline.RegisterPlugin("dag", pipeline.NewDAGProcessor)
	pipeline.RegisterPlugin("echo", NewEchoProcessor)

	if moduleCfg.APIEnabled {
		handler := API{}
		handler.Init()
		api.HandleAPIMethod(api.GET, "/pipeline/tasks/", module.getPipelines)
		api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_stop", module.stopTask)
		api.HandleAPIMethod(api.POST, "/pipeline/task/:id/_start", module.startTask)
	}

}

type PipelineConfigV2 struct {
	Name       string                `config:"name"`
	Processors pipeline.PluginConfig `config:"processors"`
}


func (module *PipeModule) startTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id:=ps.ByName("id")
	ctx,ok:=module.contexts[id]
	if ok{
		ctx.Start()
		module.WriteAckOKJSON(w)
	}else{
		module.WriteAckJSON(w,false,404,util.MapStr{
			"error":"task not found",
		})
	}
}

func (module *PipeModule) stopTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id:=ps.ByName("id")
	ctx,ok:=module.contexts[id]
	if ok{
		ctx.Stop()
		module.WriteAckOKJSON(w)
	}else{
		module.WriteAckJSON(w,false,404,util.MapStr{
			"error":"task not found",
		})
	}
}

func (module *PipeModule) getPipelines(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj:=map[string]interface{}{}
	for k,_:=range module.pipelines{
		obj[k]=module.contexts[k]
	}
	module.WriteJSON(w,obj,200)
}

func (module *PipeModule) Start() error {
	if module.started {
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

			processor, err := pipeline.New(v.Processors)
			if err != nil {
				log.Error(err)
				continue
			}
			ctx:=pipeline.AcquireContext()
			module.pipelines[v.Name]=processor
			module.contexts[v.Name]=ctx

			go func(p *pipeline.Processors,ctx *pipeline.Context) {
				defer func() {
					if !global.Env().IsDebug {
						if r := recover(); r != nil {
							var err string
							switch r.(type) {
							case error:
								err = r.(error).Error()
							case runtime.Error:
								err = r.(runtime.Error).Error()
							case string:
								err = r.(string)
							}
							log.Errorf("error on pipeline:%v, %v",p.Name(),err)
						}
					}
				}()

				log.Debug("start processing pipeline_v2:", p.Name())

				for {

					if ctx.IsCanceled(){
						log.Infof("task [%v] has been cancelled",p.Name())
						break
					}

					switch ctx.RunningState {
					case pipeline.STARTED:
						log.Debugf("task [%v] started",p.Name())
						ctx.Start()
						err = p.Process(ctx)
						if err != nil {
							ctx.Failed()
						}else{
							ctx.Finished()
						}
						break
					case pipeline.PAUSED:
						time.Sleep(1*time.Second)
						break
					case pipeline.STOPPED:
						ctx.Context.Done()
						log.Debugf("task [%v] stopped",p.Name())
						break
					case pipeline.FINISHED:
						log.Infof("task [%v] finished",p.Name())
						return
					}

					time.Sleep(10*time.Second)
				}

				//some cancel func

			}(processor,ctx)

		}
	}

	//return nil

	//TODO
	//orm.RegisterSchema(pipeline.PipelineConfig{})


	module.runners = map[string]*PipeRunner{}

	cfgs := pipeline.GetPipelineConfigs()

	for k, v := range cfgs {
		p := &PipeRunner{config: v}
		module.runners[k] = p
	}

	log.Debug("starting up pipeline framework")
	for _, v := range module.runners {
		v.Start(v.config)
	}

	module.started = true
	return nil
}

func (module *PipeModule) Stop() error {
	if module.started {
		module.started = false
		log.Debug("shutting down pipeline framework")
		for _, v := range module.runners {
			v.Stop()
		}
	} else {
		log.Error("pipeline framework is not started")
	}

	return nil
}

/*
Copyright Medcl (m AT medcl.net)

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
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"runtime"
	"sync"
	"time"
)

type PipeRunner struct {
	config       PipeRunnerConfig
	lock         sync.Mutex
	quitChannels []*chan bool
}

func (pipe *PipeRunner) Start(config PipeRunnerConfig) {
	if !config.Enabled {
		log.Debugf("pipeline: %s was disabled", config.Name)
		return
	}

	pipe.lock.Lock()
	defer pipe.lock.Unlock()
	pipe.config = config

	numGoRoutine := config.MaxGoRoutine

	pipe.quitChannels = make([]*chan bool, numGoRoutine)
	//start fetcher
	for i := 0; i < numGoRoutine; i++ {
		log.Tracef("start pipeline, %s, instance:%v", config.Name, i)
		signalC := make(chan bool, 1)
		pipe.quitChannels[i] = &signalC
		go pipe.runPipeline(&signalC, i)

	}
	log.Infof("pipeline: %s started with %v instances", config.Name, numGoRoutine)
}

func (pipe *PipeRunner) Pause() {

}

func (pipe *PipeRunner) Resume() {

}

func (pipe *PipeRunner) Update(config PipeRunnerConfig) {
	pipe.Stop()
	pipe.Start(config)
}

func (pipe *PipeRunner) Stop() {
	if !pipe.config.Enabled {
		log.Debugf("pipeline: %s was disabled", pipe.config.Name)
		return
	}
	pipe.lock.Lock()
	defer pipe.lock.Unlock()

	for i, item := range pipe.quitChannels {
		if item != nil {
			*item <- true
		}
		log.Debug("send exit signal to fetch channel, shard:", i)
	}
}

func (pipe *PipeRunner) decodeContext(context []byte) pipeline.Context {
	return pipeline.UnMarshall(context)
}

func (pipe *PipeRunner) runPipeline(signal *chan bool, shard int) {

	if pipe.config.Schedule == "" || pipe.config.Schedule == "once" {
		log.Info("use schedule in pipeline runner")
		context := pipeline.Context{}
		pipe.execute(shard, context, &pipe.config.pipelineConfig)
	} else {
		log.Info("no schedule was defined")
		var inputMessage []byte
		for {
			select {
			case <-*signal:
				log.Trace("pipeline:", pipe.config.Name, " exit, shard:", shard)
				return
			default:

				context := pipeline.Context{}
				if pipe.config.InputQueue != "" {
					inputMessage = <-queue.ReadChan(pipe.config.InputQueue)
					stats.Increment("queue."+string(pipe.config.InputQueue), "pop")

					context = pipe.decodeContext(inputMessage)

					if global.Env().IsDebug {
						log.Trace("pipeline:", pipe.config.Name, ", shard:", shard, " , message received:", util.ToJson(context, true))
					}
				}

				//TODO dynamic load pipeline config
				//url := context.GetStringOrDefault(pipeline.CONTEXT_TASK_URL, "")
				//pipelineConfigID := context.GetStringOrDefault(pipeline.CONTEXT_TASK_PipelineConfigID, "")
				//if pipelineConfigID != "" {
				//	var err error
				//	pipelineConfig, err = pipeline.GetPipelineConfig(pipelineConfigID)
				//	log.Debug("get pipeline config,", pipelineConfig.Name, ",", url, ",", pipelineConfigID)
				//	if err != nil {
				//		panic(err)
				//	}
				//}

				pipe.execute(shard, context, &pipe.config.pipelineConfig)
				log.Trace("pipeline:", pipe.config.Name, ", shard:", shard, " , message ", context.SequenceID, " process finished")
			}
		}
	}
}

func (pipe *PipeRunner) execute(shard int, context pipeline.Context, pipelineConfig *pipeline.PipelineConfig) {
	var p *pipeline.Pipeline
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				if r == nil {
					return
				}
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}

				log.Error("pipeline:", pipe.config.Name, ", shard:", shard, ", sequence:", context.SequenceID, ", err: ", v)
				if p != nil {
					log.Error("instance:", p.GetID(), " ,joint:", p.CurrentProcessor(), "context", util.ToJson(p.GetContext(), true))
				}
			}
		}
	}()

	p = pipeline.NewPipelineFromConfig(pipe.config.Name, pipelineConfig, &context)
	p.Run()

	if pipe.config.ThresholdInMs > 0 {
		log.Debug("pipeline:", pipe.config.Name, ", shard:", shard, ", instance:", p.GetID(), ", sleep ", pipe.config.ThresholdInMs, "ms to control speed")
		time.Sleep(time.Duration(pipe.config.ThresholdInMs) * time.Millisecond)
		log.Debug("pipeline:", pipe.config.Name, ", shard:", shard, ", instance:", p.GetID(), ", wake up now,continue crawing")
	}
}

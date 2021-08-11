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
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
)


type RunningState string

const STARTED RunningState = "STARTED"
const PAUSED RunningState = "PAUSED"
const STOPPED RunningState = "STOPPED"
const FINISHED RunningState = "FINISHED"

type Context struct {
	param.Parameters

	SequenceID   int64       `json:"sequence"`
	IsSimulate   bool        `json:"is_simulate"`
	IgnoreBroken bool        `json:"ignore_broken"`
	Payload      interface{} `json:"-"`

	//private parameters
	breakFlag  bool
	pauseFlag  bool
	exitFlag   bool
	PipelineID string
	flowProcess []string
	finished bool
}

func (ctx *Context)ResetContext()  {
	ctx.ResetParameters()
	ctx.finished=false
	ctx.flowProcess=[]string{}
}
func (ctx *Context)GetFlowProcess()[]string  {
	return ctx.flowProcess
}

func (ctx *Context)GetRequestProcess()[]string  {
	return ctx.flowProcess
}

func (ctx *Context)AddFlowProcess(str string)  {
	if str!=""{
		ctx.flowProcess=append(ctx.flowProcess,str)
	}
}


//is all process finished
func (ctx *Context) Finished() {
	ctx.finished=true
}

//should filters continue to process
func (ctx *Context) ShouldContinue() bool {
	return !ctx.finished
}

// End break all pipelines, but the end phrase not included
func (context *Context) End(msg interface{}) {
	log.Trace("break,", context, ",", msg)
	if context == nil {
		panic(errors.New("context is nil"))
	}
	context.breakFlag = true
	context.Payload = msg
}



//resume processing pipeline, allow filters continue
func (ctx *Context) Resume() {
	ctx.finished=false
	ctx.pauseFlag = false
	ctx.AddFlowProcess("||")
}

func (context *Context) Pause() {
	context.pauseFlag = true
}

func (context *Context) IsPause() bool {
	return context.pauseFlag
}

// IsEnd indicates whether the pipe process is end, end means no more processes will be execute
func (context *Context) IsEnd() bool {
	return context.breakFlag
}

// IsExit means all pipelines will be broke and jump to outside, even the end phrase will not be executed as well
func (context *Context) IsExit() bool {
	return context.exitFlag
}

// Exit tells pipeline to exit
func (context *Context) Exit(msg interface{}) {
	context.exitFlag = true
	context.Payload = msg
}

func (context *Context) Marshall() []byte {
	return util.MustToJSONBytes(context)
}

func UnMarshall(b []byte) Context {
	c := Context{}
	util.MustFromJSONBytes(b, &c)
	return c
}

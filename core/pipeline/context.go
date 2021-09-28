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
	"context"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)


type RunningState string

const STARTED RunningState = "STARTED"
const CANCELLED RunningState = "CANCELLED"
const STOPPED RunningState = "STOPPED"
const FAILED RunningState = "FAILED"
const FINISHED RunningState = "FINISHED"

type Context struct {
	param.Parameters `json:"parameters,omitempty"`

	IsSimulate   bool        `json:"-"`
	IgnoreBroken bool        `json:"-"`
	Payload      interface{} `json:"-"`

	//private parameters
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	RunningState    RunningState `json:"state"`
	ProcessHistory  []string     `json:"-"`
	context.Context `json:"-"`
	cancelFunc context.CancelFunc
	isPaused bool
	pause sync.WaitGroup
}

func AcquireContext()*Context{
	//TODO
	ctx:=Context{}
	ctx.ResetContext()
	return &ctx
}

func ReleaseContext(ctx *Context)  {
	//TODO
}

func (ctx *Context)ResetContext()  {
	ctx.RunningState=STARTED
	t:=time.Now()
	ctx.StartTime=&t
	ctx.EndTime=nil
	ctx.Context,ctx.cancelFunc=context.WithCancel(context.Background())
	ctx.ResetParameters()
	ctx.ProcessHistory =[]string{}
}
func (ctx *Context)GetFlowProcess()[]string  {
	return ctx.ProcessHistory
}

func (ctx *Context)GetRequestProcess()[]string  {
	return ctx.ProcessHistory
}

func (ctx *Context)AddFlowProcess(str string)  {
	if str!=""{
		ctx.ProcessHistory =append(ctx.ProcessHistory,str)
	}
}

func (ctx *Context)IsCanceled()bool  {

	select {
	case <-ctx.Context.Done():
		return true
	default:
		return false
	}
}

func (ctx *Context) Finished() {
	t:=time.Now()
	ctx.EndTime=&t
	ctx.RunningState =FINISHED
}

//should filters continue to process
func (ctx *Context) ShouldContinue() bool {
	return !(ctx.RunningState==FINISHED)
}

// End break all pipelines, but the end phrase not included
func (context *Context) End(msg interface{}) {
	log.Trace("break,", context, ",", msg)
	if context == nil {
		panic(errors.New("context is nil"))
	}
	context.RunningState = STOPPED
	context.Payload = msg
	t:=time.Now()
	context.EndTime=&t
}



func (context *Context) Start() {
	context.ResetContext()
	context.RunningState = STARTED
}

func (context *Context) Failed() {
	context.RunningState = FAILED
	t:=time.Now()
	context.EndTime=&t
}

func (context *Context) Cancelled() {
	context.RunningState = CANCELLED
	t:=time.Now()
	context.EndTime=&t
}

//resume pipeline, set to start mode
func (ctx *Context) Resume() {
	//ctx.RunningState = STARTED
	ctx.isPaused=false
	ctx.pause.Done()
}

//pause and wait signal to resume
func (ctx *Context) Pause() {
	if ctx.isPaused{
		return
	}
	//ctx.RunningState = PAUSED
	ctx.pause.Add(1)
	ctx.isPaused=true
	ctx.pause.Wait()
}

func (context *Context) Stop() {
	context.cancelFunc()
	context.RunningState = STOPPED
	t:=time.Now()
	context.EndTime=&t
}

func (context *Context) IsPause() bool {
	return context.isPaused
	//return context.RunningState==PAUSED
}

// IsEnd indicates whether the pipe process is end, end means no more processes will be execute
func (context *Context) IsEnd() bool {
	return context.RunningState==STOPPED
}

// IsExit means all pipelines will be broke and jump to outside, even the end phrase will not be executed as well
func (context *Context) IsExit() bool {
	return context.RunningState==FINISHED
}

// Exit tells pipeline to exit
func (context *Context) Exit(msg interface{}) {
	context.RunningState = FINISHED
	context.Payload = msg
	t:=time.Now()
	context.EndTime=&t
}

func (context *Context) Marshall() []byte {
	return util.MustToJSONBytes(context)
}

func UnMarshall(b []byte) Context {
	c := Context{}
	util.MustFromJSONBytes(b, &c)
	return c
}

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
	"infini.sh/framework/core/param"
	"sync"
	"time"
)


type RunningState string

const STARTING RunningState = "STARTING"
const STARTED RunningState = "STARTED"
const CANCELLED RunningState = "CANCELLED"
const STOPPING RunningState = "STOPPING"
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
	runningState    RunningState `json:"state"`
	ProcessHistory  []string     `json:"-"`
	context.Context `json:"-"`
	cancelFunc context.CancelFunc
	isPaused bool
	pause sync.WaitGroup
	isQuit bool
	stateLock sync.RWMutex
}

func AcquireContext()*Context{
	//TODO
	ctx:=Context{}
	ctx.ResetContext()
	ctx.runningState=STARTING
	return &ctx
}

func ReleaseContext(ctx *Context)  {
	//TODO
}


func (ctx *Context)GetRunningState()RunningState  {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.runningState

}
func (ctx *Context)ResetContext()  {
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
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	t:=time.Now()
	ctx.EndTime=&t
	ctx.runningState =FINISHED
}

//should filters continue to process
func (ctx *Context) ShouldContinue() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return !(ctx.runningState==FINISHED)
}

func (context *Context) Starting() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.runningState = STARTING
}

func (context *Context) Started() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.ResetContext()
	context.runningState = STARTED
}

func (context *Context) Failed() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.runningState = FAILED
	t:=time.Now()
	context.EndTime=&t
}

func (context *Context) Stopped() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.runningState = STOPPED
	t:=time.Now()
	context.EndTime=&t
}

func (context *Context) Cancelled() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.runningState = CANCELLED
	t:=time.Now()
	context.EndTime=&t
}

//resume pipeline, set to start mode
func (ctx *Context) Resume() {
	ctx.stateLock.Lock()
	ctx.isPaused=false
	ctx.stateLock.Unlock()

	ctx.pause.Done()
}

//pause and wait signal to resume
func (ctx *Context) Pause() {
	ctx.stateLock.Lock()
	if ctx.isPaused{
		return
	}
	ctx.isPaused=true
	ctx.stateLock.Unlock()

	ctx.pause.Add(1)
	ctx.pause.Wait()
}

func (context *Context) CancelTask() {
	context.stateLock.Lock()
	if context.runningState==STARTED||context.runningState==STARTING{
		context.runningState=STOPPING
	}
	context.stateLock.Unlock()

	context.cancelFunc()
}

func (context *Context) IsPause() bool {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	return context.isPaused
}

// IsExit means all pipelines will be broke and jump to outside, even the end phrase will not be executed as well
func (context *Context) IsExit() bool {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	return context.isQuit
}

// Exit tells pipeline to exit
func (context *Context) Exit() {
	context.stateLock.Lock()
	defer context.stateLock.Unlock()

	context.isQuit=true
}
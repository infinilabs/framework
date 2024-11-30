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
	"fmt"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
)

type RunningState string

const STARTING RunningState = "STARTING"
const STARTED RunningState = "STARTED"
const STOPPING RunningState = "STOPPING"
const FAILED RunningState = "FAILED"
const FINISHED RunningState = "FINISHED"
const STOPPED RunningState = "STOPPED"

func (s RunningState) IsEnded() bool {
	return s == FAILED || s == FINISHED || s == STOPPED
}

type StateItem struct {
	Steps             int64
	State             RunningState
	ContextParameters util.MapStr
	ExitErr           error
	ProcessErrs       []error
}

type Context struct {

	ParentContext *Context `json:"-"`

	context.Context  `json:"-"`
	param.Parameters `json:"parameters,omitempty"`

	Config PipelineConfigV2  `json:"-"`

	//private parameters
	createTime     time.Time
	startTime      *time.Time
	endTime        *time.Time
	runningState   RunningState
	exitErr        error
	processErrs    []error
	processHistory []string
	// steps tracks the count of state transition, doesn't reset within the context lifecycle
	id    string
	steps int64

	cancelFunc   context.CancelFunc
	isPaused     bool
	pause        sync.WaitGroup
	isQuit       bool
	stateLock    sync.Mutex
	released     bool
	loopReleased bool
}

func AcquireContext(config PipelineConfigV2) *Context {
	ctx := Context{}
	ctx.ResetContext()
	ctx.id = util.GetUUID()
	ctx.createTime = time.Now()
	ctx.runningState = FINISHED
	ctx.Config = config
	return &ctx
}

// ReleaseContext could be called concurrently
// Doesn't handle context lifecycle, only recycle the resources
// Mark the context as released, quit the pipeline loop automatically
func ReleaseContext(ctx *Context) {
	ctx.stateLock.Lock()
	if ctx.released {
		ctx.stateLock.Unlock()
		return
	}
	ctx.released = true
	ctx.stateLock.Unlock()
}

func (ctx *Context) ID() string {
	return ctx.id
}
func (ctx *Context) IsReleased() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.released
}

func (ctx *Context) SetLoopReleased() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	ctx.loopReleased = true
}

func (ctx *Context) IsLoopReleased() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.loopReleased
}

func (ctx *Context) GetCreateTime() time.Time {
	return ctx.createTime
}

func (ctx *Context) GetStartTime() *time.Time {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.startTime
}

func (ctx *Context) GetEndTime() *time.Time {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.endTime
}

func (ctx *Context) GetRunningState() RunningState {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.runningState
}

// ResetContext only clears the context informations, doesn't modify state values
func (ctx *Context) ResetContext() {
	t := time.Now()
	ctx.startTime = &t
	ctx.endTime = nil
	ctx.Context, ctx.cancelFunc = context.WithCancel(context.Background())
	ctx.exitErr = nil
	ctx.processErrs = []error{}
	ctx.processHistory = []string{}
	ctx.ResetParameters()
}

func (ctx *Context) GetFlowProcess() []string {
	return ctx.processHistory
}

func (ctx *Context) GetRequestProcess() []string {
	return ctx.processHistory
}

func (ctx *Context) AddFlowProcess(str string) {
	if str != "" {
		ctx.processHistory = append(ctx.processHistory, str)
	}
}

func (ctx *Context) IsFailed() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()
	return ctx.runningState == FAILED
}

// should filters continue to process
func (ctx *Context) ShouldContinue() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return !(ctx.runningState == FINISHED)
}

func (ctx *Context) IsCanceled() bool {
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

	t := time.Now()
	ctx.endTime = &t
	ctx.setRunningState(FINISHED)
}

func (ctx *Context) Failed(err error) {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.exitErr = err
	ctx.setRunningState(FAILED)
	t := time.Now()
	ctx.endTime = &t
}

func (ctx *Context) Starting() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.setRunningState(STARTING)
}

// Only STARTING/STARTED pipelines will get marked as STOPPING
// Other states happen when pipeline finished, no need to mark it again.
func (ctx *Context) Stopping() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	if ctx.runningState == STARTED || ctx.runningState == STARTING {
		ctx.setRunningState(STOPPING)
	}
}

func (ctx *Context) Stopped() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.setRunningState(STOPPED)
}

func (ctx *Context) Started() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.setRunningState(STARTED)
}

func (ctx *Context) RecordError(err error) {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.processErrs = append(ctx.processErrs, err)
}

func (ctx *Context) HasError() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return len(ctx.processErrs) > 0
}

func (ctx *Context) Errors() []error {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return ctx.processErrs
}

// Pause will pause the pipeline running loop until Resume called
func (ctx *Context) Pause() {
	ctx.stateLock.Lock()
	if ctx.isPaused {
		ctx.stateLock.Unlock()
		return
	}
	ctx.isPaused = true
	ctx.stateLock.Unlock()

	ctx.pause.Add(1)
	ctx.pause.Wait()
}

// Resume recovers pipeline from Pause
func (ctx *Context) Resume() {
	ctx.stateLock.Lock()
	if !ctx.isPaused {
		ctx.stateLock.Unlock()
		return
	}
	ctx.isPaused = false
	ctx.stateLock.Unlock()

	ctx.pause.Done()
}

// Restart marks the pipeline as ready to run
func (ctx *Context) Restart() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.isQuit = false
}

// IsExit means pipeline has been manually stopped, will not running until Restart
func (ctx *Context) IsExit() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return ctx.isQuit
}

// Exit tells pipeline to exit
func (ctx *Context) Exit() {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	ctx.isQuit = true
}

func (ctx *Context) CancelTask() {
	ctx.cancelFunc()
}

func (ctx *Context) IsPause() bool {
	ctx.stateLock.Lock()
	defer ctx.stateLock.Unlock()

	return ctx.isPaused
}

// setRunningState must be called after holding stateLock
func (ctx *Context) setRunningState(newState RunningState) {
	// Don't allow state modifications after ReleaseContext()
	if ctx.released {
		return
	}
	oldState := ctx.runningState
	ctx.runningState = newState
	if oldState != newState {
		ctx.steps++

		if ctx.Config.Logging.Enabled {
			ctx.pushPipelineLog()
		}
	}
}

func (ctx *Context) pushPipelineLog() {
	if global.Env().IsDebug {
		log.Info("received pipeline state change, id: ", ctx.Config.Name, ", state: ", ctx.runningState)
	}
	eventData := event.Event{
		Metadata: event.EventMetadata{
			Category: "pipeline",
			Name:     "logging",
			Datatype: "event",
		},
	}
	labels := util.MapStr{
		"task_id":    ctx.Config.Name,
		"context_id": ctx.id,
	}
	for k, v := range ctx.Config.Labels {
		labels[k] = v
	}
	eventData.Metadata.Labels = labels
	payload := util.MapStr{
		"steps":   ctx.steps,
		"status":  string(ctx.runningState),
		"config":  util.MustToJSON(ctx.Config),
		"context": ctx.Parameters.CloneData(),
	}
	if ctx.runningState.IsEnded() {
		result := util.MapStr{
			"success": ctx.exitErr == nil,
		}
		if ctx.exitErr != nil || len(ctx.processErrs) > 0 {
			result["error"] = fmt.Sprintf("exit: %v, process: %v", ctx.exitErr, ctx.processErrs)
		}
		payload["result"] = result
	}
	eventData.Fields = util.MapStr{
		"pipeline": util.MapStr{
			"logging": payload,
		},
	}

	event.SaveLog(&eventData)
}

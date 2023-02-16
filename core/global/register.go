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

package global

import (
	"context"
	"errors"
	"fmt"
	"infini.sh/framework/core/env"
	"runtime"
	log "github.com/cihub/seelog"
	"sync"
	"sync/atomic"
	"time"
)

// RegisterKey is used to register custom value and retrieve back
type RegisterKey string

type registrar struct {
	values map[RegisterKey]interface{}
	sync.Mutex
}

var (
	r      *registrar
	l      sync.RWMutex
	inited bool
	e      *env.Env
)

func getRegistrar() *registrar {
	if !inited {
		l.Lock()
		if !inited {
			r = &registrar{values: map[RegisterKey]interface{}{}}
			inited = true
		}
		l.Unlock()
		runtime.Gosched()
	}
	return r
}

// Register is used to register your own key and value
func Register(k RegisterKey, v interface{}) {
	reg := getRegistrar()
	if reg == nil {
		return
	}

	reg.Lock()
	defer reg.Unlock()
	reg.values[k] = v
}

func MustLookupString(k RegisterKey) string {
	v:=MustLookup(k)
	return v.(string)
}

func MustLookup(k RegisterKey) interface{} {
	v:=Lookup(k)
	if v==nil{
		panic(errors.New(fmt.Sprintf("invalid key: %v",k)))
	}
	return v
}

func Lookup(k RegisterKey) interface{} {
	reg := getRegistrar()
	if reg == nil {
		return nil
	}

	reg.Lock()
	defer reg.Unlock()
	return reg.values[k]
}

// RegisterEnv is used to register env to this register hub
func RegisterEnv(e1 *env.Env) {
	e = e1
}

// Env returns registered env, should be available globally
func Env() *env.Env {
	if e == nil {
		ev := env.EmptyEnv()
		ev.Init()
		RegisterEnv(ev)
	}
	return e
}

var shutdownCallback = []func(){}

func RegisterShutdownCallback(callback func()) {
	registerLock.Lock()
	defer registerLock.Unlock()
	shutdownCallback = append(shutdownCallback, callback)
}

func ShutdownCallback() []func() {
	registerLock.Lock()
	defer registerLock.Unlock()
	return shutdownCallback
}

type BackgroundTask struct {
	Tag string
	Func func()
	lastRunning time.Time
	Interval time.Duration
}

var backgroundCallback = sync.Map{}
var registerLock=sync.Mutex{}
func RegisterBackgroundCallback(task *BackgroundTask) {
	backgroundCallback.Store(task.Tag,task)
}


func FuncWithTimeout(ctx context.Context,f func()) error {
	ctx, cancel := context.WithTimeout(ctx,1*time.Second)
	defer func() {
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
			log.Error("error: ", v)
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	default:
		f()
		return nil
	}
}

func RunBackgroundCallbacks(continueRun *int32) {
	ctx := context.Background()
	for {
		if atomic.LoadInt32(continueRun)==1{
			log.Debug("exit background tasks")
			return
		}
		timeStart:=time.Now()
		backgroundCallback.Range(func(key, value any) bool {
			v:=value.(*BackgroundTask)
			if time.Since(v.lastRunning)>v.Interval{
				log.Debugf("run background job:%v, interval:%v", key,v.Interval)
				err := FuncWithTimeout(ctx,v.Func)
				if err != nil {
					log.Error(fmt.Sprintf("error on running background job: %v, %v",key,err))
				}
				v.lastRunning=time.Now()
			}
			return true
		})

		if time.Since(timeStart)<time.Second{
			time.Sleep(10*time.Second)
		}
	}
}

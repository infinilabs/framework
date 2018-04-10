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

package queue

import (
	"errors"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/infinitbyte/framework/core/stats"
	"sync"
	"time"
)

type Queue interface {
	Push(string, []byte) error
	Pop(string, time.Duration) (error, []byte)
	ReadChan(k string) chan []byte
	Close(string) error
	Depth(string) int64
	GetQueues() []string
}

var handler Queue

func Push(k string, v []byte) error {
	var err error = nil
	if handler != nil {
		err = handler.Push(k, v)
		if err == nil {
			stats.Increment("queue."+k, "push")
			return nil
		}
		stats.Increment("queue."+k, "push_error")
		return err
	}
	panic(errors.New("channel is not registered"))
}

var pauseMsg = errors.New("queue was paused to read")

func ReadChan(k string) chan []byte {
	if handler != nil {
		if pausedReadQueue.Contains(k) {
			pauseLock.Lock()
			pauseCount[k]++
			pauseLock.Unlock()
			log.Debugf("queue: %s was paused to read", k)
			<-pauseChan[k]
			log.Debugf("queue: %s was resumed to read", k)
		}
		return handler.ReadChan(k)
	}
	stats.Increment("queue."+k, "read_chan_error")
	panic(errors.New("channel is not registered"))
}

func Pop(k string) (error, []byte) {
	if handler != nil {
		if pausedReadQueue.Contains(k) {
			return pauseMsg, nil
		}

		er, o := handler.Pop(k, -1)
		if er == nil {
			stats.Increment("queue."+k, "pop")
			return er, o
		}
		stats.Increment("queue."+k, "pop_error")
		return er, o
	}
	panic(errors.New("channel is not registered"))
}

func PopTimeout(k string, timeoutInSeconds time.Duration) (error, []byte) {
	if timeoutInSeconds < 1 {
		timeoutInSeconds = 5
	}

	if handler != nil {

		if pausedReadQueue.Contains(k) {
			return pauseMsg, nil
		}

		er, o := handler.Pop(k, timeoutInSeconds)
		if er == nil {
			stats.Increment("queue."+k, "pop")
		}
		stats.Increment("queue."+k, "pop_error")
		return er, o
	}
	panic(errors.New("channel is not registered"))
}

func Close(k string) error {
	if handler != nil {
		o := handler.Close(k)
		stats.Increment("queue."+k, "close")
		return o
	}
	stats.Increment("queue."+k, "close_error")
	panic(errors.New("channel is not closed"))
}

func Depth(k string) int64 {
	if handler != nil {
		o := handler.Depth(k)
		stats.Increment("queue."+k, "call_depth")
		return o
	}
	panic(errors.New("channel is not registered"))
}

func GetQueues() []string {
	if handler != nil {
		o := handler.GetQueues()
		stats.Increment("queue.", "get_queues")
		return o
	}
	panic(errors.New("channel is not registered"))
}

var pausedReadQueue = hashset.New()
var pauseChan map[string]chan bool = map[string]chan bool{}
var pauseCount = map[string]int{}
var pauseLock sync.Mutex

func PauseRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pauseCount[k] = 0
	pauseChan[k] = make(chan bool)
	pausedReadQueue.Add(k)
}
func ResumeRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pausedReadQueue.Remove(k)
	size := pauseCount[k]
	for i := 0; i < size; i++ {
		pauseChan[k] <- true
	}
	log.Debugf("queue: %s was resumed, signal: %v", k, size)
}

func Register(h Queue) {
	handler = h
}

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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package mem_queue

import (
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/env"
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/queue"
	"github.com/rubyniu105/framework/core/stats"
	"github.com/rubyniu105/framework/core/util"
	memQueue "github.com/rubyniu105/framework/lib/lock_free/queue"
	"runtime"
	"sync"
	"time"
)

type MemoryQueue struct {
	Capacity uint32 `config:"capacity"`
	Default  bool   `config:"default"`
	Enabled  bool   `config:"enabled"`

	MemorySize int `config:"total_memory_size"`
	q          sync.Map
	locker     sync.RWMutex
}

func (this *MemoryQueue) Setup() {

	this.q = sync.Map{}
	this.Enabled = true
	this.MemorySize = 2 * 1024 * 1024
	this.Capacity = 10000
	ok, err := env.ParseConfig("memory_queue", &this)
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}
	//queue.Register("memory",this)
	//if this.Default{
	//	queue.RegisterDefaultHandler(this)
	//}

}

func (this *MemoryQueue) Start() error {
	return nil
}

func (this *MemoryQueue) Stop() error {
	return nil
}

func (this *MemoryQueue) Name() string {
	return "memory_queue"
}

func (this *MemoryQueue) Init(q string) error {
	q1 := memQueue.NewQueue(this.Capacity)
	this.q.Store(q, q1)
	return nil
}

func (this *MemoryQueue) Push(q string, data []byte) error {
	q1, ok := this.q.Load(q)
	if !ok {
		err := this.Init(q)
		if err != nil {
			panic(err)
		}
		q1, _ = this.q.Load(q)
	}

	retryTimes := 0
	da := []byte(string(data)) //TODO memory copy
	mq, ok := q1.(*memQueue.EsQueue)
	if !ok {
		panic("invalid memory queue")
	}

RETRY:
	ok, _ = mq.Put(da)
	if !ok {
		if retryTimes > 3 {
			stats.Increment("mem_queue", "dead_retry")
			return capacityFull
		} else {
			retryTimes++
			runtime.Gosched()
			log.Debugf("memory_queue %v of %v, sleep 1s", mq.Quantity(), mq.Capaciity())
			time.Sleep(1000 * time.Millisecond)
			stats.Increment("mem_queue", "retry")
			goto RETRY
		}
	}
	return nil
}

var capacityFull = errors.New("memory capacity full")

func (this *MemoryQueue) Pop(q string, t time.Duration) (data []byte, timeout bool) {
	queue, ok := this.q.Load(q)
	if !ok || queue == nil {
		return nil, true
	}

	mq, ok := queue.(*memQueue.EsQueue)
	if !ok {
		panic("invalid memory queue")
	}

	v, ok, _ := mq.Get()
	if ok && v != nil {
		d, ok := v.([]byte)
		if ok {
			return d, false
		}
	}
	return nil, true
}

func (this *MemoryQueue) Close(string) error {
	return nil
}

func (this *MemoryQueue) Depth(q string) int64 {
	q1, ok := this.q.Load(q)
	if ok {
		mq, ok := q1.(*memQueue.EsQueue)
		if !ok {
			panic("invalid memory queue")
		}
		return int64(mq.Quantity())
	}
	return 0
}

func (this *MemoryQueue) Consume(q *queue.QueueConfig, consumer *queue.ConsumerConfig, offsetStr string) (*queue.Context, []queue.Message, bool, error) {
	ctx := &queue.Context{}
	d, t := this.Pop(q.ID, consumer.GetFetchMaxWaitMs())
	msg := queue.Message{Data: d}
	msgs := []queue.Message{msg}
	return ctx, msgs, t, nil
}

func (this *MemoryQueue) LatestOffset(string) string {
	return ""
}

func (this *MemoryQueue) GetQueues() []string {
	q := []string{}
	this.q.Range(func(key, value interface{}) bool {
		q = append(q, util.ToString(key))
		return true
	})
	return q
}

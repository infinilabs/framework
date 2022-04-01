/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package mem_queue

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	memQueue "infini.sh/framework/lib/lock_free/queue"
	"sync"
	"time"
)

type MemoryQueue struct {

	Capacity uint32 `config:"capacity"`

	MemorySize int `config:"total_memory_size"`
	q          map[string]*memQueue.EsQueue
	locker     sync.RWMutex
}

func (this *MemoryQueue) Setup(config *config.Config) {

	this.q= map[string]*memQueue.EsQueue{}
	this.MemorySize=2*1024*1024
	this.Capacity=10000
	ok, err := env.ParseConfig("memory_queue", &this)
	if ok && err != nil {
		panic(err)
	}
	queue.Register("memory",this)
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

func (this *MemoryQueue)Init(q string) error{
	_,ok:=this.q[q]
	if !ok{
		this.locker.Lock()
		q1:= memQueue.NewQueue(this.Capacity)
		this.q[q]=q1
		this.locker.Unlock()
	}
	return nil
}

func (this *MemoryQueue)Push(q string,data []byte) error{
	_,ok:=this.q[q]
	if !ok{
		this.Init(q)
	}
	this.locker.Lock()
	q1:=this.q[q]
	this.locker.Unlock()
	retryTimes:=0
	da:=[]byte(string(data)) //TODO memory copy
	RETRY:
	ok,_=q1.Put(da)
	if !ok{
		if retryTimes>10{
			stats.Increment("mem_queue","dead_retry")
			return capacityFull
		}else{
			retryTimes++
			//runtime.Gosched()
			time.Sleep(1000*time.Millisecond)
			stats.Increment("mem_queue","retry")
			goto RETRY
		}
	}
	return nil
}

var capacityFull =errors.New("memory capacity full")

func (this *MemoryQueue)Pop(q string, t time.Duration) (data []byte, timeout bool){
	if this.q==nil{
		return nil, true
	}

	queue,ok:=this.q[q]
	if !ok||queue==nil{
		return nil, true
	}

	this.locker.Lock()
	defer this.locker.Unlock()
	v,ok,_:=queue.Get()
	if ok&&v!=nil{
		d,ok:=v.([]byte)
		if ok{
			return d,false
		}
	}
	return nil, true
}

func (this *MemoryQueue)Close(string) error{
	return nil
}

func (this *MemoryQueue)Depth(q string) int64{
	return int64(this.q[q].Quantity())
}

func (this *MemoryQueue)Consume(q,consumer,offsetStr string,count int,timeout time.Duration) ( *queue.Context, []queue.Message,bool,error){
	ctx:=&queue.Context{}
	d,t:=this.Pop(q,timeout)
	msg:=queue.Message{Data: d}
	msgs:=[]queue.Message{msg}
	return ctx, msgs, t, nil
}

func (this *MemoryQueue)LatestOffset(string) string{
	return ""
}

func (this *MemoryQueue)GetQueues() []string{
	q:=[]string{}
	if this.q!=nil{
		for k,_:=range this.q{
			q=append(q,k)
		}
	}
	return q
}

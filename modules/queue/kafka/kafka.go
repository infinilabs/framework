/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	memQueue "infini.sh/framework/lib/lock_free/queue"
	"runtime"
	"sync"
	"time"
	"github.com/segmentio/kafka-go"

)

type KafkaQueue struct {

	Enabled bool `config:"enabled"`
	Default bool `config:"default"`

	BatchSize        int      `config:"batch_size"`
	BatchTimeoutInMs int      `config:"batch_timeout_in_ms"`
	RequiredAcks     int      `config:"required_acks"`
	Brokers          []string `config:"brokers"`

	//share
	msgPool     *sync.Pool

	taskContext context.Context

	q          sync.Map
	locker     sync.RWMutex
}

func (this *KafkaQueue) Setup(config *config.Config) {

	this.q=sync.Map{}
	this.Enabled=true
	ok, err := env.ParseConfig("kafka_queue", &this)
	if ok && err != nil {
		panic(err)
	}

	this.taskContext = context.Background()

	this.msgPool = &sync.Pool{
		New: func() interface{} {
			return kafka.Message{}
		},
	}

	queue.Register("kafka",this)
	if this.Default{
		queue.RegisterDefaultHandler(this)
	}


}

func (this *KafkaQueue) Start() error {
	return nil
}

func (this *KafkaQueue) Stop() error {
	return nil
}

func (this *KafkaQueue) Name() string {
	return "kafka_queue"
}

func (this *KafkaQueue)Init(q string) (error){

	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      this.Brokers,
		Topic:        q,
		BatchSize:    this.BatchSize,
		BatchTimeout: time.Duration(this.BatchTimeoutInMs) * time.Millisecond,
		RequiredAcks: this.RequiredAcks,
	})

	this.q.Store(q,w)

	return nil
}

func (this *KafkaQueue)Push(q string,data []byte) error{
	q1,ok:=this.q.Load(q)
	if !ok{
		err:=this.Init(q)
		if err!=nil{
			panic(err)
		}
		q1,_=this.q.Load(q)
	}

	retryTimes:=0
	da:=[]byte(string(data)) //TODO memory copy
	mq,ok:=q1.(*kafka.Writer)
	if !ok{
		panic("invalid kafka queue")
	}

	messages := []kafka.Message{}

	msg := this.msgPool.Get().(kafka.Message)
	msg.Key = util.UnsafeStringToBytes(util.Int64ToString(util.GetIncrementID(q)))
	msg.Value = da

	messages = append(messages, msg)


RETRY:
	//ok,_=mq.Put(da)

	err := mq.WriteMessages(this.taskContext, messages...)


	//if !ok{
	//	if retryTimes>10{
	//		stats.Increment("mem_queue","dead_retry")
	//		return capacityFull
	//	}else{
	//		retryTimes++
	//		runtime.Gosched()
	//		log.Debugf("memory_queue %v of %v, sleep 1s",mq.Quantity(),mq.Capaciity())
	//		time.Sleep(1000*time.Millisecond)
	//		stats.Increment("mem_queue","retry")
	//		goto RETRY
	//	}
	//}
	return nil
}

var capacityFull =errors.New("queue capacity full")

func (this *KafkaQueue)Pop(q string, t time.Duration) (data []byte, timeout bool){
	queue,ok:=this.q.Load(q)
	if !ok||queue==nil{
		return nil, true
	}

	mq,ok:=queue.(*memQueue.EsQueue)
	if !ok{
		panic("invalid memory queue")
	}

	v,ok,_:=mq.Get()
	if ok&&v!=nil{
		d,ok:=v.([]byte)
		if ok{
			return d,false
		}
	}
	return nil, true
}

func (this *KafkaQueue)Close(string) error{
	return nil
}

func (this *KafkaQueue)Depth(q string) int64{
	q1,ok:=this.q.Load(q)
	if ok{
		mq,ok:=q1.(*memQueue.EsQueue)
		if !ok{
			panic("invalid memory queue")
		}
		return int64(mq.Quantity())
	}
	return 0
}

func (this *KafkaQueue)Consume(q *queue.QueueConfig,consumer *queue.ConsumerConfig,offset string) ( *queue.Context, []queue.Message,bool,error){
	ctx:=&queue.Context{}
	d,t:=this.Pop(q.Id,consumer.GetFetchMaxWaitMs())
	msg:=queue.Message{Data: d}
	msgs:=[]queue.Message{msg}
	return ctx, msgs, t, nil
}

func (this *KafkaQueue)LatestOffset(string) string{
	return ""
}

func (this *KafkaQueue)GetQueues() []string{
	q:=[]string{}
	this.q.Range(func(key, value interface{}) bool {
		q=append(q,util.ToString(key))
		return true
	})
	return q
}

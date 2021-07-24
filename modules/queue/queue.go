package queue

import (
	"infini.sh/framework/core/errors"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	. "infini.sh/framework/modules/queue/disk_queue"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var queues sync.Map=sync.Map{}
//map[string]*BackendQueue

type DiskQueue struct {
}

func (module DiskQueue) Name() string {
	return "Queue"
}

var initLocker sync.Mutex

//min_msg_size: 1
//max_msg_size: 500000000 #500,000,000
//max_bytes_per_file: 50*1024*1024*1024
//sync_every_in_seconds: 10
//sync_timeout_in_seconds: 10
//read_chan_buffer: 0
type QueueConfig struct {
	MinMsgSize       int   `config:"min_msg_size"`
	MaxMsgSize       int   `config:"max_msg_size"`
	MaxBytesPerFile  int64 `config:"max_bytes_per_file"`
	SyncEveryRecords int64 `config:"sync_every_records"`
	SyncTimeoutInMS  int   `config:"sync_timeout_in_ms"`
	ReadChanBuffer   int   `config:"read_chan_buffer_size"`
	WriteChanBuffer   int   `config:"write_chan_buffer_size"`
}

var cfg *QueueConfig

func (module DiskQueue) initQueue(name string) error {
	initLocker.Lock()
	defer initLocker.Unlock()

	_,ok:= queues.Load(name)
	if ok{
		return nil
	}

	log.Debugf("init queue: %s", name)

	dataPath := path.Join(global.Env().GetDataDir(), "queue", strings.ToLower(name))
	os.MkdirAll(dataPath, 0755)

	tempQueue := NewDiskQueue(strings.ToLower(name), dataPath, cfg.MaxBytesPerFile, int32(cfg.MinMsgSize), int32(cfg.MaxMsgSize), cfg.SyncEveryRecords, time.Duration(cfg.SyncTimeoutInMS), cfg.ReadChanBuffer,cfg.WriteChanBuffer)

	queues.Store(name,&tempQueue)

	return nil
}
var diskQueue *DiskQueue

func (module DiskQueue) Setup(config *config.Config) {

	cfg = &QueueConfig{
		MinMsgSize:       1,
		MaxMsgSize:       104857600, //100MB
		MaxBytesPerFile:  10 * 1024 * 1024 * 1024, //10GB
		SyncEveryRecords: 1000,
		SyncTimeoutInMS:  1000,
		ReadChanBuffer:   0,
		WriteChanBuffer:   0,
	}
	diskQueue=&DiskQueue{}

	ok,err:=env.ParseConfig("queue", cfg)
	if ok&&err!=nil{
		panic(err)
	}

	//queues = make(map[string]*BackendQueue)

	RegisterAPI()
}

func (module DiskQueue) Push(k string, v []byte) error {
	module.initQueue(k)
	q,ok:=queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Put(v)
	}
	return errors.Errorf("queue [%v] not found",k)
}

func (module DiskQueue) ReadChan(k string) <-chan []byte{
	module.initQueue(k)
	q,ok:=queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).ReadChan()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module DiskQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {
	err:=module.initQueue(k)
	if err!=nil{
		panic(err)
	}

	if timeoutDuration > 0 {
		to := time.NewTimer(timeoutDuration)
		for {
			to.Reset(timeoutDuration)
			select {
			case b := <-module.ReadChan(k):
				return b,false
			case <-to.C:
				return nil,true
			}
		}
	} else {
		b := <-module.ReadChan(k)
		return b,false
	}
}

func (module DiskQueue) Close(k string) error {
	q,ok:=queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Close()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module DiskQueue) Depth(k string) int64 {
	module.initQueue(k)
	q,ok:=queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Depth()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module DiskQueue) GetQueues() []string {
	result := []string{}

	queues.Range(func(key, value interface{}) bool {
		result = append(result, key.(string))
		return true
	})
	return result
}

func (module DiskQueue) Start() error {
	queue.Register("disk", diskQueue)
	return nil
}

func (module DiskQueue) Stop() error {

	queues.Range(func(key, value interface{}) bool {
		q,ok:=queues.Load(key)
		if ok{
			err := (*q.(*BackendQueue)).Close()
			if err != nil {
				log.Error(err)
			}
		}
		return true
	})
	return nil
}

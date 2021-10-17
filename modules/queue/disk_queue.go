package queue

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	. "infini.sh/framework/modules/queue/disk_queue"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type DiskQueue struct {
	cfg *DiskQueueConfig
	initLocker sync.Mutex
	api.Handler
	queues sync.Map
}

func (module *DiskQueue) Name() string {
	return "disk_queue"
}

type DiskQueueConfig struct {
	MinMsgSize       int   `config:"min_msg_size"`
	MaxMsgSize       int   `config:"max_msg_size"`
	MaxBytesPerFile  int64 `config:"max_bytes_per_file"`
	SyncEveryRecords int64 `config:"sync_every_records"`
	SyncTimeoutInMS  int   `config:"sync_timeout_in_ms"`
	ReadChanBuffer   int   `config:"read_chan_buffer_size"`
	WriteChanBuffer   int   `config:"write_chan_buffer_size"`
}

func (module *DiskQueue) initQueue(name string) error {
	module.initLocker.Lock()
	defer module.initLocker.Unlock()

	_,ok:= module.queues.Load(name)
	if ok{
		return nil
	}

	log.Debugf("init queue: %s", name)

	dataPath := path.Join(global.Env().GetDataDir(), "queue", strings.ToLower(name))
	os.MkdirAll(dataPath, 0755)

	tempQueue := NewDiskQueue(strings.ToLower(name), dataPath, module.cfg.MaxBytesPerFile, int32(module.cfg.MinMsgSize), int32(module.cfg.MaxMsgSize), module.cfg.SyncEveryRecords, time.Duration(module.cfg.SyncTimeoutInMS), module.cfg.ReadChanBuffer, module.cfg.WriteChanBuffer)

	module.queues.Store(name,&tempQueue)

	return nil
}

func (module *DiskQueue) Setup(config *config.Config) {

	module.cfg = &DiskQueueConfig{
		MinMsgSize:       1,
		MaxMsgSize:       104857600, //100MB
		MaxBytesPerFile:  1 * 1024 * 1024 * 1024, //1GB
		SyncEveryRecords: 1000,
		SyncTimeoutInMS:  1000,
		ReadChanBuffer:   0,
		WriteChanBuffer:   0,
	}

	ok,err:=env.ParseConfig("disk_queue", module.cfg)
	if ok&&err!=nil{
		panic(err)
	}

	module.queues=sync.Map{}

	module.RegisterAPI()
}

func (module *DiskQueue) Push(k string, v []byte) error {
	module.initQueue(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Put(v)
	}
	return errors.Errorf("queue [%v] not found",k)
}

func (module *DiskQueue) ReadChan(k string) <-chan []byte{
	module.initQueue(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).ReadChan()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {
	err:= module.initQueue(k)
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

func (module *DiskQueue) Close(k string) error {
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Close()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Depth(k string) int64 {
	module.initQueue(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Depth()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) GetQueues() []string {
	result := []string{}

	module.queues.Range(func(key, value interface{}) bool {
		result = append(result, key.(string))
		return true
	})
	return result
}

func (module *DiskQueue) Start() error {
	queue.Register("disk", module)
	queue.RegisterDefaultHandler(module)
	return nil
}

func (module *DiskQueue) Stop() error {

	module.queues.Range(func(key, value interface{}) bool {
		q,ok:=module.queues.Load(key)
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

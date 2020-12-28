package queue

import (
	"errors"
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

var queues map[string]*BackendQueue

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
	ReadChanBuffer   int   `config:"read_chan_buffer"`
}

var cfg *QueueConfig

func (module DiskQueue) initQueue(name string) error {

	channel := "default"

	if queues[name] != nil {
		return nil
	}

	initLocker.Lock()
	defer initLocker.Unlock()

	//double check after lock in
	if queues[name] != nil {
		return nil
	}

	log.Debugf("init queue: %s", name)

	dataPath := path.Join(global.Env().GetWorkingDir(), "queue", strings.ToLower(name))
	os.MkdirAll(dataPath, 0755)

	q := NewDiskQueue(strings.ToLower(channel), dataPath, cfg.MaxBytesPerFile, int32(cfg.MinMsgSize), int32(cfg.MaxMsgSize), cfg.SyncEveryRecords, time.Duration(cfg.SyncTimeoutInMS), cfg.ReadChanBuffer)
	queues[name] = &q

	return nil
}
var diskQueue *DiskQueue

func (module DiskQueue) Setup(config *config.Config) {

	cfg = &QueueConfig{
		MinMsgSize:       1,
		MaxMsgSize:       5000000000,
		MaxBytesPerFile:  50 * 1024 * 1024 * 1024,
		SyncEveryRecords: 1000,
		SyncTimeoutInMS:  1000,
		ReadChanBuffer:   0,
	}
	diskQueue=&DiskQueue{}

	ok,err:=env.ParseConfig("queue", cfg)
	if ok&&err!=nil{
		panic(err)
	}

	queues = make(map[string]*BackendQueue)
}

func (module DiskQueue) Push(k string, v []byte) error {
	module.initQueue(k)
	return (*queues[k]).Put(v)
}

func (module DiskQueue) ReadChan(k string) chan []byte {
	module.initQueue(k)
	return (*queues[k]).ReadChan()
}

func (module DiskQueue) Pop(k string, timeoutInSeconds time.Duration) ([]byte, error) {
	module.initQueue(k)
	if timeoutInSeconds > 0 {
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(timeoutInSeconds)
			timeout <- true
		}()
		select {
		case b := <-(*queues[k]).ReadChan():
			return b, nil
		case <-timeout:
			return nil, errors.New("time out")
		}
	} else {
		b := <-(*queues[k]).ReadChan()
		return b, nil
	}
}

func (module DiskQueue) Close(k string) error {
	b := (*queues[k]).Close()
	return b
}

func (module DiskQueue) Depth(k string) int64 {
	module.initQueue(k)
	b := (*queues[k]).Depth()
	return b
}

func (module DiskQueue) GetQueues() []string {
	result := []string{}
	for k := range queues {
		result = append(result, k)
	}
	return result
}

func (module DiskQueue) Start() error {
	queue.Register("disk", diskQueue)
	return nil
}

func (module DiskQueue) Stop() error {
	for _, v := range queues {
		err := (*v).Close()
		if err != nil {
			log.Debug(err)
		}
	}
	return nil
}

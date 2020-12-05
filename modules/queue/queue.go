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



	readBuffSize := cfg.readChanBuffer
	syncTime := time.Duration(cfg.syncTimeoutInSeconds) * time.Second
	var syncEvery =cfg.syncEveryInSeconds
	var maxPerFile = cfg.maxBytesPerFile
	var minMsgSize int = cfg.minMsgSize
	var maxMsgSize int = cfg.maxMsgSize

	q := NewDiskQueue(strings.ToLower(channel), dataPath, maxPerFile, int32(minMsgSize), int32(maxMsgSize), syncEvery, syncTime, readBuffSize)
	queues[name] = &q

	return nil
}

func (module DiskQueue) Setup(cfg *config.Config) {
	queues = make(map[string]*BackendQueue)
	queue.Register("disk", module)
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

//min_msg_size: 1
//max_msg_size: 500000000 #500,000,000
//max_bytes_per_file: 50*1024*1024*1024
//sync_every_in_seconds: 10
//sync_timeout_in_seconds: 10
//read_chan_buffer: 0
type QueueConfig struct {
	minMsgSize int
	maxMsgSize int
	maxBytesPerFile int64
	syncEveryInSeconds int64
	syncTimeoutInSeconds int
	readChanBuffer int
}
var cfg *QueueConfig
func (module DiskQueue) Start() error {
	cfg=&QueueConfig{
		minMsgSize: 1,
		maxMsgSize: 500000000,
		maxBytesPerFile: 50*1024*1024*1024,
		syncEveryInSeconds: 10,
		syncTimeoutInSeconds: 10,
		readChanBuffer: 0,
	}
	env.ParseConfig("queue", cfg)

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

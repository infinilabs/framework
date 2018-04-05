package queue

import (
	"errors"
	log "github.com/cihub/seelog"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/global"
	. "github.com/infinitbyte/framework/core/queue"
	. "github.com/infinitbyte/framework/modules/queue/disk_queue"
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

//func initQueue(name string,channel string,syncEvery int64,syncTimeoutInSeconds time.Duration)error  {
func initQueue(name string) error {

	channel := "default"

	if queues[name] != nil {
		return nil
	}

	initLocker.Lock()
	defer initLocker.Unlock()

	log.Debugf("init queue,%s", name)

	//double check after lock in
	if queues[name] != nil {
		return nil
	}

	path := path.Join(global.Env().SystemConfig.GetWorkingDir(), "queue", strings.ToLower(name))
	os.MkdirAll(path, 0777)

	readBuffSize := 0
	syncTimeout := 5 * time.Second
	var syncEvery int64 = 2500

	//TODO parameter
	q := NewDiskQueue(strings.ToLower(channel), path, 100*1024*1024, 1, 1<<25, syncEvery, syncTimeout, readBuffSize)
	queues[name] = &q

	return nil
}

func (module DiskQueue) Start(cfg *Config) {
	queues = make(map[string]*BackendQueue)

	//TODO clean up
	//pendingUpdateDiskQueue := NewDiskQueue("pending_update", path, 100*1024*1024, 1, 1<<20, syncEvery, syncTimeout, readBuffSize)
	//pendingCheckDiskQueue := NewDiskQueue("pending_check", path, 100*1024*1024, 1, 1<<20, syncEvery, syncTimeout, readBuffSize)
	//pendingDispatchDiskQueue := NewDiskQueue("pending_dispatch", path, 100*1024*1024, 1, 1<<20, syncEvery, syncTimeout, readBuffSize)
	//pendingIndexDiskQueue := NewDiskQueue("pending_index", path, 100*1024*1024, 1, 1<<25, syncEvery, syncTimeout, readBuffSize)
	//queues[config.FetchChannel] = &pendingFetchDiskQueue
	//queues[config.UpdateChannel] = &pendingUpdateDiskQueue
	//queues[config.CheckChannel] = &pendingCheckDiskQueue
	//queues[config.DispatcherChannel] = &pendingDispatchDiskQueue
	//queues[config.IndexChannel] = &pendingIndexDiskQueue
	//TODO configurable
	Register(module)
}

func (module DiskQueue) Push(k string, v []byte) error {
	initQueue(k)
	return (*queues[k]).Put(v)
}

func (module DiskQueue) ReadChan(k string) chan []byte {
	initQueue(k)
	return (*queues[k]).ReadChan()
}

func (module DiskQueue) Pop(k string, timeoutInSeconds time.Duration) (error, []byte) {
	initQueue(k)
	if timeoutInSeconds > 0 {
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(timeoutInSeconds) // sleep 3 second
			timeout <- true
		}()
		select {
		case b := <-(*queues[k]).ReadChan():
			return nil, b
		case <-timeout:
			return errors.New("time out"), nil
		}
	} else {
		b := <-(*queues[k]).ReadChan()
		return nil, b
	}
}

func (module DiskQueue) Close(k string) error {
	b := (*queues[k]).Close()
	return b
}

func (module DiskQueue) Depth(k string) int64 {
	initQueue(k)
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

func (module DiskQueue) Stop() error {
	for _, v := range queues {
		err := (*v).Close()
		if err != nil {
			log.Debug(err)
		}
	}
	return nil
}

package queue

import (
	"errors"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/queue"
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

func initQueue(name string) error {

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

	log.Debugf("init queue,%s", name)

	dataPath := path.Join(global.Env().GetWorkingDir(), "queue", strings.ToLower(name))
	os.MkdirAll(dataPath, 0777)

	readBuffSize := 0
	syncTimeout := 5 * time.Second
	var syncEvery int64 = 2500

	//TODO parameter
	q := NewDiskQueue(strings.ToLower(channel), dataPath, 100*1024*1024, 1, 1<<25, syncEvery, syncTimeout, readBuffSize)
	queues[name] = &q

	return nil
}

func (module DiskQueue) Setup(cfg *config.Config) {
	queues = make(map[string]*BackendQueue)
	queue.Register("disk", module)
}

func (module DiskQueue) Push(k string, v []byte) error {
	initQueue(k)
	return (*queues[k]).Put(v)
}

func (module DiskQueue) ReadChan(k string) chan []byte {
	initQueue(k)
	return (*queues[k]).ReadChan()
}

func (module DiskQueue) Pop(k string, timeoutInSeconds time.Duration) ([]byte, error) {
	initQueue(k)
	if timeoutInSeconds > 0 {
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(timeoutInSeconds) // sleep 3 second
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

func (module DiskQueue) Start() error {
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

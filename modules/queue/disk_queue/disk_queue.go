package queue

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
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

//#  disk.max_used_bytes:  100GB #trigger warning message
//#  disk.warning_free_bytes:  20GB #trigger warning message
//#  disk.reserved_free_bytes: 10GB #enter readonly mode, no writes allowed
type DiskQueueConfig struct {
	MinMsgSize       int32   `config:"min_msg_size"`
	MaxMsgSize       int32   `config:"max_msg_size"`
	MaxBytesPerFile  int64 `config:"max_bytes_per_file"`
	SyncEveryRecords int64 `config:"sync_every_records"`
	SyncTimeoutInMS  int   `config:"sync_timeout_in_ms"`
	ReadChanBuffer   int   `config:"read_chan_buffer_size"`
	WriteChanBuffer   int   `config:"write_chan_buffer_size"`

	MaxUsedBytes   uint64   `config:"max_used_bytes"`
	WarningFreeBytes   uint64   `config:"warning_free_bytes"`
	ReservedFreeBytes   uint64   `config:"reserved_free_bytes"`

	UploadToS3   bool   `config:"upload_to_s3"`

	S3 struct{
		Sync   bool   `config:"sync"`
		Server   string   `config:"server"`
		Location   string   `config:"location"`
		Bucket   string   `config:"bucket"`
	}`config:"s3"`

}

func (module *DiskQueue) Init(name string) error {
	module.initLocker.Lock()
	defer module.initLocker.Unlock()

	_,ok:= module.queues.Load(name)
	if ok{
		return nil
	}

	log.Debugf("init queue: %s", name)

	dataPath := path.Join(global.Env().GetDataDir(), "queue", strings.ToLower(name))

	os.MkdirAll(dataPath, 0755)

	tempQueue := NewDiskQueueByConfig(name,dataPath,module.cfg)

	module.queues.Store(name,&tempQueue)

	return nil
}

func (module *DiskQueue) Setup(config *config.Config) {

	module.cfg = &DiskQueueConfig{
		UploadToS3:       false,
		MinMsgSize:       1,
		MaxMsgSize:       104857600, //100MB
		MaxBytesPerFile:  1 * 1024 * 1024 * 1024, //1GB
		SyncEveryRecords: 1000,
		SyncTimeoutInMS:  1000,
		ReadChanBuffer:   0,
		WriteChanBuffer:   0,
		WarningFreeBytes: 10 * 1024 * 1024 * 1024,
		ReservedFreeBytes: 5 * 1024 * 1024 * 1024,
	}

	ok,err:=env.ParseConfig("disk_queue", module.cfg)
	if ok&&err!=nil{
		panic(err)
	}

	module.queues=sync.Map{}
	//load from metadata
	//load from directory




	module.RegisterAPI()
}

func (module *DiskQueue) Push(k string, v []byte) error {
	module.Init(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Put(v)
	}
	return errors.Errorf("queue [%v] not found",k)
}

func (module *DiskQueue) ReadChan(k string) <-chan []byte{
	module.Init(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).ReadChan()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {
	err:= module.Init(k)
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

func (module *DiskQueue) Consume(queueName,consumer,offsetStr string,count int, timeDuration time.Duration) (ctx *queue.Context,messages []queue.Message,timeout bool,err error) {

	module.Init(queueName)
	q,ok:=module.queues.Load(queueName)
	if ok{
		data:=strings.Split(offsetStr,",")
		if len(data)!=2{
			panic(errors.Errorf("invalid offset: %v",offsetStr))
		}
		var part,offset int64
		part,_=util.ToInt64(data[0])
		offset,_=util.ToInt64(data[1])
		q1:=(*q.(*BackendQueue))
		ctx,messages,timeout,err:=q1.Consume(consumer,part,offset,count, timeDuration)
		return ctx,messages,timeout,err
	}

	panic(errors.Errorf("queue [%v] not found",queueName))
}

func (module *DiskQueue) Close(k string) error {
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).Close()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) LatestOffset(k string) string {
	module.Init(k)
	q,ok:=module.queues.Load(k)
	if ok{
		return (*q.(*BackendQueue)).LatestOffset()
	}

	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Depth(k string) int64 {
	module.Init(k)
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

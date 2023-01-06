/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/status"
	"infini.sh/framework/modules/queue/common"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type DiskQueue struct {
	cfg        *DiskQueueConfig
	initLocker sync.Mutex
	api.Handler
	queues             sync.Map
	messages           chan Event
}

func (module *DiskQueue) Name() string {
	return "disk_queue"
}

type RetentionConfig struct {
	MaxNumOfLocalFiles int64 `config:"max_num_of_local_files"`
	//DeleteAfterSaveToS3 bool `config:"delete_after_save_to_s3"`
	//MaxAge int  	   `config:"max_age"`
}

//#  disk.max_used_bytes:  100GB #trigger warning message
//#  disk.warning_free_bytes:  20GB #trigger warning message
//#  disk.reserved_free_bytes: 10GB #enter readonly mode, no writes allowed

type DiskQueueConfig struct {
	MinMsgSize       int32 `config:"min_msg_size"`
	MaxMsgSize       int32 `config:"max_msg_size"`
	MaxBytesPerFile  int64 `config:"max_bytes_per_file"`
	SyncEveryRecords int64 `config:"sync_every_records"`
	SyncTimeoutInMS  int   `config:"sync_timeout_in_ms"`
	NotifyChanBuffer int   `config:"notify_chan_buffer_size"`
	ReadChanBuffer   int   `config:"read_chan_buffer_size"`
	WriteChanBuffer  int   `config:"write_chan_buffer_size"`

	MaxUsedBytes      uint64 `config:"max_used_bytes"`
	WarningFreeBytes  uint64 `config:"warning_free_bytes"`
	ReservedFreeBytes uint64 `config:"reserved_free_bytes"`

	UploadToS3 		  bool   `config:"upload_to_s3"`
	AlwaysDownload    bool   `config:"always_download"`

	PrepareFilesToRead bool `config:"prepare_files_to_read"`

	//default queue adaptor
	Default bool `config:"default"`
	Enabled bool `config:"enabled"`


	SkipZeroConsumers bool `config:"skip_zero_consumers"`

	Compress DiskCompress `config:"compress"`

	Retention RetentionConfig `config:"retention"`

	S3 struct {
		Async    bool   `config:"async"`
		Server   string `config:"server"`
		Location string `config:"location"`
		Bucket   string `config:"bucket"`
	} `config:"s3"`
}

type DiskCompress struct {
	DeleteAfterCompress  						bool `config:"delete_after_compress"`
	Message       CompressConfig `config:"message"`
	Segment       CompressConfig `config:"segment"`
	IdleThreshold int64            				`config:"idle_threshold"`
	NumOfFilesDecompressAhead int64             `config:"num_of_files_decompress_ahead"`
}

type CompressConfig struct {
	Enabled bool `config:"enabled"`
	Level   int  `config:"level"`
}

var preventRead bool

func checkCapacity(cfg *DiskQueueConfig) error {
	if cfg.WarningFreeBytes > 0 || cfg.MaxUsedBytes > 0 || cfg.ReservedFreeBytes > 0 {
		stats := status.DiskUsage(global.Env().GetDataDir())
		if cfg.MaxUsedBytes > 0 && stats.Used >= cfg.MaxUsedBytes {
			preventRead = true
			return errors.Errorf("disk usage [%v] > threshold [%v]", util.ByteSize(stats.Used), util.ByteSize(cfg.MaxUsedBytes))
		} else if cfg.ReservedFreeBytes > 0 && stats.Free <= uint64(cfg.ReservedFreeBytes) {
			preventRead = true
			return errors.Errorf("disk free space [%v] < threshold [%v]", util.ByteSize(stats.Free), util.ByteSize(cfg.ReservedFreeBytes))
		} else if cfg.WarningFreeBytes > 0 && stats.Free <= uint64(cfg.WarningFreeBytes) {
			if rate.GetRateLimiterPerSecond("queue", "disk_full_warning", 1).Allow() {
				log.Warnf("disk free space [%v] < threshold [%v]", util.ByteSize(stats.Free), util.ByteSize(cfg.WarningFreeBytes))
			}
		}
		if preventRead {
			preventRead = false
		}
	}
	return nil
}

func (module *DiskQueue) Init(name string) error {
	module.initLocker.Lock()
	defer module.initLocker.Unlock()

	_, ok := module.queues.Load(name)
	if ok {
		return nil
	}

	log.Tracef("init queue: %s", name)

	dataPath := GetDataPath(name)

	if !util.FileExists(dataPath) {
		os.MkdirAll(dataPath, 0755)
	}

	tempQueue := NewDiskQueueByConfig(name, dataPath, module.cfg)

	module.queues.Store(name, &tempQueue)

	module.compressFiles(name, tempQueue.ReadContext().WriteFileNum)
	module.deleteUnusedFiles(name, tempQueue.ReadContext().WriteFileNum)
	return nil
}

func GetDataPath(queueID string) string {
	return path.Join(global.Env().GetDataDir(), "queue", strings.ToLower(queueID))
}

func GetFileName(queueID string, segmentID int64) string {
	return path.Join(GetDataPath(queueID), fmt.Sprintf("%09d.dat", segmentID))
}

func (module *DiskQueue) Setup() {

	module.cfg = &DiskQueueConfig{
		Enabled:           true,
		Default:           true,
		UploadToS3:        false,
		Retention:         RetentionConfig{MaxNumOfLocalFiles: 10},
		MinMsgSize:        1,
		MaxMsgSize:        104857600,         //100MB
		MaxBytesPerFile:   100 * 1024 * 1024, //100MB
		SyncEveryRecords:  1000,
		SyncTimeoutInMS:   1000,
		NotifyChanBuffer:  100,
		ReadChanBuffer:    0,
		WriteChanBuffer:   0,
		WarningFreeBytes:  10 * 1024 * 1024 * 1024,
		ReservedFreeBytes: 5 * 1024 * 1024 * 1024,
		PrepareFilesToRead: true,
		Compress: DiskCompress{
			IdleThreshold: 5,
			NumOfFilesDecompressAhead: 3,
			Message: CompressConfig{
			Enabled: false,
			Level:   3,
		}, Segment: CompressConfig{
			Enabled: true,
			Level:   11,
		}},
	}

	ok, err := env.ParseConfig("disk_queue", module.cfg)
	if ok && err != nil {
		panic(err)
	}

	if !module.cfg.Enabled {
		return
	}

	common.InitQueueMetadata()

	module.queues = sync.Map{}

	module.messages = make(chan Event, module.cfg.NotifyChanBuffer)

	RegisterEventListener(func(event Event) error {

		module.messages <- event

		return nil
	})

	////register consumer listener
	//queue.RegisterConsumerConfigChangeListener(func(queueID string,configs map[string]*queue.ConsumerConfig) {
	//	persistConsumerMetadata(queueID,configs)
	//})

	queue.Register("disk", module)
	if module.cfg.Default {
		queue.RegisterDefaultHandler(module)
	}

	module.RegisterAPI()

}

func (module *DiskQueue) Push(k string, v []byte) error {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		return (*q.(*BackendQueue)).Put(v)
	}
	return errors.Errorf("queue [%v] not found", k)
}

func (module *DiskQueue) ReadChan(k string) <-chan []byte {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		return (*q.(*BackendQueue)).ReadChan()
	}
	panic(errors.Errorf("queue [%v] not found", k))
}

func (module *DiskQueue) Pop(k string, timeoutDuration time.Duration) (data []byte, timeout bool) {
	if timeoutDuration > 0 {
		to := util.AcquireTimer(timeoutDuration)
		defer util.ReleaseTimer(to)
		for {
			to.Reset(timeoutDuration)
			select {
			case b := <-module.ReadChan(k):
				return b, false
			case <-to.C:
				return nil, true
			}
		}
	} else {
		b := <-module.ReadChan(k)
		return b, false
	}
}

func (module *DiskQueue) AcquireConsumer(qconfig *queue.QueueConfig,consumer *queue.ConsumerConfig, segment, offset int64) (queue.ConsumerAPI,error){
	q, ok := module.queues.Load(qconfig.Id)
	if !ok {
		//try init
		module.Init(qconfig.Id)
		q, ok = module.queues.Load(qconfig.Id)
	}
	if ok {
		q1 := (*q.(*BackendQueue))
		return q1.AcquireConsumer(consumer,segment,offset)
	}
	panic(errors.Errorf("queue [%v] not found", qconfig.Name))
}

func (module *DiskQueue) Consume(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig, offsetStr string) (ctx *queue.Context, messages []queue.Message, timeout bool, err error) {

	q, ok := module.queues.Load(qconfig.Id)
	if !ok {
		//try init
		module.Init(qconfig.Id)
		q, ok = module.queues.Load(qconfig.Id)
	}
	if ok {
		segment, offset := queue.ConvertOffset(offsetStr)
		q1 := (*q.(*BackendQueue))
		ctx, messages, timeout, err := q1.Consume(consumer, segment, offset)

		if global.Env().IsDebug {
			log.Tracef("[%v] consumer [%v] [%v,%v] %v, fetched:%v, timeout:%v,next:%v, err:%v", qconfig.Name, consumer, segment, offset, consumer.FetchMaxMessages, len(messages), timeout, ctx.NextOffset, err)
		}

		return ctx, messages, timeout, err
	}

	panic(errors.Errorf("queue [%v] not found", qconfig.Name))
}

func (module *DiskQueue) Close(k string) error {
	q, ok := module.queues.Load(k)
	if ok {
		return (*q.(*BackendQueue)).Close()
	}
	panic(errors.Errorf("queue [%v] not found", k))
}

func (module *DiskQueue) GetStorageSize(k string) uint64 {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		ctx := (*q.(*BackendQueue)).ReadContext()
		folder := filepath.Dir(ctx.WriteFile)
		size, _ := status.DirSize(folder)
		return size
	}
	panic(errors.Errorf("queue [%v] not found", k))
}

func (module *DiskQueue) LatestOffset(k string) string {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		return (*q.(*BackendQueue)).LatestOffset()
	}

	panic(errors.Errorf("queue [%v] not found", k))
}

func (module *DiskQueue) Depth(k string) int64 {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		return (*q.(*BackendQueue)).Depth()
	}

	panic(errors.Errorf("queue [%v] not found", k))
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
	if !module.cfg.Enabled {
		return nil
	}

	//TODO move to dedicated queue module

	//load configs from local file
	cfgs := queue.GetAllConfigs()

	if cfgs != nil && len(cfgs) > 0 {
		for _, v := range cfgs {
			if v.Id == "" {
				v.Id = v.Name
			}
			if v.Type != "" && v.Type != "disk" {
				continue
			}

			if v.Type == "" && !module.cfg.Default {
				continue
			}

			queue.IniQueue(v)
		}
	}

	//trigger s3 uploading
	//from lastUpload to current WrtieFile
	if module.cfg.UploadToS3 {
		//TODO, support cancel and safety shutdown
		go func() {
			defer func() {
				if !global.Env().IsDebug {
					if r := recover(); r != nil {
						var v string
						switch r.(type) {
						case error:
							v = r.(error).Error()
						case runtime.Error:
							v = r.(runtime.Error).Error()
						case string:
							v = r.(string)
						}
						log.Errorf("error in disk_module [%v]", v)
					}
				}
			}()

			for _, v := range cfgs {
				last := GetLastS3UploadFileNum(v.Id)
				log.Trace("last upload:",v.Id,",",last)
				offsetStr := queue.LatestOffset(v)
				segment, _ := queue.ConvertOffset(offsetStr)
				log.Tracef("check offset %v/%v/%v,%v, last upload:%v", v.Name, v.Id, offsetStr, segment, last)
				if segment > last {
					for x := last; x < segment; x++ {
						if x >= 0 {
							if global.Env().IsDebug {
								log.Tracef("try to upload %v/%v", v.Id, x)
							}
							module.uploadToS3(v.Id, x)
						}
					}
				}
			}
		}()
	}

	go func() {
		defer func() {
			if !global.Env().IsDebug {
				if r := recover(); r != nil {
					var v string
					switch r.(type) {
					case error:
						v = r.(error).Error()
					case runtime.Error:
						v = r.(runtime.Error).Error()
					case string:
						v = r.(string)
					}
					log.Errorf("error in disk_module [%v]", v)
				}
			}
		}()
		var lastFilePrepared int64
		for {
			evt := <-module.messages
			switch evt.Type {
			case WriteComplete:
				//TODO, convert to signal, move to async
				module.compressFiles(evt.Queue,evt.FileNum)

				//upload old file to s3
				module.uploadToS3(evt.Queue, evt.FileNum)

				//check capacity
				checkCapacity(module.cfg)

				//delete old unused files
				module.deleteUnusedFiles(evt.Queue, evt.FileNum)

				break
			case ReadComplete:

				if module.cfg.PrepareFilesToRead{
					if lastFilePrepared>0 && evt.FileNum<=lastFilePrepared{
						break
					}
					//decompress ahead of # files
					lastFilePrepared=module.prepareFilesToRead(evt.Queue,evt.FileNum)
				}

				//delete old unused files
				module.deleteUnusedFiles(evt.Queue, evt.FileNum)

				break
			}
		}

	}()

	return nil
}

func (module *DiskQueue) Stop() error {

	if module.cfg==nil{
		return nil
	}

	if !module.cfg.Enabled {
		return nil
	}

	close(module.messages)
	module.queues.Range(func(key, value interface{}) bool {
		q, ok := module.queues.Load(key)
		if ok {
			err := (*q.(*BackendQueue)).Close()
			if err != nil {
				log.Error(err)
			}
		}
		return true
	})

	common.PersistQueueMetadata()
	return nil
}

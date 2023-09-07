/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/modules/queue/common"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/status"
)

type DiskQueue struct {
	cfg        *DiskQueueConfig
	initLocker sync.Mutex
	queues     sync.Map
	messages   chan Event
	cfgs       map[string]*queue.QueueConfig
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

	EOFRetryDelayInMs int64 `config:"eof_retry_delay_in_ms" json:"eof_retry_delay_in_ms,omitempty"`

	MaxUsedBytes      uint64 `config:"max_used_bytes"`
	WarningFreeBytes  uint64 `config:"warning_free_bytes"`
	ReservedFreeBytes uint64 `config:"reserved_free_bytes"`

	AutoSkipCorruptFile bool `config:"auto_skip_corrupted_file"`

	UploadToS3     bool `config:"upload_to_s3"`
	AlwaysDownload bool `config:"always_download"`

	PrepareFilesToRead bool `config:"prepare_files_to_read"`

	CompressAndCleanupDuringInit bool `config:"cleanup_files_on_init"`

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
	DeleteAfterCompress       bool           `config:"delete_after_compress"`
	Message                   CompressConfig `config:"message"`
	Segment                   CompressConfig `config:"segment"`
	IdleThreshold             int64          `config:"idle_threshold"`
	NumOfFilesDecompressAhead int64          `config:"num_of_files_decompress_ahead"`
}

type CompressConfig struct {
	Enabled bool `config:"enabled"`
	Level   int  `config:"level"`
}

var preventRead bool

func checkCapacity(cfg *DiskQueueConfig) error {
	if cfg.WarningFreeBytes > 0 || cfg.MaxUsedBytes > 0 || cfg.ReservedFreeBytes > 0 {
		pathUsed,err := status.DirSize(global.Env().GetDataDir())
		if err!=nil{
			panic(err)
		}

		partitonStats := status.DiskPartitionUsage(global.Env().GetDataDir())
		if cfg.MaxUsedBytes > 0 && pathUsed >= cfg.MaxUsedBytes {
			preventRead = true
			return errors.Errorf("disk usage [%v] > threshold [%v]", util.ByteSize(pathUsed), util.ByteSize(cfg.MaxUsedBytes))
		} else if cfg.ReservedFreeBytes > 0 && partitonStats.Free <= uint64(cfg.ReservedFreeBytes) {
			preventRead = true
			return errors.Errorf("disk free space [%v] < threshold [%v]", util.ByteSize(partitonStats.Free), util.ByteSize(cfg.ReservedFreeBytes))
		} else if cfg.WarningFreeBytes > 0 && partitonStats.Free <= uint64(cfg.WarningFreeBytes) {
			if rate.GetRateLimiterPerSecond("queue", "disk_full_warning", 1).Allow() {
				log.Warnf("disk free space [%v] < threshold [%v]", util.ByteSize(partitonStats.Free), util.ByteSize(cfg.WarningFreeBytes))
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

	module.queues.Store(name, tempQueue)

	if module.cfg.CompressAndCleanupDuringInit {
		module.compressFiles(name, tempQueue.ReadContext().WriteFileNum)
		module.deleteUnusedFiles(name, tempQueue.ReadContext().WriteFileNum)
	}

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
		Enabled:             true,
		Default:             true,
		AutoSkipCorruptFile: true,
		UploadToS3:          false,
		Retention:           RetentionConfig{MaxNumOfLocalFiles: 5},
		MinMsgSize:          1,
		MaxMsgSize:          104857600,         //100MB
		MaxBytesPerFile:     100 * 1024 * 1024, //100MB
		EOFRetryDelayInMs:   500,
		SyncEveryRecords:    1000,
		SyncTimeoutInMS:     1000,
		NotifyChanBuffer:    100,
		ReadChanBuffer:      0,
		WriteChanBuffer:     0,
		WarningFreeBytes:    10 * 1024 * 1024 * 1024,
		ReservedFreeBytes:   5 * 1024 * 1024 * 1024,
		PrepareFilesToRead:  true,
		Compress: DiskCompress{
			IdleThreshold:             3,
			DeleteAfterCompress:       true,
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

	//load configs from local metadata
	if util.FileExists(common.GetLocalQueueConfigPath()) {
		data, err := util.FileGetContent(common.GetLocalQueueConfigPath())
		if err != nil {
			panic(err)
		}

		cfgs := map[string]*queue.QueueConfig{}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			panic(err)
		}
		module.cfgs = cfgs
	}

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

}

func (module *DiskQueue) Destroy(k string) error {
	q, ok := module.queues.Load(k)
	if !ok {
		return nil
	}
	err := (q.(*DiskBasedQueue)).Destroy()
	if err != nil {
		return err
	}
	module.queues.Delete(k)
	return nil
}

func (module *DiskQueue) Push(k string, v []byte) error {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {

		msgSize := len(v)
		if int32(msgSize) < module.cfg.MinMsgSize || int32(msgSize) > module.cfg.MaxMsgSize {
			return errors.Errorf("queue:%v, invalid message size: %v, should between: %v TO %v", k, msgSize, module.cfg.MinMsgSize, module.cfg.MaxMsgSize)
		}

		res:= (q.(*DiskBasedQueue)).Put(v)
		return res.Error
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
		return (q.(*DiskBasedQueue)).ReadChan()
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

func (this *DiskQueue) ReleaseConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig,instance queue.ConsumerAPI) error {
	return instance.Close()
}

func (module *DiskQueue) AcquireConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig) (queue.ConsumerAPI, error) {
	offsetStr, _ := queue.GetOffset(qconfig, consumer)
	q, ok := module.queues.Load(qconfig.ID)
	if !ok {
		//try init
		module.Init(qconfig.ID)
		q, ok = module.queues.Load(qconfig.ID)
	}
	if ok {
		q1 := q.(*DiskBasedQueue)
		return q1.AcquireConsumer(qconfig, consumer, offsetStr.Segment, offsetStr.Position)
	}
	panic(errors.Errorf("queue [%v] not found", qconfig.Name))
}

func (module *DiskQueue) Close(k string) error {
	q, ok := module.queues.Load(k)
	if ok {
		return (q.(*DiskBasedQueue)).Close()
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
		ctx := (q.(*DiskBasedQueue)).ReadContext()
		folder := filepath.Dir(ctx.WriteFile)
		size, _ := status.DirSize(folder)
		return size
	}
	panic(errors.Errorf("queue [%v] not found", k))
}

func (module *DiskQueue) LatestOffset(k *queue.QueueConfig) queue.Offset {
	q, ok := module.queues.Load(k.ID)
	if !ok {
		//try init
		module.Init(k.ID)
		q, ok = module.queues.Load(k.ID)
	}
	if ok {
		return (q.(*DiskBasedQueue)).LatestOffset()
	}

	panic(errors.Errorf("queue [%v] not found", k.ID))
}

func (module *DiskQueue) Depth(k string) int64 {
	q, ok := module.queues.Load(k)
	if !ok {
		//try init
		module.Init(k)
		q, ok = module.queues.Load(k)
	}
	if ok {
		return (q.(*DiskBasedQueue)).Depth()
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
	if module.cfg != nil && !module.cfg.Enabled {
		return nil
	}

	//load configs from local file
	if module.cfgs != nil {
		for _, v := range module.cfgs {
			if v.ID == "" {
				v.ID = v.Name
			}
			if v.Type != "" && v.Type != "disk" {
				continue
			}

			if v.Type == "" && !module.cfg.Default {
				continue
			}
			queue.IniQueue(v)
			queue.RegisterConfig(v)
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

			for _, v := range module.cfgs {
				last := GetLastS3UploadFileNum(v.ID)
				log.Trace("last upload:", v.ID, ",", last)
				offsetStr := queue.LatestOffset(v)
				log.Tracef("check offset %v/%v/%v,%v, last upload:%v", v.Name, v.ID, offsetStr, offsetStr.Segment, last)
				if offsetStr.Segment > last {
					for x := last; x < offsetStr.Segment; x++ {
						if x >= 0 {
							if global.Env().IsDebug {
								log.Tracef("try to upload %v/%v", v.ID, x)
							}
							module.uploadToS3(v.ID, x)
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
				module.compressFiles(evt.Queue, evt.FileNum)

				//upload old file to s3
				module.uploadToS3(evt.Queue, evt.FileNum)

				//check capacity
				checkCapacity(module.cfg)

				//delete old unused files
				module.deleteUnusedFiles(evt.Queue, evt.FileNum)

				break
			case ReadComplete:

				if module.cfg.PrepareFilesToRead {
					if lastFilePrepared > 0 && evt.FileNum <= lastFilePrepared {
						break
					}
					//decompress ahead of # files
					lastFilePrepared = module.prepareFilesToRead(evt.Queue, evt.FileNum)
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

	if module.cfg != nil && module.cfg == nil {
		return nil
	}

	if !module.cfg.Enabled {
		return nil
	}

	close(module.messages)
	module.queues.Range(func(key, value interface{}) bool {
		q, ok := module.queues.Load(key)
		if ok {
			err := (q.(*DiskBasedQueue)).Close()
			if err != nil {
				log.Error(err)
			}
		}
		return true
	})
	return nil
}

const consumerOffsetBucket = "queue_consumer_commit_offset"

func getCommitKey(k *queue.QueueConfig, consumer *queue.ConsumerConfig) string {
	return fmt.Sprintf("%v-%v", k.ID, consumer.ID)
}

func (module *DiskQueue) GetOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig) (queue.Offset, error) {

	bytes, err := kv.GetValue(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)))
	if err != nil {
		log.Error(err)
	}

	if bytes != nil && len(bytes) > 0 {
		str := string(bytes)
		off := queue.NewOffsetFromStr(str)
		return off, nil
	}

	return queue.NewOffset(0, 0), nil
}

func (module *DiskQueue) DeleteOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig) error {
	return kv.DeleteKey(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)))
}

func (module *DiskQueue) CommitOffset(k *queue.QueueConfig, consumer *queue.ConsumerConfig, offset queue.Offset) (bool, error) {

	if global.Env().IsDebug {
		log.Tracef("queue [%v] [%v][%v] commit offset:%v", k.ID, consumer.Group, consumer.Name, offset)
	}

	err := kv.AddValue(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)), []byte(offset.String()))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (module *DiskQueue) AcquireProducer(cfg *queue.QueueConfig) (queue.ProducerAPI, error) {

	if cfg == nil || cfg.ID == "" {
		panic("queue config is nil")
	}

	q, ok := module.queues.Load(cfg.ID)
	if !ok {
		//try init
		err := module.Init(cfg.ID)
		if err != nil {
			panic(err)
		}
		q, ok = module.queues.Load(cfg.ID)
	}

	if !ok {
		return nil, errors.Errorf("queue:%v not found", cfg.ID)
	}

	producer := &Producer{q: q.(*DiskBasedQueue), cfg: cfg, diskQueueConfig:module.cfg}
	return producer, nil
}


func (this *DiskQueue)  ReleaseProducer(k *queue.QueueConfig,producer queue.ProducerAPI) error{
	return nil
}
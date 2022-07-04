/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/status"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type DiskQueue struct {
	cfg *DiskQueueConfig
	initLocker sync.Mutex
	api.Handler
	queues sync.Map
	messages chan Event

}

func (module *DiskQueue) Name() string {
	return "disk_queue"
}

type RetentionConfig struct{
		MaxNumOfLocalFiles int64  `config:"max_num_of_local_files"`
		//DeleteAfterSaveToS3 bool `config:"delete_after_save_to_s3"`
		//MaxAge int  	   `config:"max_age"`
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
	NotifyChanBuffer   int   `config:"notify_chan_buffer_size"`
	ReadChanBuffer   int   `config:"read_chan_buffer_size"`
	WriteChanBuffer   int   `config:"write_chan_buffer_size"`

	MaxUsedBytes   uint64   `config:"max_used_bytes"`
	WarningFreeBytes   uint64   `config:"warning_free_bytes"`
	ReservedFreeBytes   uint64   `config:"reserved_free_bytes"`


	UploadToS3   bool   `config:"upload_to_s3"`

	//default queue adaptor
	Default bool `config:"default"`
	Enabled bool `config:"enabled"`

	CompressOnSegment        CompressConfig `config:"compress_on_segment"`
	CompressOnMessagePayload CompressConfig `config:"compress_on_message_payload"`

	Retention RetentionConfig `config:"retention"`

	S3 struct{
		Async   bool   `config:"async"`
		Server   string   `config:"server"`
		Location   string   `config:"location"`
		Bucket   string   `config:"bucket"`
	}`config:"s3"`

}

type CompressConfig struct {
	Enabled bool  `config:"enabled"`
	Level   int  `config:"level"`
}

const queueS3LastFileNum ="last_success_file_for_queue"

func GetLastS3UploadFileNum(queueID string)int64  {
	b,err:=kv.GetValue(queueS3LastFileNum,util.UnsafeStringToBytes(queueID))
	if err!=nil{
		panic(err)
	}
	if b==nil||len(b)==0{
		return -1
	}
	//log.Errorf("bytes to int64: %v",b)
	return util.BytesToInt64(b)
}

func getS3FileLocation(fileName string) string {
	return path.Join(global.Env().SystemConfig.NodeConfig.ID,util.TrimLeftStr(fileName,global.Env().GetDataDir()))
}

var s3uploaderLocker sync.RWMutex
func (module *DiskQueue)uploadToS3(queueID string,fileNum  int64){
	//TODO move to channel, async
	s3uploaderLocker.Lock()
	defer s3uploaderLocker.Unlock()

	//send s3 upload signal
	if module.cfg.UploadToS3{

		consumers,_,_:=queue.GetEarlierOffsetByQueueID(queueID)
		if consumers==0{
			//skip upload queue without any consumers
			return
		}

		//skip uploaded file
		lastFileNum:= GetLastS3UploadFileNum(queueID)
		log.Tracef("last upload:%v, fileNum:%v",lastFileNum, fileNum)
		if fileNum<=lastFileNum{
			//skip old queue file, no need to upload
			return
		}

		if module.cfg.S3.Server!=""&&module.cfg.S3.Bucket!=""{
			fileName:= GetFileName(queueID,fileNum)
			toFile:= getS3FileLocation(fileName)
			var success=false
			var err error
			if module.cfg.S3.Async{
				err:=s3.AsyncUpload(fileName,module.cfg.S3.Server,module.cfg.S3.Location,module.cfg.S3.Bucket,toFile)
				if err!=nil{
					log.Error(err)
				}else{
					success=true
				}
			}else{
				var ok bool
				ok,err=s3.SyncUpload(fileName,module.cfg.S3.Server,module.cfg.S3.Location,module.cfg.S3.Bucket,toFile)
				if err!=nil{
					log.Error(err)
				}else if ok{
					success=true
				}
			}
			//update last mark
			if success{
				err=kv.AddValue(queueS3LastFileNum,util.UnsafeStringToBytes(queueID),util.Int64ToBytes(fileNum))
				if err!=nil{
					panic(err)
				}
				log.Debugf("queue [%v][%v] uploaded to s3",queueID,fileNum)
			}else{
				log.Debugf("failed to upload queue [%v][%v] to s3, %v",queueID,fileNum,err)
			}
		}else{
			log.Errorf("invalid s3 config:%v",module.cfg.S3)
		}
	}
}

func (module *DiskQueue) Init(name string) error {
	module.initLocker.Lock()
	defer module.initLocker.Unlock()

	_,ok:= module.queues.Load(name)
	if ok{
		return nil
	}

	log.Debugf("init queue: %s", name)

	dataPath := GetDataPath(name)

	if !util.FileExists(dataPath){
		os.MkdirAll(dataPath, 0755)
	}

	tempQueue := NewDiskQueueByConfig(name,dataPath,module.cfg)

	module.queues.Store(name,&tempQueue)

	module.deleteUnusedFiles(name,tempQueue.ReadContext().WriteFileNum)
	return nil
}

func GetDataPath(queueID string)string  {
	return path.Join(global.Env().GetDataDir(), "queue", strings.ToLower(queueID))
}

func getQueueConfigPath() string {
	os.MkdirAll(path.Join(global.Env().GetDataDir(),"queue"),0755)
	return path.Join(global.Env().GetDataDir(),"queue","configs")
}

func GetFileName(queueID string,segmentID int64) string {
	//return path.Join(GetDataPath(queueID),fmt.Sprintf("%s.segment.%06d.dat",queueID , segmentID))
	return path.Join(GetDataPath(queueID),fmt.Sprintf("%s.diskqueue.%06d.dat",queueID , segmentID))
}


func (module *DiskQueue) Setup(config *config.Config) {

	module.cfg = &DiskQueueConfig{
		Enabled:           true,
		Default:           true,
		UploadToS3:        false,
		Retention:         RetentionConfig{ MaxNumOfLocalFiles: 10},
		MinMsgSize:        1,
		MaxMsgSize:        104857600, //100MB
		MaxBytesPerFile:   100 * 1024 * 1024, //100MB
		SyncEveryRecords:  1000,
		SyncTimeoutInMS:   1000,
		NotifyChanBuffer:  100,
		ReadChanBuffer:    0,
		WriteChanBuffer:   0,
		WarningFreeBytes:  10 * 1024 * 1024 * 1024,
		ReservedFreeBytes: 5 * 1024 * 1024 * 1024,
		CompressOnMessagePayload: CompressConfig{
			Enabled: false,
			Level: 11,
		},
	}

	ok,err:=env.ParseConfig("disk_queue", module.cfg)
	if ok&&err!=nil{
		panic(err)
	}

	if !module.cfg.Enabled{
		return
	}

	module.queues=sync.Map{}

	//load configs from static config
	configs := []queue.Config{}
	ok, err = env.ParseConfig("queue", &configs)
	if ok && err != nil {
		panic(err)
	}

	for _,v:=range configs{
		v.Source="file"
		if v.Id==""{
			v.Id=v.Name
		}
		queue.RegisterConfig(v.Name,&v)
	}

	//load configs from local metadata
	if util.FileExists(getQueueConfigPath()){
		data,err:=util.FileGetContent(getQueueConfigPath())
		if err!=nil{
			panic(err)
		}

		cfgs:=map[string]*queue.Config{}
		err=util.FromJSONBytes(data,&cfgs)
		if err!=nil{
			panic(err)
		}

		for _,v:=range cfgs{
			if v.Id==""{
				v.Id=v.Name
			}
			log.Debugf("init config:%v, type:%v",v.Name,v.Type)
			queue.RegisterConfig(v.Name,v)
		}
	}


	//load queue information from directory

	//load configs from remote elasticsearch


	//register queue listener
	queue.RegisterQueueConfigChangeListener(func(v *queue.Config) {
		persistQueueMetadata()
	})

	module.messages = make(chan Event,module.cfg.NotifyChanBuffer)

	RegisterEventListener(func(event Event) error {

		module.messages<-event

		return nil
	})

	////register consumer listener
	//queue.RegisterConsumerConfigChangeListener(func(queueID string,configs map[string]*queue.ConsumerConfig) {
	//	persistConsumerMetadata(queueID,configs)
	//})

	queue.Register("disk", module)
	if module.cfg.Default{
		queue.RegisterDefaultHandler(module)
	}
}

func (module *DiskQueue) Push(k string, v []byte) error {
	q,ok:=module.queues.Load(k)
	if !ok{
		//try init
		module.Init(k)
		q,ok=module.queues.Load(k)
	}
	if ok{
		return (*q.(*BackendQueue)).Put(v)
	}
	return errors.Errorf("queue [%v] not found",k)
}

func (module *DiskQueue) ReadChan(k string) <-chan []byte{
	q,ok:=module.queues.Load(k)
	if !ok{
		//try init
		module.Init(k)
		q,ok=module.queues.Load(k)
	}
	if ok{
		return (*q.(*BackendQueue)).ReadChan()
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {
	if timeoutDuration > 0 {
		to := util.AcquireTimer(timeoutDuration)
		defer util.ReleaseTimer(to)
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

func ConvertOffset(offsetStr string) (int64,int64) {
	data:=strings.Split(offsetStr,",")
	if len(data)!=2{
		panic(errors.Errorf("invalid offset: %v",offsetStr))
	}
	var segment,offset int64
	segment,_=util.ToInt64(data[0])
	offset,_=util.ToInt64(data[1])
	return segment,offset
}

func (module *DiskQueue) Consume(queueName,consumer,offsetStr string,count int, timeDuration time.Duration) (ctx *queue.Context,messages []queue.Message,timeout bool,err error) {

	q,ok:=module.queues.Load(queueName)
	if !ok{
		//try init
		module.Init(queueName)
		q,ok=module.queues.Load(queueName)
	}
	if ok{
		segment, offset := ConvertOffset(offsetStr)
		q1 := (*q.(*BackendQueue))
		ctx, messages, timeout, err := q1.Consume(consumer, segment, offset, count, timeDuration)

		////no new message found
		//if len(messages) == 0 && ctx.NextOffset == ctx.InitOffset {
		//	err = errors.New("EOF")
		//}

		log.Debugf("[%v] consumer [%v] [%v,%v] %v, fetched:%v, timeout:%v,next:%v, err:%v", queueName, consumer, segment, offset, count, len(messages), timeout, ctx.NextOffset, err)

		return ctx, messages, timeout, err
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

func (module *DiskQueue) GetStorageSize(k string) uint64 {
	q,ok:=module.queues.Load(k)
	if !ok{
		//try init
		module.Init(k)
		q,ok=module.queues.Load(k)
	}
	if ok{
		ctx:= (*q.(*BackendQueue)).ReadContext()
		folder:=filepath.Dir(ctx.WriteFile)
		size,_:=status.DirSize(folder)
		return size
	}
	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) LatestOffset(k string) string {
	q,ok:=module.queues.Load(k)
	if !ok{
		//try init
		module.Init(k)
		q,ok=module.queues.Load(k)
	}
	if ok{
		return (*q.(*BackendQueue)).LatestOffset()
	}

	panic(errors.Errorf("queue [%v] not found",k))
}

func (module *DiskQueue) Depth(k string) int64 {
	q,ok:=module.queues.Load(k)
	if !ok{
		//try init
		module.Init(k)
		q,ok=module.queues.Load(k)
	}
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
	if !module.cfg.Enabled{
		return nil
	}

	//TODO move to dedicated queue module

	//load configs from local file
	cfgs:=queue.GetAllConfigs()

	if cfgs != nil && len(cfgs) > 0 {
		for _, v := range cfgs {
			if v.Id==""{
				v.Id=v.Name
			}
			if v.Type!=""&&v.Type!="disk"{
				continue
			}

			if v.Type==""&&!module.cfg.Default{
				continue
			}

			queue.IniQueue(v)
		}
	}

	module.RegisterAPI()

	//trigger s3 uploading
	//from lastUpload to current WrtieFile
	if module.cfg.UploadToS3{
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
				last:= GetLastS3UploadFileNum(v.Id)
				offsetStr:=queue.LatestOffset(v)
				segment,_:= ConvertOffset(offsetStr)
				log.Tracef("check offset %v/%v/%v,%v, last upload:%v",v.Name,v.Id,offsetStr, segment,last)
				if segment >last{
					for x:=last;x< segment;x++{
						if x>=0{
							if global.Env().IsDebug{
								log.Tracef("try to upload %v/%v",v.Id,x)
							}
							module.uploadToS3(v.Id,x)
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

		for {
			evt := <-module.messages
			switch evt.Type {
			case WriteComplete:

				//TODO, convert to signal, move to async

				//upload old file to s3
				module.uploadToS3(evt.Queue,evt.FileNum)

				//check capacity

				//delete old unused files
				module.deleteUnusedFiles(evt.Queue,evt.FileNum)

				break
			case ReadComplete:

				//delete old unused files
				module.deleteUnusedFiles(evt.Queue,evt.FileNum)

				break;
			}
		}

	}()

	return nil
}

func (module *DiskQueue) Stop() error {
	if !module.cfg.Enabled{
		return nil
	}

	close(module.messages)
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

	persistQueueMetadata()
	return nil
}

func (module *DiskQueue) deleteUnusedFiles(queueID string,fileNum  int64) {

	//no consumers or consumer/s3 already ahead of this file
	//TODO add config to configure none-consumers queue, to enable upload to s3 or not


		//check consumers offset
		consumers, segmentNum,_:=queue.GetEarlierOffsetByQueueID(queueID)
		fileStartToDelete:=fileNum-module.cfg.Retention.MaxNumOfLocalFiles

		if fileStartToDelete<=0||consumers<=0{
			return
		}

		if module.cfg.UploadToS3 {
			//check last uploaded mark
			var lastSavedFileNum= GetLastS3UploadFileNum(queueID)

			if global.Env().IsDebug{
				log.Tracef("files start to delete:%v, consumer_on:%v, last_saved:%v",fileStartToDelete,segmentNum,lastSavedFileNum)
			}

			if fileStartToDelete>= lastSavedFileNum{
				return
			}
		}

		if global.Env().IsDebug{
			log.Tracef("files start to delete:%v, consumer_on:%v",fileStartToDelete,segmentNum)
		}

		//has consumers
		if consumers>0 &&fileStartToDelete< segmentNum{
			log.Trace(queueID," start to delete:",fileStartToDelete,",consumers:",consumers,",segment:", segmentNum)
			for x:=fileStartToDelete;x>=0;x--{
				file:= GetFileName(queueID,x)
				if util.FileExists(file){
					log.Debug("delete queue file:",file)
					err:=os.Remove(file)
					if err!=nil{
						panic(err)
					}
				}else{
					//skip
					break
				}
			}
		}else{
			//FIFO queue, need to delete old files
			//log.Errorf("queue:%v, fileID:%v, file start to delete:%v , segment num:%v",queueID,fileNum,fileStartToDelete,segmentNum)
			//check current read depth and file num
		}

}

var persistentLocker sync.RWMutex
func persistQueueMetadata()  {
	persistentLocker.Lock()
	defer persistentLocker.Unlock()

	//persist configs to local store
	bytes:=queue.GetAllConfigBytes()
	_,err:=util.FilePutContentWithByte(getQueueConfigPath(),bytes)
	if err!=nil{
		panic(err)
	}
}

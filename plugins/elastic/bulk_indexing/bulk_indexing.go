package bulk_indexing

import (
	"fmt"
	"infini.sh/framework/core/locker"
	"runtime"
	"sync"
	"time"

	"github.com/OneOfOne/xxhash"
	log "github.com/cihub/seelog"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

//#操作合并任务
//写入到本地一个队列，hash 散列
//【内存、磁盘、Kafka 三种持久化选项】以分片为单位合并数据，写本地磁盘队列，一个分片一个队列
//读取分片数据，发送到所在节点上

//#bulk 发送任务
//以节点为单位，然后以主分片为单位进行流量合并发送
//一个节点一个 go 协程，用于发送数据

//#将写模式改成拉模式，由各个分片主动去拉数据
//各个分片的数据线本地压缩好，变成固定大小的包
//由各个节点所在的 agent，压缩传输过去之后，本地快速重建
//调用目标节点所在的 agent 服务，rpc 远程写磁盘数据，然后目标服务器本地读取磁盘队列。

//读各个分片的数据，写 es

// 处理 bulk 格式的数据索引。
type BulkIndexingProcessor struct {
	config               *Config
	runningConfigs       map[string]*queue.QueueConfig
	wg                   sync.WaitGroup
	inFlightQueueConfigs sync.Map
	detectorRunning      bool
	id                   string
	sync.RWMutex
	pool *pipeline.Pool

	bulkStats      *elastic.BulkResult
	statsLock      sync.Mutex
	bulkBufferPool *elastic.BulkBufferPool
}

type Config struct {
	NumOfSlices int   `config:"num_of_slices"`
	Slices      []int `config:"slices"`

	enabledSlice map[int]int

	IdleTimeoutInSecond  int `config:"idle_timeout_in_seconds"`
	MaxConnectionPerHost int `config:"max_connection_per_node"`

	QueueLabels map[string]interface{} `config:"queues,omitempty"`

	Selector queue.QueueSelector `config:"queue_selector"`

	Consumer queue.ConsumerConfig `config:"consumer"`

	MaxWorkers int `config:"max_worker_size"`

	DetectActiveQueue bool `config:"detect_active_queue"`

	VerboseBulkResult bool `config:"verbose_bulk_result"`

	SliceHashUseDocumentID bool `config:"slice_hash_use_document_id"`

	DetectIntervalInMs int `config:"detect_interval"`

	ValidateRequest   bool `config:"valid_request"`
	SkipEmptyQueue    bool `config:"skip_empty_queue"`
	SkipOnMissingInfo bool `config:"skip_info_missing"`
	LogBulkError      bool `config:"log_bulk_error"`

	BulkConfig elastic.BulkProcessorConfig `config:"bulk"`

	Elasticsearch       string                       `config:"elasticsearch,omitempty"`
	ElasticsearchConfig *elastic.ElasticsearchConfig `config:"elasticsearch_config"`

	WaitingAfter           []string `config:"waiting_after"`
	RetryDelayIntervalInMs int      `config:"retry_delay_interval"`
}

func init() {
	pipeline.RegisterProcessorPlugin("bulk_indexing", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		NumOfSlices:          1,
		MaxWorkers:           10,
		MaxConnectionPerHost: 1,
		IdleTimeoutInSecond:  5,
		DetectIntervalInMs:   5000,

		Selector: queue.QueueSelector{
			Labels: map[string]interface{}{},
		},

		Consumer: queue.ConsumerConfig{
			Group:                  "group-001",
			Name:                   "consumer-001",
			FetchMinBytes:          1,
			FetchMaxBytes:          20 * 1024 * 1024,
			FetchMaxMessages:       500,
			EOFRetryDelayInMs:      500,
			FetchMaxWaitMs:         10000,
			ConsumeTimeoutInSeconds:         60,
			EOFMaxRetryTimes:         10,
			ClientExpiredInSeconds: 60,
		},

		DetectActiveQueue:      true,
		VerboseBulkResult:      false,
		ValidateRequest:        false,
		SkipEmptyQueue:         true,
		SkipOnMissingInfo:      false,
		LogBulkError:           true,
		BulkConfig:             elastic.DefaultBulkProcessorConfig,
		RetryDelayIntervalInMs: 5000,
	}

	if err := c.Unpack(&cfg); err != nil {
		log.Error(err)
		return nil, fmt.Errorf("failed to unpack the configuration of flow_runner processor: %s", err)
	}

	if len(cfg.QueueLabels) > 0 {
		for k, v := range cfg.QueueLabels {
			cfg.Selector.Labels[k] = v
		}
	}

	if cfg.NumOfSlices <= 0 {
		cfg.NumOfSlices = 1
	}

	if len(cfg.Slices) > 0 {
		cfg.enabledSlice = map[int]int{}
		for _, v := range cfg.Slices {
			cfg.enabledSlice[v] = v
		}
	}

	runner := BulkIndexingProcessor{
		id:                   util.GetUUID(),
		config:               &cfg,
		runningConfigs:       map[string]*queue.QueueConfig{},
		inFlightQueueConfigs: sync.Map{},
	}

	runner.wg = sync.WaitGroup{}

	if runner.config.MaxWorkers < 0 {
		runner.config.MaxWorkers = 10
	}

	pool, err := pipeline.NewPoolWithTag("bulk_indexing", runner.config.MaxWorkers)
	if err != nil {
		panic(err)
	}

	runner.pool = pool

	runner.bulkBufferPool=elastic.NewBulkBufferPool("bulk_indexing_main",1024*1024*1024,100000)

	return &runner, nil
}

func (processor *BulkIndexingProcessor) Release() error {
	if processor.pool != nil {
		processor.pool.Release()
		processor.pool = nil
	}
	return nil
}

func (processor *BulkIndexingProcessor) Name() string {
	return "bulk_indexing"
}

func (processor *BulkIndexingProcessor) Process(c *pipeline.Context) error {
	processor.bulkStats = &elastic.BulkResult{}

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
				log.Error("error in bulk indexing processor,", v)
			}
		}
		log.Debug("exit bulk indexing processor")
	}()

	//handle updates
	if processor.config.DetectActiveQueue {
		if global.Env().IsDebug {
			log.Tracef("detector running [%v]", processor.detectorRunning)
		}
		if !processor.detectorRunning {
			processor.detectorRunning = true
			processor.wg.Add(1)
			go func(c *pipeline.Context) {
				if global.Env().IsDebug {
					log.Tracef("init detector for active queue [%v] ", processor.id)
				}
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
							log.Errorf("pipeline: %v, error in bulk indexing processor, %v",c.Config.Name, v)
						}
					}
					processor.detectorRunning = false
					log.Debug("exit detector for active queue")
					processor.wg.Done()
				}()

				for {

					if global.ShuttingDown() {
						break
					}

					if c.IsCanceled() {
						return
					}
					if global.Env().IsDebug {
						log.Tracef("inflight queues: %v", util.MapLength(&processor.inFlightQueueConfigs))
					}
					if global.Env().IsDebug {
						processor.inFlightQueueConfigs.Range(func(key, value interface{}) bool {
							if global.Env().IsDebug {
								log.Tracef("inflight queue:%v", key)
							}
							return true
						})
					}

					cfgs := queue.GetConfigBySelector(&processor.config.Selector)
					if global.Env().IsDebug {
						log.Tracef("get %v queues", len(cfgs))
					}
					for _, v := range cfgs {
						if c.IsCanceled() {
							return
						}
						//if have depth and not in in flight
						if !processor.config.SkipEmptyQueue || queue.HasLag(v) {
							_, ok := processor.inFlightQueueConfigs.Load(v.ID)
							if !ok {
								if global.Env().IsDebug {
									log.Tracef("detecting new queue: %v", v.Name)
								}
								processor.HandleQueueConfig(v, c)
							}
						} else {
							if global.Env().IsDebug {
								log.Tracef("skipping empty queue: %v", v.Name)
							}
						}
					}
					if processor.config.DetectIntervalInMs > 0 {
						time.Sleep(time.Millisecond * time.Duration(processor.config.DetectIntervalInMs))
					}
				}
			}(c)
		}
	} else {
		cfgs := queue.GetConfigBySelector(&processor.config.Selector)
		log.Debugf("filter queue by:%v, num of queues:%v", processor.config.Selector.ToString(), len(cfgs))
		for _, v := range cfgs {
			if global.Env().IsDebug {
				log.Tracef("checking queue: %v", v)
			}
			processor.HandleQueueConfig(v, c)
		}
	}

	processor.wg.Wait()

	return nil
}

const queueHandleSingleton = "queue_handler_singleton"

func (processor *BulkIndexingProcessor) HandleQueueConfig(v *queue.QueueConfig, parentContext *pipeline.Context) {

	//TODO, add config to enable/disable singleton, may have performance issue
	ok, _ := locker.Hold(queueHandleSingleton, v.ID, global.Env().SystemConfig.NodeConfig.ID, 60*time.Second, true)
	if !ok {
		log.Debugf("failed to hold lock for queue:[%v], already hold by somewhere", v.ID)
		return
	}

	if processor.config.SkipEmptyQueue && !queue.HasLag(v) {
		if global.Env().IsDebug {
			log.Tracef("skip empty queue:[%v]", v.Name)
		}
		return
	}

	esClusterID, meta := processor.getElasticsearchMeatadata(v)
	if meta == nil {
		log.Debugf("metadata for [%v] is nil", esClusterID)
		return
	}

	level, _ := v.Labels["level"]

	if level == "node" {
		nodeID, ok := v.Labels["node_id"]
		if ok {
			nodeInfo := meta.GetNodeInfo(util.ToString(nodeID))
			if nodeInfo != nil {
				host := nodeInfo.GetHttpPublishHost()
				processor.NewBulkWorker(parentContext, v, host)
				return
			} else {
				log.Debugf("node info not found: %v", nodeID)
			}
		} else {
			log.Debugf("node_id not found: %v", v)
		}
		if processor.config.SkipOnMissingInfo {
			return
		}
	} else if level == "shard" || level == "partition" {
		index, ok := v.Labels["index"]
		if ok {
			routingTable, err := meta.GetIndexRoutingTable(util.ToString(index))
			if err != nil {
				if rate.GetRateLimiter("error", err.Error(), 1, 1, time.Second*3).Allow() {
					log.Warn(err)
				}
				return
			}
			shard, ok := v.Labels["shard"]
			if ok {
				shards, ok := routingTable[util.ToString(shard)]
				if ok {
					for _, x := range shards {
						if x.Primary {
							//each primary shard has a goroutine, or run by one goroutine
							if x.Node != "" {
								nodeInfo := meta.GetNodeInfo(x.Node)
								if nodeInfo != nil {
									nodeHost := nodeInfo.GetHttpPublishHost()
									processor.NewBulkWorker( parentContext,  v, nodeHost)
									return
								} else {
									log.Debugf("nodeInfo not found: %v", v)
								}
							} else {
								log.Debugf("nodeID not found: %v", v)
							}
							if processor.config.SkipOnMissingInfo {
								return
							}
						}
					}
				} else {
					log.Debugf("routing table not found: %v", v)
				}
			} else {
				log.Debugf("shard not found: %v", v)
			}
		} else {
			log.Debugf("index not found: %v", v)
		}
		if processor.config.SkipOnMissingInfo {
			return
		}
	}

	host := meta.GetActiveHost()
	if global.Env().IsDebug {
		log.Tracef("random choose node [%v] to consume queue [%v]", host, v.ID)
	}
	processor.NewBulkWorker(parentContext, v, host)
}

func (processor *BulkIndexingProcessor) NewBulkWorker(parentContext *pipeline.Context, qConfig *queue.QueueConfig, preferedHost string) {

	bulkSizeInByte:=processor.config.BulkConfig.GetBulkSizeInBytes()
	//check slice
	for sliceID := 0; sliceID < processor.config.NumOfSlices; sliceID++ {

		if global.ShuttingDown() {
			break
		}

		if global.Env().IsDebug {
			log.Tracef("checking slice_id: %v", sliceID)
		}
		if len(processor.config.enabledSlice) > 0 {
			_, ok := processor.config.enabledSlice[sliceID]
			if !ok {
				log.Debugf("skipping slice_id: %v", sliceID)
				continue
			}
		}

		//queue-slice
		key := fmt.Sprintf("%v-%v", qConfig.ID, sliceID)

		if processor.config.MaxWorkers > 0 && util.MapLength(&processor.inFlightQueueConfigs) > processor.config.MaxWorkers {
			log.Debugf("reached max num of workers, skip init [%v], slice_id:%v", qConfig.Name, sliceID)
			return
		}

		processor.Lock()
		v2, exists := processor.inFlightQueueConfigs.Load(key)
		if exists {
			if global.Env().IsDebug {
				log.Tracef("[%v], queue [%v], slice_id:%v has more then one consumer, key:%v,v:%v", preferedHost, qConfig.ID, sliceID, key, v2)
			}
			processor.Unlock()
			continue
		} else {
			var workerID = util.GetUUID()
			log.Debugf("starting worker:[%v], queue:[%v], slice_id:%v, host:[%v]", workerID, qConfig.Name, sliceID, preferedHost)

			ctx1 := &pipeline.Context{}
			ctx1.Set("key", key)
			ctx1.Set("workerID", workerID)
			ctx1.Set("sliceID", sliceID)
			ctx1.Set("numOfSlices", processor.config.NumOfSlices)
			ctx1.Set("tag", preferedHost)
			ctx1.Set("qConfig", qConfig)
			ctx1.Set("host", preferedHost)
			ctx1.Set("bulkSizeInByte", bulkSizeInByte)

			err := processor.pool.Submit(&pipeline.Task{
				Handler: func(ctx *pipeline.Context, v ...interface{}) {
					key := ctx.MustGetString("key")
					workerID := ctx.MustGetString("workerID")
					host := ctx.MustGetString("host")
					sliceID := ctx.MustGetInt("sliceID")
					tag := ctx.MustGetString("tag")
					numOfSlices := ctx.MustGetInt("numOfSlices")
					bulkSizeInByte := ctx.MustGetInt("bulkSizeInByte")
					qConfig := ctx.MustGet("qConfig").(*queue.QueueConfig)
					pCtx := v[0].(*pipeline.Context)
					processor.NewSlicedBulkWorker(pCtx, key, workerID, sliceID, numOfSlices, tag, bulkSizeInByte, qConfig, host)
				},
				Context: ctx1,
				Params:  []interface{}{parentContext}, // 也可以在创建任务时设置参数
			})
			processor.Unlock()
			if err != nil {
				panic(err)
			}
		}
	}
}

var xxHashPool = sync.Pool{
	New: func() interface{} {
		return xxhash.New32()
	},
}

func (processor *BulkIndexingProcessor) getConsumerConfig(queueID,consumerName string, sliceID,maxSlices int)*queue.ConsumerConfig{
	var groupName = processor.config.Consumer.Group

	if maxSlices > 1 {
		if processor.config.Consumer.SimpleSlicedGroup {
			groupName = fmt.Sprintf("%v-%v", processor.config.Consumer.Group, sliceID)
		}else{
			groupName = fmt.Sprintf("%v-%v-%v", processor.config.Consumer.Group, queueID, sliceID)
		}
	}

	consumerConfig:= queue.GetOrInitConsumerConfig(queueID, groupName, consumerName)

	//override consumer config with processor's consumer config
	if processor.config.Consumer.EOFRetryDelayInMs > 0 {
		consumerConfig.EOFRetryDelayInMs = processor.config.Consumer.EOFRetryDelayInMs
	}
	if processor.config.Consumer.FetchMaxMessages > 0 {
		consumerConfig.FetchMaxMessages = processor.config.Consumer.FetchMaxMessages
	}
	if processor.config.Consumer.FetchMaxWaitMs > 0 {
		consumerConfig.FetchMaxWaitMs = processor.config.Consumer.FetchMaxWaitMs
	}
	if processor.config.Consumer.ConsumeTimeoutInSeconds > 0 {
		consumerConfig.ConsumeTimeoutInSeconds = processor.config.Consumer.ConsumeTimeoutInSeconds
	}
	if processor.config.Consumer.FetchMinBytes > 0 {
		consumerConfig.FetchMinBytes = processor.config.Consumer.FetchMinBytes
	}
	if processor.config.Consumer.FetchMaxBytes > 0 {
		consumerConfig.FetchMaxBytes = processor.config.Consumer.FetchMaxBytes
	}

	return consumerConfig
}

func (processor *BulkIndexingProcessor) NewSlicedBulkWorker(ctx *pipeline.Context,key, workerID string, sliceID, maxSlices int, tag string,  bulkSizeInByte int, qConfig *queue.QueueConfig, host string) {
	processor.wg.Add(1)
	processor.inFlightQueueConfigs.Store(key, workerID)

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
				log.Errorf("error in bulk indexing processor, %v, queue:%v, slice_id:%v", v, qConfig.ID, sliceID)
			}
		}
		processor.inFlightQueueConfigs.Delete(key)
		processor.wg.Done()
		if global.Env().IsDebug {
			log.Tracef("exit slice_worker, queue:%v, slice_id:%v, key:%v", qConfig.ID, sliceID, key)
		}
	}()


	if global.Env().IsDebug {
		log.Debugf("new slice_worker: %v, %v, %v, %v, %v", key, workerID, sliceID, tag, qConfig.ID)
	}

	mainBuf := processor.bulkBufferPool.AcquireBulkBuffer()
	mainBuf.Queue = qConfig.ID
	defer processor.bulkBufferPool.ReturnBulkBuffer(mainBuf)

	var bulkProcessor elastic.BulkProcessor
	var esClusterID string
	var meta *elastic.ElasticsearchMetadata
	var committedOffset *queue.Offset
	var offset *queue.Offset

	var consumerConfig =processor.getConsumerConfig(qConfig.ID, processor.config.Consumer.Name,sliceID,maxSlices)

	//try to get consumer instance
	var err error
	var consumerInstance queue.ConsumerAPI
	consumerInstance, err = queue.AcquireConsumer(qConfig, consumerConfig,workerID)
	if err != nil || consumerInstance == nil {
		if util.ContainStr(err.Error(), "already owning this topic") {
			if global.Env().IsDebug {
				log.Warnf("other consumer already owning this topic, queue:%v-%v, slice_id:%v", qConfig.Name, qConfig.ID, sliceID)
			}
			return
		}
		panic(err)
	}

	defer queue.ReleaseConsumer(qConfig, consumerConfig,consumerInstance)

	var skipFinalDocsProcess bool

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
				log.Errorf("worker[%v], queue:[%v], slice:[%v], offset:[%v]->[%v],%v", workerID, qConfig.ID, sliceID, committedOffset, offset, v)
				ctx.RecordError(fmt.Errorf("NewSlicedBulkWorker panic: %+v", r))
				skipFinalDocsProcess = true
			}
		}

		if skipFinalDocsProcess {
			ctx.CancelTask()
			return
		}

		//cleanup buffer before exit worker
		//log.Info("start final submit:",qConfig.ID,",",esClusterID,",msg count:",mainBuf.GetMessageCount(),", ",committedOffset," vs ",offset )
		if mainBuf.GetMessageCount()> 0 {
			continueNext, err := processor.submitBulkRequest(ctx, qConfig, tag, esClusterID, meta, host, bulkProcessor, mainBuf)

			if global.Env().IsDebug {
				log.Debugf("slice_worker, [%v][%v][%v][%v] submit request:%v,continue:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, mainBuf.GetMessageCount(), continueNext, err)
			}

			mainBuf.ResetData()
			if continueNext {
				if !offset.Equals(*committedOffset) {
					if consumerInstance != nil {
						if global.Env().IsDebug {
							log.Debugf("queue: %v, consumer: %v, commit offset: %v, init: %v", qConfig.ID, consumerConfig.ID, offset, committedOffset)
						}

						//log.Info("final commit:",qConfig.ID,",",esClusterID,",msg count:",mainBuf.GetMessageCount(),", ",committedOffset," vs ",offset )
						//log.Infof("final commit, queue: %v, consumer: %v, commit offset: %v, init: %v", qConfig.ID, consumerConfig.String(), offset, committedOffset)
						err := consumerInstance.CommitOffset(*offset)
						if err != nil {
							if global.Env().IsDebug {
								panic(err)
							}
						}
						//log.Infof("%v, success commit offset to: %v, previous init: %v", consumerConfig.String(),*offset,committedOffset)
						committedOffset = nil
						offset = nil
					} else {
						panic("invalid consumer instance")
					}
				}
			} else {
				if global.Env().IsDebug {
					log.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], err:%v", qConfig.ID, sliceID, committedOffset, offset, err)
				}
				panic(errors.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], bulk can't continue (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err))
			}
		}
		log.Debugf("exit worker[%v], queue:[%v], slice_id:%v", workerID, qConfig.ID, sliceID)
	}()

	if global.Env().IsDebug {
		log.Tracef("place slice_worker lock: [%v], queue [%v], slice_id:%v, key:%v,v:%v", tag, qConfig.ID, sliceID, key, workerID)
	}
	idleDuration := time.Duration(processor.config.IdleTimeoutInSecond) * time.Second
	esClusterID, meta = processor.getElasticsearchMeatadata(qConfig)
	if meta == nil {
		panic(errors.Errorf("slice_worker, cluster metadata [%v] not ready", esClusterID))
	}

	if host == "" {
		host = meta.GetActivePreferredSeedHost()
	}

	if global.Env().IsDebug {
		log.Trace("slice_worker, get host:", host, ",is dead:", elastic.IsHostDead(host), ",is available:", elastic.IsHostAvailable(host))
	}

	if elastic.IsHostDead(host) || !elastic.IsHostAvailable(host) {
		host = meta.GetActiveHost()
	}

	if global.Env().IsDebug {
		log.Trace("slice_worker, get final host:", host)
	}

	bulkProcessor = elastic.NewBulkProcessor("bulk_indexing",esClusterID,processor.config.BulkConfig)

	var lastCommit time.Time = time.Now()
	tempOffset, err := queue.GetOffset(qConfig, consumerConfig)
	if err != nil {
		panic(err)
	}
	committedOffset =&tempOffset
	//log.Infof("%v, update init offset to: %v", consumerConfig.String(),committedOffset)

	if global.Env().IsDebug {
		log.Debugf("slice_worker, get init offset: %v for consumer:%v", committedOffset, consumerConfig.Key())
	}
	offset = committedOffset
	//log.Infof("%v, update offset to: %v", consumerConfig.String(),offset)

	ctx1 := &queue.Context{}

READ_DOCS:

	//TODO
	//consumerInstance.ResetOffset(queue.ConvertOffset(offset))

	for {

		if global.ShuttingDown() {
			break
		}

		if ctx.IsCanceled() {
			goto CLEAN_BUFFER
		}

		//TODO add config to enable check or not, panic or skip
		if global.Env().IsDebug {
			log.Tracef("check host available: %v", host)
		}
		if !elastic.IsHostAvailable(host) {
			if elastic.IsHostDead(host) {
				host1 := host
				host = meta.GetActiveHost()
				if rate.GetRateLimiter("host_dead", host, 1, 1, time.Second*3).Allow() {
					if host != host1 {
						log.Infof("slice_worker, host [%v] is dead, choose: [%v]", host1, host)
					} else {
						log.Infof("slice_worker, host [%v] is dead, no other host available", host1)
					}
				}
				//if no new host found, skip processing
				if host1 == host {
					log.Debugf("slice_worker, host [%v] is dead, but still use [%v], skip and clean_buffer", host1, host)
					goto CLEAN_BUFFER
				}
			} else {
				if global.Env().IsDebug {
					log.Debugf("slice_worker, host [%v] is not available", host)
				}
				time.Sleep(time.Second * 1)
			}
			goto READ_DOCS
		}
		if global.Env().IsDebug {
			log.Tracef("check waiting queue: %v", processor.config.WaitingAfter)
		}
		if len(processor.config.WaitingAfter) > 0 {
			for _, v := range processor.config.WaitingAfter {
				qCfg := queue.GetOrInitConfig(v)
				hasLag := queue.HasLag(qCfg)

				if global.Env().IsDebug {
					log.Debugf("slice_worker, check queue lag: [%v] for [%v], %v", qCfg.Name, qConfig.Name, hasLag)
				}

				if hasLag {
					log.Warnf("slice_worker, %v has pending messages to consume, cleanup it first", v)
					time.Sleep(5 * time.Second)
					goto READ_DOCS
				}
			}
		}

		//each message is complete bulk message, must be end with \n
		if global.Env().IsDebug {
			log.Tracef("slice_worker, worker:[%v] start consume queue:[%v][%v] offset:%v", workerID, qConfig.ID, sliceID, offset)
		}

		if global.Env().IsDebug {
			log.Debugf("slice_worker, star to consume queue:%v, slice:%v， offset:%v", qConfig.Name, sliceID, offset)
		}

		messages, timeout, err := consumerInstance.FetchMessages(ctx1, consumerConfig.FetchMaxMessages)
		consumerConfig.KeepTouch()
		stats.IncrementBy("queue", qConfig.ID+".msg_fetched_from_queue", int64(len(messages)))

		if global.Env().IsDebug {
			log.Debugf("slice_worker, [%v][%v][%v][%v] fetched message:%v,ctx:%v,timeout:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, len(messages), ctx1.String(), timeout, err)
		}

		//TODO 不能重复处理，也需要处理 offset 的妥善持久化，避免重复数据，也要避免拿不到数据迟迟不退出。

		if err != nil {
			if err.Error() == "EOF" || err.Error() == "unexpected EOF" {
				if len(messages) > 0 || mainBuf.GetMessageCount() > 0 {
					goto HANDLE_MESSAGE
				}

				log.Errorf("slice_worker, error on consume queue:[%v], slice_id:%v, no data fetched, offset: %v, err: %v", qConfig.Name, sliceID, ctx1,err)
				goto CLEAN_BUFFER
				return
			}
			log.Errorf("slice_worker, error on queue:[%v], slice_id:%v, %v", qConfig.Name, sliceID, err)
			log.Flush()
			panic(err)
		}

	HANDLE_MESSAGE:
		//update temp offset, not committed, continued reading
		if ctx1 == nil {
			goto CLEAN_BUFFER
		}

		if len(messages) > 0 {

			if global.Env().IsDebug {
				log.Trace("total messages return from consumer: ", len(messages))
			}
			for msgOffset, pop := range messages {
				if processor.config.ValidateRequest {
					elastic.ValidateBulkRequest("write_pop", string(pop.Data))
				}

				if maxSlices > 1 {
					var totalOps, sliceOps int
					var collectMeta = false
					elastic.WalkBulkRequests(pop.Data, func(eachLine []byte) (skipNextLine bool) {
						return false
					}, func(metaBytes []byte, actionStr, index, typeName, id, routing string, offset int) (err error) {
						totalOps++

						var partitionID int
						var msgID = id
						var hashValue int

						if processor.config.SliceHashUseDocumentID{
							if id != "" {
								//check hash
								xxHash := xxHashPool.Get().(*xxhash.XXHash32)
								defer xxHashPool.Put(xxHash)
								xxHash.Reset()
								xxHash.WriteString(id)
								hashValue= int(xxHash.Sum32()) //TODO hash function to be configurable
							} else {
								hashValue= int(pop.Offset.Position)
								msgID = fmt.Sprintf("%v", msgOffset)
							}
						}else{
							hashValue= int(pop.Offset.Position)
							msgID = fmt.Sprintf("%v", msgOffset)
						}

						partitionID = hashValue % maxSlices

						if global.Env().IsDebug {
							log.Trace("slice_worker, ", sliceID, ",", id, ",", partitionID, ",", msgOffset, ",", partitionID == sliceID)
						}

						if global.Env().IsDebug {
							log.Tracef("slice_worker, [%v][%v][%v][%v] hash msg_id: %v->%v > %v/%v/%v, [%v/%v], [%v,%v,%v,%v,%v], meta:%v, ctx:%v,err:%v",
								qConfig.Name,consumerConfig.Group,consumerConfig.Name,sliceID,
								msgID,hashValue,
								partitionID,sliceID,maxSlices,
								sliceOps,totalOps,
								actionStr, index, typeName, id, routing,
								string(metaBytes),ctx1.String(), err)
						}

						if partitionID == sliceID {
							sliceOps++
							mainBuf.WriteNewByteBufferLine("meta1", metaBytes)
							mainBuf.WriteMessageID(msgID)
							collectMeta = true
						} else {
							collectMeta = false
						}
						return nil
					}, func(payloadBytes []byte, actionStr, index, typeName, id, routing string) {
						if collectMeta {
							mainBuf.WriteNewByteBufferLine("payload1", payloadBytes)
							collectMeta = false
						}
					},nil)
				} else {
					mainBuf.WriteMessageID(pop.Offset.String())
					mainBuf.WriteByteBuffer(pop.Data)
				}

				if global.Env().IsDebug {
					log.Tracef("slice_worker, message count: %v, size: %v", mainBuf.GetMessageCount(), util.ByteSize(uint64(mainBuf.GetMessageSize())))
				}

				msgSize := mainBuf.GetMessageSize()
				msgCount := mainBuf.GetMessageCount()

				if (bulkSizeInByte > 0 && msgSize > (bulkSizeInByte)) || (processor.config.BulkConfig.BulkMaxDocsCount > 0 && msgCount > processor.config.BulkConfig.BulkMaxDocsCount) {
					if global.Env().IsDebug {
						log.Debugf("slice_worker, consuming [%v], slice_id:%v, hit buffer limit, size:%v, count:%v, submit now", qConfig.Name, sliceID, msgSize, msgCount)
					}

					//submit request
					continueNext, err := processor.submitBulkRequest(ctx, qConfig, tag, esClusterID, meta, host, bulkProcessor, mainBuf)
					if global.Env().IsDebug {
						log.Tracef("slice_worker, [%v][%v][%v][%v] submit request:%v,continue:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, mainBuf.GetMessageCount(), continueNext, err)
					}
					if !continueNext {
						//TODO handle 429 gracefully
						if !util.ContainStr(err.Error(), "code 429") {
							panic(errors.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host:%v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err))
						}
						log.Errorf("error on submit bulk_requests, queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err)
						time.Sleep(time.Duration(processor.config.RetryDelayIntervalInMs) * time.Millisecond)
						continue
					} else {
						//reset buffer
						mainBuf.ResetData()
						if offset!=nil&& committedOffset !=nil&& !pop.NextOffset.Equals(*committedOffset) {
							err := consumerInstance.CommitOffset(pop.NextOffset)
							if err != nil {
								panic(err)
							}

							if global.Env().IsDebug {
								log.Tracef("slice_worker, [%v][%v][%v][%v] success commit offset:%v,ctx:%v,timeout:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, *offset,committedOffset, ctx1.String(), timeout, err)
							}
							//log.Infof("%v, success commit offset to: %v, previous init: %v", consumerConfig.String(),*offset,committedOffset)

							committedOffset = &pop.NextOffset
							//log.Infof("%v, update init offset to: %v", consumerConfig.String(),committedOffset)

						}
						offset = &pop.NextOffset
						//log.Infof("%v, update offset to: %v", consumerConfig.String(),offset)

					}
				}

			}

			offset = &ctx1.NextOffset //TODO
		}

		if time.Since(lastCommit) > idleDuration && mainBuf.GetMessageSize() > 0 {
			if global.Env().IsDebug {
				log.Trace("slice_worker, hit idle timeout, ", idleDuration.String())
			}
			goto CLEAN_BUFFER
		}
		if len(messages) == 0 && mainBuf.GetMessageSize() == 0 {
			if global.Env().IsDebug {
				log.Trace("slice_worker, no message found in queue: " + qConfig.ID)
			}
			return
		}
	}

CLEAN_BUFFER:

	if global.Env().IsDebug {
		log.Tracef("slice_worker, [%v][%v][%v][%v] cleanup buffer:%v,ctx:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, offset, ctx1.String(), err)
	}

	if global.Env().IsDebug {
		log.Debugf("cleanup buffer, queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err)
	}
	lastCommit = time.Now()
	// check bulk result, if ok, then commit offset, or retry non-200 requests, or save failure offset
	if mainBuf.GetMessageCount()>0{
		continueNext, err := processor.submitBulkRequest(ctx, qConfig, tag, esClusterID, meta, host, bulkProcessor, mainBuf)
		if global.Env().IsDebug {
			log.Tracef("slice_worker, [%v][%v][%v][%v] submit request:%v,continue:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, mainBuf.GetMessageCount(), continueNext, err)
		}

		if !continueNext {
			log.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err)
		}

		if global.Env().IsDebug {
			log.Debug(qConfig.ID, ",", qConfig.Name, ",", offset, ",", committedOffset, ",", continueNext, ",", err)
		}

		if continueNext {
			//reset buffer
			mainBuf.ResetData()
			if offset!=nil&& committedOffset !=nil&& !offset.Equals(*committedOffset) {
				err := consumerInstance.CommitOffset(*offset)
				if err != nil {
					panic(err)
				}
				//log.Infof("%v, success commit offset to: %v, previous init: %v", consumerConfig.String(),*offset,committedOffset)

				committedOffset = offset

				if global.Env().IsDebug {
					log.Tracef("slice_worker, [%v][%v][%v][%v] commit offset:%v,ctx:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, offset, ctx1.String(), err)
				}

			}
		} else {
			//logging failure offset boundry
			//TODO handle 429 gracefully
			if !util.ContainStr(err.Error(), "429") {
				panic(errors.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err))
			}

			if global.Env().IsDebug {
				log.Tracef("slice_worker, [%v][%v][%v][%v] skip continue:%v,ctx:%v,err:%v",qConfig.Name,consumerConfig.Group,consumerConfig.Name, sliceID, offset, ctx1.String(), err)
			}

			log.Errorf("queue:[%v], slice_id:%v, offset [%v]-[%v], bulk failed (host: %v, err: %v)", qConfig.ID, sliceID, committedOffset, offset, host, err)
			time.Sleep(time.Duration(processor.config.RetryDelayIntervalInMs) * time.Millisecond)
			goto CLEAN_BUFFER
		}
	}

	if ctx.IsCanceled() || ctx.HasError() {
		log.Debugf("offset[%v], canceled[%v], errors[%v], return on queue:[%v], slice_id:%v", offset, ctx.IsCanceled(), ctx.Errors(), qConfig.Name, sliceID)
		return
	}
	if global.Env().IsDebug {
		log.Tracef("slice_worker, goto READ_DOCS, return on queue:[%v], slice_id:%v", qConfig.Name, sliceID)
	}
	if global.ShuttingDown() {
		return
	}

	if !ctx.IsCanceled() {
		goto READ_DOCS
	}
}

func (processor *BulkIndexingProcessor) submitBulkRequest(ctx *pipeline.Context, qConfig *queue.QueueConfig, tag, esClusterID string, meta *elastic.ElasticsearchMetadata, host string, bulkProcessor elastic.BulkProcessor, mainBuf *elastic.BulkBuffer) (bool, error) {

	stats.IncrementBy("queue", qConfig.ID+".docs_submit_bulk", int64(mainBuf.GetMessageCount()))

	if mainBuf == nil || meta == nil {
		return true, errors.New("invalid buffer or meta")
	}

	count := mainBuf.GetMessageCount()
	size := mainBuf.GetMessageSize()

	if count > 0 && size == 0 || count == 0 && size > 0 {
		panic(errors.Errorf("invalid bulk message, count: %v, size:%v, msg: %v", mainBuf.GetMessageCount(), mainBuf.GetMessageSize(), string(mainBuf.GetMessageBytes())))
	}

	if count > 0 && size > 0 {

		if global.Env().IsDebug {
			log.Infof("submit bulk request, count: %v, size:%v", count, util.ByteSize(uint64(size)))
		}

		start := time.Now()
		continueRequest, statsMap, bulkResult, err := bulkProcessor.Bulk(ctx.Context, tag, meta, host, mainBuf)
		if global.Env().IsDebug {
			stats.Timing("elasticsearch."+esClusterID+".bulk", "elapsed_ms", time.Since(start).Milliseconds())
		}

		total:=0
		for k, v := range statsMap {
			stats.IncrementBy("queue", qConfig.ID+".docs_status_code."+util.ToString(k), int64(v))
			total+=v
		}

		stats.IncrementBy("queue", qConfig.ID+".docs_fetched_from_queue", int64(total))

		if err != nil && processor.config.LogBulkError {
			var msg elastic.BulkDetail
			if bulkResult!=nil{
				msg=bulkResult.Detail
			}
			log.Warnf("elasticsearch [%v], stats:%v, detail: %v, err:%v", meta.Config.Name, statsMap, msg, err)
		}

		if global.Env().IsDebug {
			log.Debug(tag, ", ", meta.Config.Name, ", ", host, ", stats: ", statsMap, ", count: ", count, ", size: ", util.ByteSize(uint64(size)), ", elapsed: ", time.Since(start), ", continue: ", continueRequest, ", bulkResult: ", bulkResult)
		} else {
			if processor.config.VerboseBulkResult {
				log.Info("queue:", qConfig.Name, ", ", meta.Config.Name, ", ", host, ", stats: ", statsMap, ", count: ", count, ", size: ", util.ByteSize(uint64(size)), ", elapsed: ", time.Since(start), ", continue: ", continueRequest)
			}
		}
		processor.updateContext(ctx, bulkResult)
		return continueRequest, err
	}

	return true, nil
}

func (processor *BulkIndexingProcessor) updateContext(ctx *pipeline.Context, bulkResult *elastic.BulkResult) {
	if bulkResult == nil {
		return
	}
	processor.statsLock.Lock()
	defer processor.statsLock.Unlock()
	processor.bulkStats.Summary.Failure.Count += bulkResult.Summary.Failure.Count
	processor.bulkStats.Summary.Failure.Size += bulkResult.Summary.Failure.Size
	processor.bulkStats.Summary.Invalid.Count += bulkResult.Summary.Invalid.Count
	processor.bulkStats.Summary.Invalid.Size += bulkResult.Summary.Invalid.Size
	processor.bulkStats.Summary.Success.Count += bulkResult.Summary.Success.Count
	processor.bulkStats.Summary.Success.Size += bulkResult.Summary.Success.Size
	ctx.PutValue("bulk_indexing.failure", processor.bulkStats.Summary.Failure)
	ctx.PutValue("bulk_indexing.success", processor.bulkStats.Summary.Success)
	ctx.PutValue("bulk_indexing.invalid", processor.bulkStats.Summary.Invalid)

	processor.bulkStats.Detail.Invalid.Documents = appendStrArr(processor.bulkStats.Detail.Invalid.Documents, 10, bulkResult.Detail.Invalid.Documents)
	processor.bulkStats.Detail.Invalid.Reasons = appendStrArr(processor.bulkStats.Detail.Invalid.Reasons, 10, bulkResult.Detail.Invalid.Reasons)
	processor.bulkStats.Detail.Failure.Documents = appendStrArr(processor.bulkStats.Detail.Failure.Documents, 10, bulkResult.Detail.Failure.Documents)
	processor.bulkStats.Detail.Failure.Reasons = appendStrArr(processor.bulkStats.Detail.Failure.Reasons, 10, bulkResult.Detail.Failure.Reasons)
	processor.bulkStats.ErrorMsgs = appendStrArr(processor.bulkStats.ErrorMsgs, 10, bulkResult.ErrorMsgs)

	ctx.PutValue("bulk_indexing.error_msgs", processor.bulkStats.ErrorMsgs)
	ctx.PutValue("bulk_indexing.detail.failure", processor.bulkStats.Detail.Failure)
	ctx.PutValue("bulk_indexing.detail.invalid", processor.bulkStats.Detail.Invalid)
}

func appendStrArr(arr []string, size int, elems []string) []string {
	if len(arr) >= size {
		return arr
	}
	remaining := size - len(arr)
	if len(elems) > remaining {
		return append(arr, elems[0:remaining]...)
	}
	return append(arr, elems...)
}

func (processor *BulkIndexingProcessor) getElasticsearchMeatadata(qConfig *queue.QueueConfig) (string, *elastic.ElasticsearchMetadata) {

	elasticsearch, ok := qConfig.Labels["elasticsearch"]
	if !ok {
		if processor.config.Elasticsearch == "" {
			panic(errors.Errorf("slice_worker, label [elasticsearch] was not found in: %v", qConfig))
		}
		elasticsearch = processor.config.Elasticsearch
	}

	esClusterID := util.ToString(elasticsearch)
	meta := elastic.GetMetadata(esClusterID)
	if meta == nil {
		esConfig := elastic.GetConfigNoPanic(esClusterID)
		if esConfig == nil {
			if processor.config.ElasticsearchConfig != nil {
				processor.config.ElasticsearchConfig.Source = "bulk_indexing"
				esConfig = processor.config.ElasticsearchConfig
			}
			esConfig = processor.config.ElasticsearchConfig
		}
		if esConfig == nil {
			return esClusterID, nil
		}
		meta = elastic.GetOrInitMetadata(esConfig)
	}
	return esClusterID, meta
}

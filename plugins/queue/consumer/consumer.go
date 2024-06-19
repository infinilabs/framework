package consumer

import (
	"fmt"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/locker"
	"runtime"
	"sync"
	"time"

	"github.com/OneOfOne/xxhash"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

type QueueConsumerProcessor struct {
	config               *Config
	wg                   sync.WaitGroup
	inFlightQueueConfigs sync.Map
	failedQueueConfigs   sync.Map
	detectorRunning      bool
	id                   string
	sync.RWMutex
	pool *pipeline.Pool

	processors *pipeline.Processors
	onCleanup  func() bool
}

type MessageHandlerAPI interface {
	HandleMessages(c *pipeline.Context, msgs []queue.Message) (bool, error)
}

type Config struct {
	NumOfSlices             int   `config:"num_of_slices"`
	Slices                  []int `config:"slices"`
	enabledSlice            map[int]int
	IdleTimeoutInSecond     int                    `config:"idle_timeout_in_seconds"`
	MaxConnectionPerHost    int                    `config:"max_connection_per_node"`
	QueueLabels             map[string]interface{} `config:"queues,omitempty"`
	Selector                queue.QueueSelector    `config:"queue_selector"`
	Consumer                *queue.ConsumerConfig  `config:"consumer"`
	MaxWorkers              int                    `config:"max_worker_size"`
	DetectActiveQueue       bool                   `config:"detect_active_queue"`
	DetectIntervalInMs      int                    `config:"detect_interval"`
	QuitDetectAfterIdleInMs int                    `config:"quite_detect_after_idle_in_ms"`

	MessageProcessors []*config.Config `config:"processor"`

	SkipEmptyQueue bool `config:"skip_empty_queue"`
	QuitOnEOFQueue bool `config:"quit_on_eof_queue"`

	QuitNeedTag     bool   `config:"quit_need_tag"`      //need tag to quit, or wait for timeout
	QuitNeedTagName string `config:"quit_need_tag_name"` //need tag to quit, or wait for timeout

	QueueField             string   `config:"queue_name_field"`
	MessageField           string   `config:"message_field"`
	WaitingAfter           []string `config:"waiting_after"`
	RetryDelayIntervalInMs int      `config:"retry_delay_interval"`
	AutoCommitOffset       bool     `config:"auto_commit_offset"`
}

const name = "consumer"

func init() {
	pipeline.RegisterProcessorPlugin(name, New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		NumOfSlices:             1,
		MaxWorkers:              10,
		MaxConnectionPerHost:    1,
		IdleTimeoutInSecond:     5,
		DetectIntervalInMs:      5000,
		QuitDetectAfterIdleInMs: 30000,
		QueueField:              "queue_name",
		MessageField:            "messages",
		AutoCommitOffset:        true,
		Selector: queue.QueueSelector{
			Labels: map[string]interface{}{},
		},

		Consumer: &queue.ConsumerConfig{
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
		SkipEmptyQueue:         false,
		QuitOnEOFQueue:         true,
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

	runner := QueueConsumerProcessor{
		id:                   util.GetUUID(),
		config:               &cfg,
		inFlightQueueConfigs: sync.Map{},
	}

	runner.wg = sync.WaitGroup{}

	if runner.config.MaxWorkers < 0 {
		runner.config.MaxWorkers = 1
	}

	processor, err := pipeline.NewPipeline(runner.config.MessageProcessors)
	if err != nil {
		panic(err)
	}

	processor.SkipCatchError=true //skip catch internal error
	runner.processors = processor

	pool, err := pipeline.NewPoolWithTag(name, runner.config.MaxWorkers)
	if err != nil {
		panic(err)
	}

	runner.pool = pool

	return &runner, nil
}

func (processor *QueueConsumerProcessor) Release() error {
	if processor.pool != nil {
		processor.pool.Release()
		processor.pool = nil
	}
	return nil
}

func (processor *QueueConsumerProcessor) Name() string {
	return name
}

func (processor *QueueConsumerProcessor) Process(c *pipeline.Context) error {
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
				log.Error("error in consumer processor,", v)
			}
		}
		log.Debug("exit consumer processor")
	}()

	//handle updates
	if processor.config.DetectActiveQueue {
		log.Tracef("detector running [%v]", processor.detectorRunning)
		if !processor.detectorRunning {
			processor.detectorRunning = true
			processor.wg.Add(1)
			go func(c *pipeline.Context) {
				log.Tracef("init detector for active queue [%v] ", processor.id)
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
							log.Error("error in queue processor,", v)
						}
					}
					processor.detectorRunning = false
					log.Debug("exit detector for active queue")
					processor.wg.Done()
				}()

				var lastDispatch = time.Now()

				for {
					if c.IsCanceled() {
						return
					}

					log.Tracef("inflight queues: %v", util.MapLength(&processor.inFlightQueueConfigs))

					if global.Env().IsDebug {
						processor.inFlightQueueConfigs.Range(func(key, value interface{}) bool {
							log.Tracef("inflight queue:%v", key)
							return true
						})
					}

					cfgs := queue.GetConfigBySelector(&processor.config.Selector)

					//log.Errorf("filter queue by:%v, num of queues:%v", processor.config.Selector.ToString(), len(cfgs))

					for _, v := range cfgs {
						if c.IsCanceled() {
							return
						}

						//if have depth and not in in flight
						if !processor.config.SkipEmptyQueue || queue.HasLag(v) {
							_, ok := processor.inFlightQueueConfigs.Load(v.ID)
							if !ok {
								log.Tracef("detecting new queue: %v", v.Name)
								lastDispatch = time.Now()
								err := processor.HandleQueueConfig(v, c)
								if err != nil {
									panic(err)
									//return
								}
							}
						}
					}
					if processor.config.DetectIntervalInMs > 0 {
						time.Sleep(time.Millisecond * time.Duration(processor.config.DetectIntervalInMs))
					}

					if time.Since(lastDispatch) > time.Millisecond*time.Duration(processor.config.QuitDetectAfterIdleInMs) {
						log.Tracef("quite detect after idle for %v ms", processor.config.QuitDetectAfterIdleInMs)
						inflight := util.MapLength(&processor.inFlightQueueConfigs)
						if inflight == 0 {
							log.Debugf("quite detect after idle for %v ms, inflight: %v", processor.config.QuitDetectAfterIdleInMs, inflight)
							return
						}
					}
				}
			}(c)
		}
	} else {
		cfgs := queue.GetConfigBySelector(&processor.config.Selector)
		log.Debugf("filter queue by:%v, num of queues:%v", processor.config.Selector.ToString(), len(cfgs))
		for _, v := range cfgs {
			log.Tracef("checking queue: %v", v)
			err := processor.HandleQueueConfig(v, c)
			if err != nil {
				panic(err)
			}
		}
	}

	processor.wg.Wait()

	return nil
}

const queueConsumerHandleSingleton = "queue_consumer_handler_singleton"

func (processor *QueueConsumerProcessor) HandleQueueConfig(qConfig *queue.QueueConfig, ctx *pipeline.Context) error {

	log.Tracef("handle queue config:%v ", qConfig.Name)

	ok, _ := locker.Hold(queueConsumerHandleSingleton, qConfig.ID, global.Env().SystemConfig.NodeConfig.ID, 60*time.Second, true)
	if !ok {
		log.Debugf("failed to hold lock for queue:[%v], already hold by somewhere", qConfig.ID)
		return nil
	}

	var sliceStats = qConfig.ID + "FAILED_SLICES"

	if ctx.Stats(sliceStats) >= processor.config.NumOfSlices {
		log.Debugf("all slices failed for queue [%v], skip", qConfig.Name)
		return errors.Errorf("all slices failed for queue [%v], skip", qConfig.Name)
	}

	//check slice
	for sliceID := 0; sliceID < processor.config.NumOfSlices; sliceID++ {
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
			return nil
		}

		processor.Lock()
		v2, exists := processor.inFlightQueueConfigs.Load(key)
		if exists {
			log.Debugf("queue [%v], slice_id:%v has more then one consumer, key:%v,v:%v", qConfig.ID, sliceID, key, v2)
			processor.Unlock()
			continue
		} else {
			var workerID = util.GetUUID()
			log.Debugf("starting worker:[%v], queue:[%v], slice_id:%v", workerID, qConfig.Name, sliceID)

			processor.wg.Add(1)
			contextForWorker := pipeline.Context{}
			contextForWorker.ResetContext()
			err := processor.pool.Submit(&pipeline.Task{
				Handler: func(ctx *pipeline.Context, v ...interface{}) {
					processor.NewSlicedWorker(ctx, v...)
					//if slice worker failed, add to failed queue
					if ctx.IsFailed() || ctx.HasError() {
						if len(v) > 4 {
							parentContext := v[4].(*pipeline.Context)
							if parentContext != nil {
								parentContext.Increment(sliceStats, 1)
							}
						}
					}
				},
				Context: &contextForWorker,
				Params:  []interface{}{qConfig, workerID, sliceID, processor.config.NumOfSlices, ctx}, //在创建任务时设置参数
			})
			processor.Unlock()
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}

var xxHashPool = sync.Pool{
	New: func() interface{} {
		return xxhash.New32()
	},
}

func (processor *QueueConsumerProcessor) NewSlicedWorker(ctx *pipeline.Context, v ...interface{}) {
	qConfig := v[0].(*queue.QueueConfig)
	workerID := v[1].(string)
	sliceID := v[2].(int)
	maxSlices := v[3].(int)
	parentContext := v[4].(*pipeline.Context)

	key := fmt.Sprintf("%v-%v", qConfig.ID, sliceID)

	if global.Env().IsDebug {
		log.Debugf("new slice_worker: %v, %v, %v, %v", key, workerID, sliceID, qConfig.ID)
	}

	//defer log.Errorf("exit slice_worker: %v, %v, %v, %v", key, workerID, sliceID, qConfig.ID)

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
				log.Errorf("error in consumer processor, %v, queue:%v, slice_id:%v", v, qConfig.ID, sliceID)
			}
		}
		processor.inFlightQueueConfigs.Delete(key)
		processor.wg.Done()
		log.Tracef("exit slice_worker, queue:%v, slice_id:%v, key:%v", qConfig.ID, sliceID, key)
	}()

	var initOffset queue.Offset
	var offset queue.Offset
	var groupName = processor.config.Consumer.Group
	if maxSlices > 1 {
		if processor.config.Consumer.SimpleSlicedGroup {
			groupName = fmt.Sprintf("%v-%v", processor.config.Consumer.Group, sliceID)
		}else{
			groupName = fmt.Sprintf("%v-%v-%v", processor.config.Consumer.Group, qConfig.ID, sliceID)
		}
	}

	var consumerConfig = queue.GetOrInitConsumerConfig(qConfig.ID, groupName, processor.config.Consumer.Name)
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

	//skip empty queue
	if processor.config.SkipEmptyQueue && !queue.ConsumerHasLag(qConfig, consumerConfig) {
		if global.Env().IsDebug {
			log.Tracef("skip empty queue:[%v]", qConfig.Name)
		}
		return
	}

	xxHash := xxHashPool.Get().(*xxhash.XXHash32)
	defer xxHashPool.Put(xxHash)

	defer func() {
		defer log.Debugf("exit worker[%v], queue:[%v], slice_id:%v", workerID, qConfig.ID, sliceID)
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
				log.Errorf("worker[%v], queue:[%v], slice:[%v], offset:[%v]->[%v],%v", workerID, qConfig.ID, sliceID, initOffset, offset, v)
				ctx.Failed(fmt.Errorf("panic in slice worker: %+v", r))
				if parentContext != nil {
					parentContext.RecordError(fmt.Errorf("panic in slice worker: %+v", r))
				}
			}
		}

		if parentContext != nil && (parentContext.IsFailed()) || ctx.IsFailed() {
			return
		}

		if processor.onCleanup != nil {
			if !processor.onCleanup() {
				log.Warnf("failed to cleanup on queue:[%v], slice_id:%v, offset[%v]", qConfig.Name, sliceID, offset)
				return
			}
		}

		if processor.config.AutoCommitOffset {
			//cleanup buffer before exit worker
			if !offset.Equals(initOffset) {
				ok, err := queue.CommitOffset(qConfig, consumerConfig, offset)
				if !ok || err != nil {
					panic(err)
				}
				initOffset = offset
			}
		}

	}()

	processor.inFlightQueueConfigs.Store(key, workerID)

	log.Tracef("place slice_worker lock, queue [%v], slice_id:%v, key:%v,v:%v", qConfig.ID, sliceID, key, workerID)

	idleDuration := time.Duration(processor.config.IdleTimeoutInSecond) * time.Second

	var lastCommit time.Time = time.Now()
	initOffset, _ = queue.GetOffset(qConfig, consumerConfig)

	if global.Env().IsDebug {
		log.Debugf("slice_worker, get init offset: %v for consumer:%v,%v", initOffset, groupName, consumerConfig.Name)
	}
	offset = initOffset

	consumerInstance, err := queue.AcquireConsumer(qConfig, consumerConfig,workerID)
	defer queue.ReleaseConsumer(qConfig, consumerConfig, consumerInstance)

	if err != nil {
		if util.ContainStr(err.Error(), "already owning this topic") {
			if global.Env().IsDebug {
				log.Infof("other consumer already owning this topic, queue:%v-%v, slice_id:%v, skipping...", qConfig.Name, qConfig.ID, sliceID)
			}
			return
		}
	}

	if err != nil || consumerInstance == nil {
		panic(err)
	}

	ctx1 := &queue.Context{}

READ_DOCS:

	//log.Error("read docs: ",qConfig.Name)

	//TODO
	//consumerInstance.ResetOffset(queue.ConvertOffset(offset))
	EOF := false
	for {
		if global.ShuttingDown() || ctx.IsCanceled() || ctx.IsFailed() || parentContext != nil && (parentContext.IsFailed() || parentContext.IsCanceled()) {
			goto CLEAN_BUFFER
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

		if global.Env().IsDebug {
			log.Tracef("slice_worker, worker:[%v] start consume queue:[%v][%v] offset:%v", workerID, qConfig.ID, sliceID, offset)
		}
		consumerConfig.KeepActive()
		messages, timeout, err := consumerInstance.FetchMessages(ctx1, consumerConfig.FetchMaxMessages)
		if global.Env().IsDebug {
			log.Infof("[%v] slice_worker, [%v][%v] consume message:%v,ctx:%v,timeout:%v,err:%v", qConfig.Name, consumerConfig.Name, sliceID, len(messages), ctx1.String(), timeout, err)
		}

		if err != nil {
			if err.Error() == "EOF" || err.Error() == "unexpected EOF" {
				if len(messages) > 0 {
					goto HANDLE_MESSAGE
				}

				//log.Errorf("slice_worker, error on consume queue:[%v], slice_id:%v, no data fetched, offset: %v", qConfig.Name, sliceID, ctx1)
				goto CLEAN_BUFFER
				return
			}
			//log.Errorf("slice_worker, error on queue:[%v], slice_id:%v, %v", qConfig.Name, sliceID, err)
			log.Flush()
			panic(err)
		}

	HANDLE_MESSAGE:
		//log.Error("handle message: ",qConfig.Name,",",len(messages))
		//update temp offset, not committed, continued reading
		if ctx1 == nil {
			goto CLEAN_BUFFER
		}

		//filter message per slice
		if processor.config.NumOfSlices > 1 {
			newMessages := make([]queue.Message, 0)
			for _, m := range messages {
				partitionID := int(m.Offset.Position % int64(maxSlices))
				if partitionID == sliceID {
					newMessages = append(newMessages, m)
				}
			}
			if len(newMessages)>0 {
				messages = newMessages
			}
		}

		if len(messages) > 0 {

			newCtx := pipeline.Context{}
			newCtx.ParentContext = ctx
			newCtx.Context = ctx.Context
			newCtx.Data = ctx.CloneData()

			_, err := newCtx.PutValue(processor.config.QueueField, qConfig.Name)
			if err != nil {
				panic(err)
			}

			_, err = newCtx.PutValue("QUEUE_CONFIG", qConfig)
			if err != nil {
				panic(err)
			}

			_, err = newCtx.PutValue("CONSUMER_CONFIG", consumerConfig)
			if err != nil {
				panic(err)
			}

			_, err = newCtx.PutValue(processor.config.MessageField, messages)
			if err != nil {
				panic(err)
			}

			//log.Error("start processing message:",len(messages),",",qConfig.Name)
			err = processor.processors.Process(&newCtx)
			//log.Error("end processing message:",len(messages),",",qConfig.Name,",",err)
			if err != nil {
				panic(err)
			}
			offset = ctx1.NextOffset //TODO
			messages = nil
		} else {
			EOF = true
		}

		if time.Since(lastCommit) > idleDuration || EOF {
			if global.Env().IsDebug {
				log.Trace("slice_worker, hit idle timeout or empty message ", idleDuration.String())
			}
			goto CLEAN_BUFFER
		}
	}

CLEAN_BUFFER:

	//log.Error("clean buffer: ",qConfig.Name)

	lastCommit = time.Now()

	if global.Env().IsDebug {
		log.Info("commit offset: ", offset, ",", qConfig.Name)
	}

	if processor.onCleanup != nil {
		if !processor.onCleanup() {
			log.Warnf("offset[%v], canceled[%v], errors[%v], failed cleanup on queue:[%v], slice_id:%v", offset, ctx.IsCanceled(), ctx.Errors(), qConfig.Name, sliceID)
			return
		}
	}

	if processor.config.AutoCommitOffset {
		if !offset.Equals(initOffset) {
			ok, err := queue.CommitOffset(qConfig, consumerConfig, offset)
			if !ok || err != nil {
				panic(err)
			}
			initOffset = offset
		}
	}

	if global.ShuttingDown() {
		return
	}

	if processor.config.QuitOnEOFQueue && EOF {

		if processor.config.QuitNeedTag && processor.config.QuitNeedTagName != "" && !ctx.HasTag(processor.config.QuitNeedTagName) {
			time.Sleep(1 * time.Second)
			log.Debug("EOF without quit tag, sleep 1s: ", qConfig.Name)
			goto READ_DOCS
		}

		ctx.CancelTask()
		log.Debug("EOF, cancel task: ", qConfig.Name)
		return
	}

	log.Tracef("slice_worker, goto READ_DOCS, return on queue:[%v], slice_id:%v", qConfig.Name, sliceID)
	if !ctx.IsCanceled() && !(parentContext != nil && (parentContext.IsFailed() || parentContext.IsCanceled())) {
		goto READ_DOCS
	}
}

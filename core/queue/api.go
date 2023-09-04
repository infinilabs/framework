/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

type QueueAPI interface {
	Name() string
	Init(string) error
	Close(string) error
	GetStorageSize(k string) uint64
	Destroy(string) error
	GetQueues() []string

	Push(string, []byte) error
}

type SimpleQueueAPI interface {
	QueueAPI
	Pop(string, time.Duration) (data []byte, timeout bool)
	Depth(string) int64
}

type AdvancedQueueAPI interface {
	QueueAPI

	LatestOffset(k *QueueConfig) Offset
	GetOffset(k *QueueConfig, consumer *ConsumerConfig) (Offset, error)
	DeleteOffset(k *QueueConfig, consumer *ConsumerConfig) error
	CommitOffset(k *QueueConfig, consumer *ConsumerConfig, offset Offset) (bool, error)

	AcquireConsumer(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error)
	ReleaseConsumer(k *QueueConfig, c *ConsumerConfig,consumer ConsumerAPI) error

	AcquireProducer(cfg *QueueConfig) (ProducerAPI, error)
	ReleaseProducer(k *QueueConfig,producer ProducerAPI) error
}

type ProducerAPI interface {
	Produce(*[]ProduceRequest) (*[]ProduceResponse, error)
	Close() error
}

type ConsumerAPI interface {
	Close() error
	ResetOffset(segment, readPos int64) (err error)
	FetchMessages(ctx *Context, numOfMessages int) (messages []Message, isTimeout bool, err error)
	CommitOffset(offset Offset) error
}

var defaultHandler QueueAPI

func getSimpleHandler(k *QueueConfig) SimpleQueueAPI {
	handler := getHandler(k)
	if handler != nil {
		h, ok := handler.(SimpleQueueAPI)
		if ok {
			return h
		}
	}
	panic(errors.Errorf("no handler registered for queue [%v][%v]", k.Name, k.Type))
}

func getAdvancedHandler(k *QueueConfig) AdvancedQueueAPI {
	handler := getHandler(k)
	if handler != nil {
		h, ok := handler.(AdvancedQueueAPI)
		if ok {
			return h
		}
	}
	panic(errors.Errorf("no handler registered for queue [%v][%v]", k.Name, k.Type))
}

func GetHandlerByType(t string) QueueAPI {
	handler, ok := adapters[t]
	if ok && handler != nil {
		return handler
	}
	if defaultHandler == nil {
		panic(errors.New("no queue handler was found"))
	}
	return defaultHandler
}

func getHandler(k *QueueConfig) QueueAPI {
	return GetHandlerByType(k.Type)
}

var configs = sync.Map{}
var idConfigs = sync.Map{}
var consumerCfgLock = sync.RWMutex{}

const BucketWhoOwnsThisTopic = "who_owns_this_topic" //queue_group: node_id/timestamp

func AcquireConsumer(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error) {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getAdvancedHandler(k)
	if handler != nil {
		return handler.AcquireConsumer(k, consumer)
	}
	panic(errors.New("handler is not registered"))
}

func ReleaseConsumer(k *QueueConfig, c *ConsumerConfig,consumer ConsumerAPI) error{
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getAdvancedHandler(k)
	if handler != nil {
		return handler.ReleaseConsumer(k, c,consumer)
	}
	panic(errors.New("handler is not registered"))
}

func AcquireProducer(cfg *QueueConfig) (ProducerAPI, error) {

	handler := GetHandlerByType(cfg.Type)
	if handler != nil {
		x, ok := handler.(AdvancedQueueAPI)
		if ok {
			return x.AcquireProducer(cfg)
		}
	}
	panic(errors.New("handler is not registered"))
}

func Close(k *QueueConfig) error {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		o := handler.Close(k.ID)
		stats.Increment("queue", k.ID, "close")
		return o
	}
	stats.Increment("queue", k.ID, "close_error")
	panic(errors.New("handler is not closed"))
}

func GetEarlierOffsetStrByQueueID(queueID string) Offset {
	_, seg, pos := GetEarlierOffsetByQueueID(queueID)
	return NewOffset(seg, pos)
}

func GetEarlierOffsetByQueueID(queueID string) (consumerSize int, segment int64, pos int64) {
	q, ok := GetConfigByUUID(queueID)
	if !ok {
		q, ok = GetConfigByKey(queueID)
		if !ok {
			panic(errors.Errorf("queue [%v] was not found", queueID))
		}

		oldID := queueID
		queueID = q.ID

		if global.Env().IsDebug {
			log.Tracef("[%v] is not a valid uuid, found as key, continue as [%v]", oldID, queueID)
		}
	}
	consumers, ok := GetConsumerConfigsByQueueID(queueID)

	if !ok {
		if global.Env().IsDebug {
			log.Warnf("no consumer found for queue [%v]", queueID)
		}
		return 0, 0, 0
	}
	var iPart int64 = 0
	var iPos int64 = 0
	var init = true
	for _, v := range consumers {
		offset, err := GetOffset(q, v)
		if err == nil {

			if init {
				iPart = offset.Segment
				iPos = offset.Position
				init = false
			} else {

				if offset.Position < iPos {
					iPos = offset.Position
				}
				if offset.Segment < iPart {
					iPart = offset.Segment
					iPos = offset.Position
				}
			}
		}
	}
	return len(consumers), iPart, iPos
}

func GetLatestOffsetByQueueID(queueID string) (consumerSize int, segment int64, pos int64) {
	q, ok := GetConfigByUUID(queueID)
	if !ok {
		q, ok = GetConfigByKey(queueID)
		if !ok {
			panic(errors.Errorf("queue [%v] was not found", queueID))
		}

		oldID := queueID
		queueID = q.ID

		if global.Env().IsDebug {
			log.Tracef("[%v] is not a valid uuid, found as key, continue as [%v]", oldID, queueID)
		}
	}
	consumers, ok := GetConsumerConfigsByQueueID(queueID)
	if !ok {
		return 0, 0, 0
	}
	var iPart int64
	var iPos int64
	for _, v := range consumers {
		offset, err := GetOffset(q, v)
		if err == nil {

			if offset.Position > iPos {
				iPos = offset.Position
			}
			if offset.Segment > iPart {
				iPart = offset.Segment
				iPos = offset.Position
			}
		}
	}
	return len(consumers), iPart, iPos
}

func GetOffset(k *QueueConfig, consumer *ConsumerConfig) (Offset, error) {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getAdvancedHandler(k)
	if handler != nil {
		return handler.GetOffset(k, consumer)
	}
	panic(errors.New("handler is not registered"))
}

func DeleteOffset(k *QueueConfig, consumer *ConsumerConfig) error {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getAdvancedHandler(k)
	if handler != nil {
		return handler.DeleteOffset(k, consumer)
	}
	panic(errors.New("handler is not registered"))
}

func CommitOffset(k *QueueConfig, consumer *ConsumerConfig, offset Offset) (bool, error) {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getAdvancedHandler(k)
	if handler != nil {
		return handler.CommitOffset(k, consumer, offset)
	}
	panic(errors.New("handler is not registered"))
}

func GetStorageSize(k string) uint64 {
	if k == "" {
		panic(errors.New("queue name can't be nil"))
	}
	cfg, _ := SmartGetConfig(k)
	handler := getAdvancedHandler(cfg)
	if handler != nil {
		return handler.GetStorageSize(k)
	}
	panic(errors.New("handler is not registered"))
}

func Destroy(k *QueueConfig) error {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		return handler.Destroy(k.ID)
	}
	panic(errors.New("handler is not registered"))
}

func HasLag(k *QueueConfig) bool {
	if global.Env().IsDebug {
		stats.Increment("queue", k.ID, "check_lag")
	}

	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)

	if handler != nil {

		latestProduceOffset := LatestOffset(k)
		offset := GetEarlierOffsetStrByQueueID(k.ID)
		log.Tracef("latestProduceOffset: %v, offset: %v", latestProduceOffset, offset)
		if !latestProduceOffset.Equals(offset) {
			return true
		}

		return false
	}

	panic(errors.New("handler is not registered"))
}

func ConsumerHasLag(k *QueueConfig, c *ConsumerConfig) bool {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)

	if handler != nil {
		latestProduceOffset := LatestOffset(k)
		offset, err := GetOffset(k, c)
		if err != nil {
			panic(err)
		}

		if global.Env().IsDebug{
			log.Debugf("queue:%v,consumer:%v,latestProduceOffset: %v, offset: %v",k.Name,c.Key(), latestProduceOffset, offset)
		}

		if latestProduceOffset != offset {
			return true
		}

		if global.Env().IsDebug {
			stats.Increment("queue", k.ID, "check_consumer_lag")
		}
		return false
	}

	panic(errors.New("handler is not registered"))
}

func LatestOffset(k *QueueConfig) Offset {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getAdvancedHandler(k)
	if handler != nil {
		o := handler.LatestOffset(k)
		return o
	}
	panic(errors.New("handler is not registered"))
}

func GetQueues() map[string][]string {
	results := map[string][]string{}
	for q, handler := range adapters {
		result := []string{}
		if handler != nil {
			o := handler.GetQueues()
			if global.Env().IsDebug {
				stats.Increment("queue", q, "get_queues")
			}
			result = append(result, o...)
			results[q] = result
		}
	}
	return results
}

func GetConfigBySelector(selector *QueueSelector) []*QueueConfig {
	cfgs := []*QueueConfig{}
	if selector != nil {
		if len(selector.Ids) > 0 {
			for _, id := range selector.Ids {
				cfg, ok := GetConfigByUUID(id)
				if ok {
					cfgs = append(cfgs, cfg)
				}
			}
		}

		if len(selector.Keys) > 0 {
			for _, key := range selector.Keys {
				cfg, ok := GetConfigByKey(key)
				if ok {
					cfgs = append(cfgs, cfg)
				}
			}
		}

		if len(selector.Labels) > 0 {
			cfgs1 := GetConfigByLabels(selector.Labels)
			if cfgs1 != nil {
				cfgs = append(cfgs, cfgs1...)
			}
		}
	}

	log.Tracef("selector:%v, get queues: %v", selector, cfgs)

	return cfgs
}

func GetConfigByLabels(labels map[string]interface{}) []*QueueConfig {

	cfgs := []*QueueConfig{}
	configs.Range(func(key, value interface{}) bool {
		v := value.(*QueueConfig)
		if v != nil {
			matched := false
			for x, y := range labels {
				if v.Labels != nil {
					z, ok := v.Labels[x]
					if ok {
						if util.ToString(z) == util.ToString(y) {
							matched = true
						}
					}
				}
			}
			if matched {
				cfgs = append(cfgs, v)
			}
		}
		return true
	})

	return cfgs
}

var adapters map[string]QueueAPI = map[string]QueueAPI{}

func RegisterDefaultHandler(h QueueAPI) {
	defaultHandler = h
}

func IniQueue(k *QueueConfig) {
	if k == nil || k.ID == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k)
	err := handler.Init(k.ID)
	if err != nil {
		panic(err)
	}
}

func Register(name string, h QueueAPI) {
	_, ok := adapters[name]
	if ok {
		panic(errors.Errorf("queue adapter with same name: %v already exists", name))
	}

	adapters[name] = h
	log.Debug("register queue adapter: ", name)
}

// TODO only update specify event, func(queueID)
var queueConfigListener = []func(cfg *QueueConfig){}

var cfgLock = sync.RWMutex{}

func RegisterQueueConfigChangeListener(l func(cfg *QueueConfig)) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	queueConfigListener = append(queueConfigListener, l)
}

var consumerConfigListener = []func(id string, configs map[string]*ConsumerConfig){}

func RegisterConsumerConfigChangeListener(l func(id string, configs map[string]*ConsumerConfig)) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()
	consumerConfigListener = append(consumerConfigListener, l)
}

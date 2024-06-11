/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"runtime"
	"sync"
	"time"
)

type ConsumerConfig struct {
	orm.ORMObjectBase

	Source           string `config:"source" json:"source,omitempty"`
	Group            string `config:"group" json:"group,omitempty"`
	Name             string `config:"name" json:"name,omitempty"`
	AutoResetOffset  string `config:"auto_reset_offset" json:"auto_reset_offset,omitempty"`
	AutoCommitOffset bool   `config:"auto_commit_offset" json:"auto_commit_offset,omitempty"`

	//don't add queue id to generated sliced consumer group
	SimpleSlicedGroup bool `config:"simple_sliced_group" json:"simple_sliced_group,omitempty"`

	FetchMinBytes     int   `config:"fetch_min_bytes" json:"fetch_min_bytes,omitempty"`
	FetchMaxBytes     int   `config:"fetch_max_bytes" json:"fetch_max_bytes,omitempty"`
	FetchMaxMessages  int   `config:"fetch_max_messages" json:"fetch_max_messages,omitempty"`
	FetchMaxWaitMs    int64 `config:"fetch_max_wait_ms" json:"fetch_max_wait_ms,omitempty"`
	ConsumeTimeoutInSeconds    int `config:"consume_timeout" json:"consume_timeout,omitempty"`
	EOFMaxRetryTimes  int   `config:"eof_max_retry_times" json:"eof_max_retry_times,omitempty"`
	EOFRetryDelayInMs int64 `config:"eof_retry_delay_in_ms" json:"eof_retry_delay_in_ms,omitempty"`

	ClientExpiredInSeconds int64 `config:"client_expired_in_seconds" json:"client_expired_in_seconds,omitempty"` //client acquires lock for this long
	fetchMaxWaitMs         time.Duration
	lastAccessTimestamp    *time.Time

	CommitLocker sync.Mutex
}

func (cfg *ConsumerConfig) Key() string {
	return getConsumerKey(cfg.Group , cfg.Name)
}

func (cfg *ConsumerConfig) KeepTouch() {
	t:=util.GetLowPrecisionCurrentTime()
	cfg.lastAccessTimestamp= &t
}
func (cfg *ConsumerConfig) GetLastTouchTime()*time.Time {
	return cfg.lastAccessTimestamp
}

func (cfg *ConsumerConfig) GetFetchMaxWaitMs() time.Duration {
	if cfg.fetchMaxWaitMs.Milliseconds() > 0 {
		return cfg.fetchMaxWaitMs
	}

	cfg.fetchMaxWaitMs = time.Duration(cfg.FetchMaxWaitMs) * time.Millisecond
	return cfg.fetchMaxWaitMs
}

func (cfg *ConsumerConfig) String() string {
	return fmt.Sprintf("group:%v,name:%v,id:%v,source:%v, simple:%v", cfg.Group, cfg.Name, cfg.ID, cfg.Source,cfg.SimpleSlicedGroup)
}

const ConsumerBucket = "queue_consumers"

func RegisterConsumer(queueID string, consumer *ConsumerConfig) (bool, error) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	ok, _ := kv.ExistsKey(ConsumerBucket, queueIDBytes)

	cfgs := map[string]*ConsumerConfig{}
	if ok {
		data, err := kv.GetValue(ConsumerBucket, queueIDBytes)
		if err != nil {
			panic(err)
		}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			panic(err)
		}
	}
	cfgs[consumer.Key()] = consumer
	kv.AddValue(ConsumerBucket, queueIDBytes, util.MustToJSONBytes(cfgs))

	TriggerChangeEvent(queueID, cfgs, false)

	return true, nil
}

func TriggerChangeEvent(queueID string, cfgs map[string]*ConsumerConfig, async bool) {
	if async {
		//async notify
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
						log.Error(v)
					}
				}
			}()
			for _, f := range consumerConfigListener {
				f(queueID, cfgs)
			}
		}()
	} else {
		for _, f := range consumerConfigListener {
			f(queueID, cfgs)
		}
	}
}

func RemoveAllConsumers(qConfig *QueueConfig) (bool, error) {
	v, ok := GetConsumerConfigsByQueueID(qConfig.ID)
	if ok {
		for k, v1 := range v {
			ok, err := RemoveConsumer(qConfig.ID, v1.Key())
			if err != nil {
				log.Error(err)
				return false, err
			}
			err = DeleteOffset(qConfig, v1)
			if err != nil {
				log.Errorf("delete consumer:%v %v, %v, %v, %v, %v", qConfig.ID, v1.Key(), ok, err, k, util.MustToJSON(v1))
				return false, err
			}
		}
	}
	err := kv.DeleteKey(ConsumerBucket, util.UnsafeStringToBytes(qConfig.ID))
	if err != nil {
		log.Error(err)
		return false, err
	}
	log.Debugf("success delete all consumers for queue:%v", qConfig.ID)
	return true, nil
}

func RemoveConsumer(queueID string, consumerKey string) (bool, error) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	ok, _ := kv.ExistsKey(ConsumerBucket, queueIDBytes)
	cfgs := map[string]*ConsumerConfig{}
	if ok {
		data, err := kv.GetValue(ConsumerBucket, queueIDBytes)
		if err != nil {
			return false, err
		}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			return false, err
		}
		delete(cfgs, consumerKey)
		err = kv.AddValue(ConsumerBucket, queueIDBytes, util.MustToJSONBytes(cfgs))
		if err != nil {
			return false, err
		}

		TriggerChangeEvent(queueID, cfgs, false)

		return true, nil
	}

	return false, errors.New("consumer not found")
}

func GetConsumerConfig(queueID, group, name string) (*ConsumerConfig, bool) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	cfgs := map[string]*ConsumerConfig{}
	data, err := kv.GetValue(ConsumerBucket, queueIDBytes)
	if err != nil {
		panic(err)
	}
	err = util.FromJSONBytes(data, &cfgs)
	if err != nil {
		panic(err)
	}
	if cfgs != nil {
		x, ok := cfgs[getConsumerKey(group,name)]
		return x, ok
	}

	return nil, false
}

func getConsumerKey(group,name string)string{
	return group+"-"+name
}

func NewConsumerConfig(queueID, group, name string) *ConsumerConfig {
	cfg := &ConsumerConfig{
		FetchMinBytes:          1,
		FetchMaxBytes:          20 * 1024 * 1024,
		FetchMaxMessages:       500,
		EOFRetryDelayInMs:      500,
		FetchMaxWaitMs:         10000,
		ConsumeTimeoutInSeconds:         60,
		EOFMaxRetryTimes:       10,
		ClientExpiredInSeconds: 60,
	}
	cfg.ID = util.MD5digest(fmt.Sprintf("%v_%v_%v", queueID, group, name))
	cfg.Source = "dynamic"
	cfg.Group = group
	cfg.Name = name
	return cfg
}

func GetOrInitConsumerConfig(queueID, group, name string) *ConsumerConfig {
	cfg, exists := GetConsumerConfig(queueID, group, name)
	if !exists || cfg == nil {
		cfg = NewConsumerConfig(queueID, group, name)
		cfg.Source = "dynamic"
		RegisterConsumer(queueID, cfg)
	}
	return cfg
}

func GetConsumerConfigsByQueueID(queueID string) (map[string]*ConsumerConfig, bool) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	cfgs := map[string]*ConsumerConfig{}
	data, err := kv.GetValue(ConsumerBucket, queueIDBytes)
	if err != nil {
		panic(err)
	}
	//TODO optimize performance
	err = util.FromJSONBytes(data, &cfgs)
	if err != nil {
		panic(err)
	}

	if cfgs != nil {
		return cfgs, len(cfgs) > 0
	}

	return nil, false
}

func GetConsumerConfigID(queueID, consumerID string) (*ConsumerConfig, bool) {
	m, ok := GetConsumerConfigsByQueueID(queueID)
	if ok {
		for _, v := range m {
			if v.ID == consumerID {
				return v, true
			}
		}
	}
	return nil, false
}

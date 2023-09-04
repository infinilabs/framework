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
	"time"
)

type ConsumerConfig struct {
	orm.ORMObjectBase

	Source           string `config:"source" json:"source,omitempty"`
	Group            string `config:"group" json:"group,omitempty"`
	Name             string `config:"name" json:"name,omitempty"`
	AutoResetOffset  string `config:"auto_reset_offset" json:"auto_reset_offset,omitempty"`
	AutoCommitOffset bool   `config:"auto_commit_offset" json:"auto_commit_offset,omitempty"`

	FetchMinBytes     int   `config:"fetch_min_bytes" json:"fetch_min_bytes,omitempty"`
	FetchMaxBytes     int   `config:"fetch_max_bytes" json:"fetch_max_bytes,omitempty"`
	FetchMaxMessages  int   `config:"fetch_max_messages" json:"fetch_max_messages,omitempty"`
	FetchMaxWaitMs    int64 `config:"fetch_max_wait_ms" json:"fetch_max_wait_ms,omitempty"`
	EOFRetryDelayInMs int64 `config:"eof_retry_delay_in_ms" json:"eof_retry_delay_in_ms,omitempty"`

	ClientExpiredInSeconds int64 `config:"client_expired_in_seconds" json:"client_expired_in_seconds,omitempty"` //client acquires lock for this long
	fetchMaxWaitMs         time.Duration
}

func (cfg *ConsumerConfig) Key() string {
	return cfg.Group + "-" + cfg.Name
}

func (cfg *ConsumerConfig) GetFetchMaxWaitMs() time.Duration {
	if cfg.fetchMaxWaitMs.Milliseconds() > 0 {
		return cfg.fetchMaxWaitMs
	}

	cfg.fetchMaxWaitMs = time.Duration(cfg.FetchMaxWaitMs) * time.Millisecond
	return cfg.fetchMaxWaitMs
}

func (cfg *ConsumerConfig) String() string {
	return fmt.Sprintf("group:%v,name:%v,id:%v,source:%v", cfg.Group, cfg.Name, cfg.ID, cfg.Source)
}

const consumerBucket = "queue_consumers"

func RegisterConsumer(queueID string, consumer *ConsumerConfig) (bool, error) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	ok, _ := kv.ExistsKey(consumerBucket, queueIDBytes)

	cfgs := map[string]*ConsumerConfig{}
	if ok {
		data, err := kv.GetValue(consumerBucket, queueIDBytes)
		if err != nil {
			panic(err)
		}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			panic(err)
		}
	}
	cfgs[consumer.Key()] = consumer
	kv.AddValue(consumerBucket, queueIDBytes, util.MustToJSONBytes(cfgs))

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

func RemoveAllConsumers(queueID string) (bool, error) {
	v, ok := GetConsumerConfigsByQueueID(queueID)
	if ok {
		for _, v := range v {
			RemoveConsumer(queueID, v.Key())
		}
	}
	kv.DeleteKey(consumerBucket, util.UnsafeStringToBytes(queueID))
	return true, nil
}

func RemoveConsumer(queueID string, consumerKey string) (bool, error) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	ok, _ := kv.ExistsKey(consumerBucket, queueIDBytes)
	cfgs := map[string]*ConsumerConfig{}
	if ok {
		data, err := kv.GetValue(consumerBucket, queueIDBytes)
		if err != nil {
			return false, err
		}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			return false, err
		}
		delete(cfgs, consumerKey)
		err = kv.AddValue(consumerBucket, queueIDBytes, util.MustToJSONBytes(cfgs))
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
	data, err := kv.GetValue(consumerBucket, queueIDBytes)
	if err != nil {
		panic(err)
	}
	err = util.FromJSONBytes(data, &cfgs)
	if err != nil {
		panic(err)
	}
	if cfgs != nil {
		x, ok := cfgs[group+"-"+name]
		return x, ok
	}

	return nil, false
}

func NewConsumerConfig(queueID,group, name string) *ConsumerConfig {
	cfg := &ConsumerConfig{
		FetchMinBytes:          1,
		FetchMaxBytes:          20 * 1024 * 1024,
		FetchMaxMessages:       500,
		EOFRetryDelayInMs:      500,
		FetchMaxWaitMs:         10000,
		ClientExpiredInSeconds: 60,
	}
	cfg.ID = util.MD5digest(fmt.Sprintf("%v_%v_%v",queueID,group,name))
	cfg.Source = "dynamic"
	cfg.Group = group
	cfg.Name = name
	return cfg
}

func GetOrInitConsumerConfig(queueID, group, name string) *ConsumerConfig {
	cfg, exists := GetConsumerConfig(queueID, group, name)
	if !exists || cfg == nil {
		cfg = NewConsumerConfig(queueID,group,name)
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
	data, err := kv.GetValue(consumerBucket, queueIDBytes)
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

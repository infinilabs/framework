// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"context"
	"errors"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type QueueConfig struct {
	ID      string      `config:"id" json:"id,omitempty"`     //uuid for each queue
	Name    string      `config:"name" json:"name,omitempty"` //unique name of each queue
	Source  string      `config:"source" json:"source,omitempty"`
	Codec   string      `config:"codec" json:"codec,omitempty"`
	Type    string      `config:"type" json:"type,omitempty"`
	Created string      `config:"created" json:"created,omitempty"`
	Labels  util.MapStr `config:"label" json:"label,omitempty"`
	sync.RWMutex
}

var queueConfigPool = sync.Pool{
	New: func() interface{} {
		return new(QueueConfig)
	},
}

func (q *QueueConfig) UpdateLabel(key string,val interface{}) {
	q.Lock()
	defer q.Unlock()
	q.Labels[key] = val
}
func (q *QueueConfig) ReplaceLabels(labels util.MapStr) {
	q.Lock()
	defer q.Unlock()
	q.Labels=labels
}

func AcquireQueueConfig() *QueueConfig {

	cfg := queueConfigPool.Get().(*QueueConfig)
	cfg.ID = ""
	cfg.Name = ""
	cfg.Type = ""
	cfg.Codec = ""
	cfg.Source = ""
	cfg.Labels = util.MapStr{}
	return cfg
}

func ReturnQueueConfig(cfg *QueueConfig) {
	cfg.ID = ""
	cfg.Name = ""
	cfg.Type = ""
	cfg.Codec = ""
	cfg.Source = ""
	cfg.Labels = nil
	queueConfigPool.Put(cfg)
}

const queueConfigBucket = "queue_configs"

func addCfgToCache(cfg *QueueConfig) {
	configs.Store(cfg.Name, cfg)
	idConfigs.Store(cfg.ID, cfg)
}

func RegisterConfig(cfg *QueueConfig) (preExists bool, err error) {

	if global.Env().IsDebug{
		log.Info("register queue config:", cfg.ID, ",", cfg.Name, ",", cfg.Labels)
	}

	if cfg.Name == "" {
		if cfg.ID != "" {
			cfg.Name = cfg.ID
		}
	}

	//assert basic info for queue
	if cfg.ID == "" {
		if cfg.Name == "" {
			panic("queue name is empty")
		}
		cfg.ID = util.MD5digest(cfg.Name)
	}

	cfg.Created = time.Now().String()

	log.Debug("init new queue config:", cfg.ID, ",", cfg.Name)

	addCfgToCache(cfg)

	//persist to kv	store
	err = kv.AddValue(queueConfigBucket, []byte(cfg.ID), util.MustToJSONBytes(cfg))
	if err != nil {
		panic(err)
	}

	//async notify
	task.RunWithContext("queue_config_changed_callback", func(ctx context.Context) error {
		v:=ctx.Value("cfg")
		if v!=nil{
			x,ok:=v.(*QueueConfig)
			if ok{
				//notify all listeners
				for _, f := range queueConfigListener {
					f(x)
				}
			}
		}
		return nil
	},context.WithValue(context.Background(), "cfg", cfg))

	return false, nil

}

func RemoveConfig(cfg *QueueConfig) bool {
	if cfg == nil {
		panic(errors.New("queue config can't be nil"))
	}
	idConfigs.Delete(cfg.ID)
	configs.Delete(cfg.Name)
	return true
}

func IsConfigExists(key string) bool {
	_, ok := configs.Load(key)
	return ok
}

func GetOrInitConfig(key string) *QueueConfig {
	if key == "" {
		panic(errors.New("queue config key can't be empty"))
	}
	return AdvancedGetOrInitConfig("", key, nil)
}

func SmartGetOrInitConfig(cfg *QueueConfig) *QueueConfig {
	if cfg.ID!=""{
		v,_:=GetConfigByUUID(cfg.ID)
		return v
	}
	return AdvancedGetOrInitConfig(cfg.Type,cfg.Name,cfg.Labels)
}

func AdvancedGetOrInitConfig(queueType, key string, labels map[string]interface{}) *QueueConfig {
	cfg, exists := SmartGetConfig(key)
	if !exists || cfg == nil {
		cfg = &QueueConfig{}
		cfg.Type = queueType
		cfg.ID = util.MD5digest(key)
		cfg.Name = key
		cfg.Source = "dynamic"
		if labels != nil {
			cfg.Labels = labels
		}
		_, err := RegisterConfig(cfg)
		if err != nil {
			panic(err)
		}
		return cfg

	} else {
		//TODO: check if labels changed, then replace the config
		if  labelChanged(labels, cfg.Labels){
			cfg.Name = key
			cfg.Labels = labels
			RegisterConfig(cfg)
		}
	}
	return cfg
}

func labelChanged(newLabels, oldLabels map[string]interface{}) bool {
	if len(newLabels) > len(oldLabels){
		return true
	}

	if len(oldLabels)==0&&len(newLabels)>0{
		return true
	}

	for k, v := range newLabels {
		if ov, ok := oldLabels[k]; !ok || ov != v {
			return true
		}
	}

	return false
}

func SmartGetConfig(keyOrID string) (*QueueConfig, bool) {
	q, ok := GetConfigByKey(keyOrID)
	if !ok {
		q, ok = GetConfigByUUID(keyOrID)
	}
	return q, ok
}

func GetConfigByKey(key string) (*QueueConfig, bool) {
	v, ok := configs.Load(key)
	if ok {
		cfg, ok1 := v.(*QueueConfig)
		return cfg, ok1
	}
	return nil, false
}

func GetConfigByUUID(id string) (*QueueConfig, bool) {
	x, ok := idConfigs.Load(id)
	v, ok := x.(*QueueConfig)

	//try get from kv
	if !ok {
		vbytes, err := kv.GetValue(queueConfigBucket, []byte(id))
		if err != nil {
			panic(err)
		}

		if vbytes != nil {
			cfg := &QueueConfig{}
			err := util.FromJSONBytes(vbytes, cfg)
			if err != nil {
				panic(err)
			}
			if cfg.ID != "" && cfg.ID == id { //verify config
				if global.Env().IsDebug {
					log.Debug("get queue config from kv:", cfg.ID, ",", string(vbytes))
				}
				//save to local cache
				addCfgToCache(cfg)
				return cfg, true
			}
		}
	}

	return v, ok
}

func GetAllConfigBytes() []byte {
	cfgs := GetAllConfigs()
	return util.MustToJSONBytes(cfgs)
}

func GetAllConfigs() map[string]*QueueConfig {
	cfgs := map[string]*QueueConfig{}
	configs.Range(func(key, value interface{}) bool {
		cfg := value.(*QueueConfig)
		if cfg != nil {
			cfgs[cfg.Name] = cfg
		}
		return true
	})
	return cfgs
}

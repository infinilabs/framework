/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queue

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type Context struct {
	//Metadata   map[string]interface{} `config:"metadata" json:"metadata"`
	NextOffset string                  `config:"next_offset" json:"next_offset"`
	InitOffset string 				   `config:"init_offset" json:"init_offset"`
}

type Message struct {
	Offset string `config:"offset" json:"offset"`
	Size int64    `config:"size" json:"size"`
	Data []byte   `config:"data" json:"data"`
}

type QueueAPI interface {
	Init(string) error
	Push(string, []byte) error
	Pop(string, time.Duration) (data []byte, timeout bool)
	//part means the sequence id of the queue, offset is within the part, count means how many messages will be fetching
	Consume(queue,consumer,offsetStr string,count int,timeout time.Duration) ( *Context, []Message,bool,error)
	Close(string) error
	Depth(string) int64
	GetQueues() []string
}

var defaultHandler QueueAPI
var handlers map[string]QueueAPI = map[string]QueueAPI{}

type Config struct {
	Source   string                 `config:"source" json:"source,omitempty"`
	Id       string                 `config:"id" json:"id,omitempty"`   //uuid for each queue
	Name     string                 `config:"name" json:"name,omitempty"` //unique name of each queue
	Codec    string                 `config:"codec" json:"codec,omitempty"`
	Type     string                 `config:"type" json:"type,omitempty"`
	Labels map[string]interface{}   `config:"label" json:"label"`
}

type ConsumerConfig struct {
	Group   string                  `config:"group" json:"group,omitempty"`
	Name     string                 `config:"name" json:"name,omitempty"`
	AutoReset     string       `config:"auto_offset_reset" json:"auto_offset_reset,omitempty"`
	AutoCommit     bool              `config:"auto_commit" json:"auto_commit,omitempty"`
}

func getHandler(queueID string) QueueAPI {
	handler, ok := handlers[queueID]
	if ok {
		return handler
	}
	return defaultHandler
}

func Push(k *Config, v []byte) error {
	var err error = nil
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k.Id)
	if handler != nil {
		err = handler.Push(k.Id, v)
		if err == nil {
			stats.Increment("queue."+k.Id, "push")
			return nil
		}
		stats.Increment("queue."+k.Id, "push_error")
		return err
	}
	panic(errors.New("handler is not registered"))
}

var pauseMsg = errors.New("queue was paused to read")

var configs = map[string]*Config{}
var idConfigs = map[string]*Config{}
var cfgLock = sync.RWMutex{}

func RegisterConfig(queueKey string, cfg *Config) (bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	_, ok := configs[queueKey]
	if ok {
		return true, errors.New("config exists")
	} else {
		//init empty id
		if cfg.Id == "" {
			cfg.Id = util.GetUUID()
		}
		idConfigs[cfg.Id] = cfg
		configs[queueKey] = cfg

		//async notify
		go func() {
			for _,f:=range listener{
				f()
			}
		}()

		return false, nil
	}
}

func IsConfigExists(key string) bool {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	_, ok := configs[key]
	return ok
}

func GetOrInitConfig(key string) (*Config) {
	cfg,exists:=GetConfig(key)
	if !exists{
		_, ok := configs[key]
		if !ok{
			cfg=&Config{}
			cfg.Id=util.GetUUID()
			cfg.Name= key
			cfg.Source="dynamic"
			RegisterConfig(key,cfg)
		}
	}
	return cfg
}

func GetConfig(key string) (*Config, bool) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	v, ok := configs[key]
	return v, ok
}

func GetConfigByUUID(id string) (*Config, bool) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	v, ok := idConfigs[id]
	return v, ok
}

func GetAllConfigBytes()[]byte {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return util.MustToJSONBytes(configs)
}
func GetAllConfigs() map[string]*Config {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return configs
}

func Pop(k *Config) ([]byte, error) {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k.Id)
	if handler != nil {
		if pausedReadQueue.Contains(k) {
			return nil, pauseMsg
		}

		o, timeout := handler.Pop(k.Id, -1)
		if !timeout {
			stats.Increment("queue."+k.Id, "pop")
			return o, nil
		}
		stats.Increment("queue."+k.Id, "pop_timeout")
		return o, errors.New("timeout")
	}
	panic(errors.New("handler is not registered"))
}

func Consume(k *Config,consumer,offsetStr string,count int,timeout time.Duration) ( ctx *Context, messages []Message,isTimeout bool,err error) {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	//data:=strings.Split(offsetStr,",")
	//var part,offset int64
	//part,_=util.ToInt64(data[0])
	//offset,_=util.ToInt64(data[1])

	messages=[]Message{}
	handler := getHandler(k.Id)
	if handler != nil {
		if pausedReadQueue.Contains(k) {
			return ctx,messages,isTimeout, pauseMsg
		}

		ctx,messages, isTimeout,err = handler.Consume(k.Id,consumer,offsetStr,count, timeout)

		if !isTimeout {
			stats.Increment("queue."+k.Id, "pop")
			return ctx, messages,isTimeout,err
		}
		stats.Increment("queue."+k.Id, "pop_timeout")
		return ctx,messages,isTimeout, errors.New("timeout")
	}
	panic(errors.New("handler is not registered"))
}

func PopTimeout(k *Config, timeoutInSeconds time.Duration) (data []byte, timeout bool, err error) {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	if timeoutInSeconds < 1 {
		timeoutInSeconds = 5
	}

	handler := getHandler(k.Id)

	if handler != nil {

		if pausedReadQueue.Contains(k) {
			return nil, false, pauseMsg
		}

		o, timeout := handler.Pop(k.Id, timeoutInSeconds)
		if !timeout {
			stats.Increment("queue."+k.Id, "pop")
		}
		stats.Increment("queue."+k.Id, "pop_timeout")
		return o, timeout, nil
	}
	panic(errors.New("handler is not registered"))
}

func Close(k *Config) error {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k.Id)
	if handler != nil {
		o := handler.Close(k.Id)
		stats.Increment("queue."+k.Id, "close")
		return o
	}
	stats.Increment("queue."+k.Id, "close_error")
	panic(errors.New("handler is not closed"))
}

func getCommitKey(k *Config, consumer ConsumerConfig)string  {
	return fmt.Sprintf("%v-%v",k.Id,consumer.Group)
}

const bucket ="queue_consumer_commit_offset"
func GetOffset(k *Config, consumer ConsumerConfig) (string,error) {

	bytes,err:=kv.GetValue(bucket,util.UnsafeStringToBytes(getCommitKey(k,consumer)))
	if err!=nil{
		log.Error(err)
	}

	if bytes!=nil&&len(bytes)>0{
		return string(bytes),nil
	}

	return "0,0",nil
}

func CommitOffset(k *Config, consumer ConsumerConfig, offset string)(bool,error) {

	if global.Env().IsDebug{
		log.Tracef("queue [%v] [%v][%v] commit offset:%v",k.Id,consumer.Group,consumer.Name,offset)
	}

	err:=kv.AddValue(bucket,util.UnsafeStringToBytes(getCommitKey(k,consumer)),[]byte(offset))
	if err!=nil{
		return false, err
	}

	return true,nil
}

func Depth(k *Config) int64 {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k.Id)
	if handler != nil {
		o := handler.Depth(k.Id)
		stats.Increment("queue."+k.Id, "call_depth")
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
			stats.Increment("queue."+q, "get_queues")
			result = append(result, o...)
			results[q] = result
		}
	}
	return results
}

func GetQueuesFilterByLabel(labels map[string]interface{}) []*Config {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfgs:=[]*Config{}

	for _,v:=range configs{
		notMatch:=false
		for x,y:=range labels{
			z,ok:=v.Labels[x]
			if ok{
				if util.ToString(z)!=util.ToString(y){
					notMatch=true
				}
			}else{
				notMatch=true
			}
		}
		if !notMatch{
			cfgs=append(cfgs,v)
		}
	}
	return cfgs
}

var pausedReadQueue = hashset.New()
var pauseChan map[string]chan bool = map[string]chan bool{}
var pauseCount = map[string]int{}
var pauseLock sync.Mutex

func PauseRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pauseCount[k] = 0
	pauseChan[k] = make(chan bool)
	pausedReadQueue.Add(k)
}

func ResumeRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pausedReadQueue.Remove(k)
	size := pauseCount[k]
	for i := 0; i < size; i++ {
		pauseChan[k] <- true
	}
	log.Debugf("queue: %s was resumed, signal: %v", k, size)
}

var adapters map[string]QueueAPI = map[string]QueueAPI{}

func RegisterDefaultHandler(h QueueAPI) {
	defaultHandler = h
}

func IniQueue(k *Config, typeOfAdaptor string) {
	if k==nil||k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler:=getHandler(typeOfAdaptor)
	handlers[k.Id] = handler
	err:=handler.Init(k.Id)
	if err!=nil{
		panic(err)
	}
}

func Register(name string, h QueueAPI) {
	_, ok := adapters[name]
	if ok {
		panic(errors.Errorf("queue handler with same name: %v already exists", name))
	}

	adapters[name] = h
	log.Debug("register queue handler: ", name)
}

var listener =[]func(){}
func RegisterConfigChangeListener(l func()){
	cfgLock.Lock()
	defer cfgLock.Unlock()
	listener=append(listener,l)
}

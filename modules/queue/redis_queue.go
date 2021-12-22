/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
	"sync"
	"time"
)

type RedisModule struct {
	client  *redis.Client
	config RedisConfig
}

type RedisConfig struct {
	Enabled     bool `config:"enabled"`
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Username string `config:"username"`
	Password string `config:"password"`
	PoolSize int `config:"pool_size"`
	Db       int    `config:"db"`
}

func (module *RedisModule) Name() string {
	return "redis"
}

func (module *RedisModule) Setup(cfg *config.Config) {
	module.config = RedisConfig{
		Db: 0,
		PoolSize: 1000,
	}
	ok, err := env.ParseConfig("redis", &module.config)
	if ok && err != nil {
		panic(err)
	}
}

var ctx = context.Background()

type RedisQueue struct {
	client *redis.Client
	pubsub map[string]int
}

func (module *RedisQueue) Push(k string, v []byte) error {
	module.Init(k)
	_,err:=module.client.LPush(ctx,k,v).Result()
	return err
}


func (module *RedisQueue) Init(k string) error{
	_,ok:= module.pubsub[k]
	if ok{
		return nil
	}
	module.pubsub[k]=1
	return nil
}

var lock sync.RWMutex

func (module *RedisQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {

	lock.Lock()
	defer lock.Unlock()

	if timeoutDuration > 0 {
		v,err:= module.client.BLPop(ctx,timeoutDuration,k).Result()
		if err!=nil{
			return nil,true
		}

		if len(v)==1{
			return []byte(v[0]),false
		}

	} else {
		v,err:=module.client.LPop(ctx,k).Result()
		if err!=nil{
			return nil,true
		}
		return []byte(v),false
	}
	return nil,true
}

func (module *RedisQueue) Close(k string) error {
	return module.client.Subscribe(ctx,k).Close()
}

func (module *RedisQueue) Depth(k string) int64 {
	c,err:=module.client.LLen(ctx,k).Result()
	if err!=nil{
		return -1
	}
	return c
}

func (module *RedisQueue) GetQueues() []string {
	result := []string{}
	for k,_:=range module.pubsub{
		result=append(result,k)
	}
	return result
}


func (module *RedisModule) Start() error {
	if !module.config.Enabled{
		return nil
	}

	module.client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%v", module.config.Host, module.config.Port),
		Username: module.config.Username,
		Password: module.config.Password,
		PoolSize: module.config.PoolSize,
		DB:       module.config.Db,
	})

	_, err := module.client.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}

	handler:=&RedisQueue{client: module.client,pubsub: map[string]int{}}
	queue.Register("redis",handler)

	return nil
}

func (module *RedisModule) Stop() error {
	if !module.config.Enabled{
		return nil
	}

	module.client.Close()
	return nil
}

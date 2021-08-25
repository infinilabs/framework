/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"context"
	"fmt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
	"github.com/go-redis/redis"
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
	pubsub map[string]*redis.PubSub
}

func (module *RedisQueue) Push(k string, v []byte) error {
	module.getChannel(k)
	_,err:=module.client.Publish(ctx,k,v).Result()
	return err
}


func (module *RedisQueue) getChannel(k string)*redis.PubSub{
	v,ok:= module.pubsub[k]
	if ok{
		return v
	}
	v=module.client.PSubscribe(ctx,k)
	module.pubsub[k]=v
	return v
}

var lock sync.RWMutex

func (module *RedisQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {

	lock.Lock()
	defer lock.Unlock()

	pub:=module.getChannel(k)
	if timeoutDuration > 0 {
		to := time.NewTimer(timeoutDuration)
		for {
			to.Reset(timeoutDuration)
			select {
			case m := <-pub.Channel():
				if m==nil||len(m.Payload)==0{
					return nil,true
				}
				return []byte(m.Payload),false
			case <-to.C:
				return nil,true
			}
		}
	} else {
		m := <-pub.Channel()
		if m==nil||len(m.Payload)==0{
			return nil,true
		}
		return []byte(m.Payload),false
	}
}

func (module *RedisQueue) Close(k string) error {
	return module.client.Subscribe(ctx,k).Close()
}

func (module *RedisQueue) Depth(k string) int64 {
	//module.client.
	return -1
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

	handler:=&RedisQueue{client: module.client,pubsub: map[string]*redis.PubSub{}}
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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
	log "github.com/cihub/seelog"
	"github.com/go-redis/redis"
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
	Password string `config:"password"`
	Db       int    `config:"db"`
}

func (module *RedisModule) Name() string {
	return "redis"
}

func (module *RedisModule) Setup(cfg *config.Config) {
	module.config = RedisConfig{
		Db: 0,
	}
	ok, err := env.ParseConfig("redis", &module.config)
	if ok && err != nil {
		panic(err)
	}
}

type RedisQueue struct {
	client *redis.Client
}

func (module *RedisQueue) Push(k string, v []byte) error {
	_,err:=module.client.Publish(k,v).Result()
	return err
}

func (module *RedisQueue) Pop(k string, timeoutDuration time.Duration) (data []byte,timeout bool) {
	if timeoutDuration > 0 {
		msg,err:=module.client.PSubscribe(k).ReceiveTimeout(timeoutDuration)
		if err!=nil{
			log.Error(err)
			return msg.([]byte),true
		}
		return msg.([]byte),false
	} else {
		msg,err:=module.client.PSubscribe(k).Receive()
		if err!=nil{
			log.Error(err)
			return msg.([]byte),true
		}
		return msg.([]byte),false
	}
}

func (module *RedisQueue) Close(k string) error {
	return module.client.Subscribe(k).Close()
}

func (module *RedisQueue) Depth(k string) int64 {
	//module.client.
	return -1
}

func (module *RedisQueue) GetQueues() []string {
	result := []string{}
	return result
}


func (module *RedisModule) Start() error {
	if !module.config.Enabled{
		return nil
	}

	module.client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%v", module.config.Host, module.config.Port),
		Password: module.config.Password,
		DB:       module.config.Db,
	})

	_, err := module.client.Ping().Result()
	if err != nil {
		panic(err)
	}

	handler:=&RedisQueue{client: module.client}
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

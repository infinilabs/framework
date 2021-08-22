/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
)

type QueueModule struct {
	configs []QueueConfig
}

type QueueConfig struct {
	Name string `config:"name"`
	Type string `config:"type"`
}

func (module *QueueModule) Name() string {
	return "queue"
}

func (module *QueueModule) Setup(cfg *config.Config) {
	module.configs=[]QueueConfig{}
	ok,err:=env.ParseConfig("queue", &module.configs)
	if ok&&err!=nil{
		panic(err)
	}
}

func (module *QueueModule) Start() error {

	if module.configs!=nil&&len(module.configs)>0{
		for _,v:=range module.configs{
			queue.IniQueue(v.Name,v.Type)
		}
	}

	return nil
}

func (module *QueueModule) Stop() error {
	return nil
}

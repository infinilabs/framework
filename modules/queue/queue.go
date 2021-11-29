/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
)

type QueueModule struct {
	configs []queue.Config
}



func (module *QueueModule) Name() string {
	return "queue"
}

func (module *QueueModule) Setup(cfg *config.Config) {
	configs := []queue.Config{}
	ok, err := env.ParseConfig("queue", &module.configs)
	if ok && err != nil {
		panic(err)
	}

	//load configs from local metadata

	//load configs from remote elasticsearch

	for _,v:=range configs{
		v.Source="file"
		if v.Id==""{
			v.Id=v.Name
		}
		queue.RegisterConfig(v.Name,&v)
	}
}

func (module *QueueModule) Start() error {

	//load configs from local file
	cfgs:=queue.GetAllConfigs()

	if cfgs != nil && len(cfgs) > 0 {
		for _, v := range cfgs {
			if v.Id==""{
				v.Id=v.Name
			}
			queue.IniQueue(v.Id, v.Type)
		}
	}

	return nil
}

func (module *QueueModule) Stop() error {

	//persist configs to local store

	//persist queue information
	return nil
}

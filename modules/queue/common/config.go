/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	log "github.com/cihub/seelog"
)

//Init queue metadata
func InitQueueMetadata() {

	//load configs from static config
	configs := []queue.QueueConfig{}
	ok, err := env.ParseConfig("queue", &configs)
	if ok && err != nil {
		panic(err)
	}

	for _,v:=range configs{
		v.Source="file"
		if v.Id==""{
			v.Id=v.Name
		}
		queue.RegisterConfig(v.Name,&v)
	}

	//load configs from local metadata
	if util.FileExists(GetLocalQueueConfigPath()){
		data,err:=util.FileGetContent(GetLocalQueueConfigPath())
		if err!=nil{
			panic(err)
		}

		cfgs := map[string]*queue.QueueConfig{}
		err=util.FromJSONBytes(data,&cfgs)
		if err!=nil{
			panic(err)
		}

		for _,v:=range cfgs{
			if v.Id==""{
				v.Id=v.Name
			}
			log.Debugf("init config:%v, type:%v",v.Name,v.Type)
			queue.RegisterConfig(v.Name,v)
		}
	}


	//load queue information from directory

	//load configs from remote elasticsearch


	//register queue listener
	queue.RegisterQueueConfigChangeListener(func(v *queue.QueueConfig) {
		PersistQueueMetadata()
	})

}

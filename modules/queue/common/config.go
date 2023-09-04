/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/queue"
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
		if v.ID ==""{
			v.ID =v.Name
		}
		queue.RegisterConfig(&v)
	}

	//register queue listener
	queue.RegisterQueueConfigChangeListener(func(v *queue.QueueConfig) {
		PersistQueueMetadata()
	})

}

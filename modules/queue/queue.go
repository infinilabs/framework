/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"os"
	"path"
	"sync"
)

type QueueModule struct {
}

func (module *QueueModule) Name() string {
	return "queue"
}

func (module *QueueModule) Setup(cfg *config.Config) {
	configs := []queue.Config{}
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
	if util.FileExists(getPath()){
		data,err:=util.FileGetContent(getPath())
		if err!=nil{
			panic(err)
		}

		cfgs:=map[string]*queue.Config{}
		err=util.FromJSONBytes(data,&cfgs)
		if err!=nil{
			panic(err)
		}

		for _,v:=range cfgs{
			if v.Id==""{
				v.Id=v.Name
			}
			queue.RegisterConfig(v.Name,v)
		}
	}

	//load configs from remote elasticsearch


	//register listener
	queue.RegisterConfigChangeListener(func() {
		persistMetadata()
	})
}

func (module *QueueModule) Start() error {

	//load configs from local file
	cfgs:=queue.GetAllConfigs()

	if cfgs != nil && len(cfgs) > 0 {
		for _, v := range cfgs {
			if v.Id==""{
				v.Id=v.Name
			}
			queue.IniQueue(v, v.Type)
		}
	}

	return nil
}

func getPath() string {
	os.MkdirAll(path.Join(global.Env().GetDataDir(),"queue"),0755)
	return path.Join(global.Env().GetDataDir(),"queue","configs")
}

func (module *QueueModule) Stop() error {
	persistMetadata()
	return nil
}

var locker sync.RWMutex
func persistMetadata()  {
	locker.Lock()
	defer locker.Unlock()

	//persist configs to local store
	bytes:=queue.GetAllConfigBytes()
	_,err:=util.FilePutContentWithByte(getPath(),bytes)
	if err!=nil{
		panic(err)
	}
}

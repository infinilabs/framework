/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"os"
	"path"
	"sync"
)


func GetLocalQueueConfigPath() string {
	os.MkdirAll(path.Join(global.Env().GetDataDir(),"queue"),0755)
	return path.Join(global.Env().GetDataDir(),"queue","configs")
}

var persistentLocker sync.RWMutex
func PersistQueueMetadata()  {
	persistentLocker.Lock()
	defer persistentLocker.Unlock()

	//persist configs to local store
	bytes:=queue.GetAllConfigBytes()
	_,err:=util.FilePutContentWithByte(GetLocalQueueConfigPath(),bytes)
	if err!=nil{
		panic(err)
	}
}
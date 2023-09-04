/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/lib/bytebufferpool"
	"sync"
)

type Message struct {
	Payload *bytebufferpool.ByteBuffer
	Context Context
}

type Context struct {
	WriteFile           string             `json:"write_file_path"`
	WriteFileNum        int64             `json:"write_file_num"`
}

type EventType string

const WriteComplete = EventType("WriteComplete")
const ReadComplete = EventType("ReadComplete")

type Event struct {
	Queue   string
	Type    EventType
	FileNum int64
}

type EventHandler func(event Event)error

var handlers =[]EventHandler{}
var locker =sync.RWMutex{}

func RegisterEventListener(handler EventHandler){
	locker.Lock()
	defer locker.Unlock()
	handlers =append(handlers,handler)
}

func Notify(queue string, eventType EventType,fileNum int64)  {

	if global.Env().IsDebug{
		log.Tracef("notify on queue: %v, type: %v, segment: %v",queue,eventType,fileNum)
	}

	event:= Event{
		Queue: queue,
		Type: eventType,
		FileNum: fileNum,
	}

	for _,v:=range handlers {
		v(event)
	}
}


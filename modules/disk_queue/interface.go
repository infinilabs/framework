/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/lib/bytebufferpool"
	log "github.com/cihub/seelog"
	"sync"
	"time"
)

// BackendQueue represents the behavior for the secondary message
// storage system
type BackendQueue interface {
	Put([]byte) error
	ReadChan() <-chan []byte // this is expected to be an *unbuffered* channel
	Close() error
	Delete() error
	Depth() int64
	LatestOffset() string
	ReadContext() Context

	Consume(consumer string,part,readPos int64,messageCount int,timeout time.Duration) (ctx *queue.Context,messages []queue.Message, isTimeout bool, err error)
	Empty() error
}

type Message struct {
	Payload *bytebufferpool.ByteBuffer
	Context Context
}

type Context struct {
	//Metadata       map[string]interface{} `json:"metadata"`

	WriteFile           string             `json:"write_file_path"`

	//Depth          int64                  `json:"queue_depth"`
	//PartitionID    int64                  `json:"partition_id"`

	//MinFileNum        int64               `json:"min_file_num"`
	WriteFileNum        int64             `json:"write_file_num"`
	//NextReadOffset int64                  `json:"next_read_offset"`
	//MaxLength      int64                  `json:"max_length"`
}

//func AcquireMessage()*Message  {
//
//}

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


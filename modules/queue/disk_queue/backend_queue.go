package queue

import (
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
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
	ReadContext() Context

	Consume(consumer string,part,readPos int64,messageCount int,timeout time.Duration) (ctx *queue.Context,messages []util.MapStr, isTimeout bool, err error)
	Empty() error
}

type Message struct {
	Payload *bytebufferpool.ByteBuffer
	Context Context
}

type Context struct {
	Metadata       map[string]interface{} `json:"metadata"`
	File           string                 `json:"file_path"`
	Depth          int64                  `json:"queue_depth"`
	PartitionID    int64                  `json:"partition_id"`

	MinFileNum        int64               `json:"min_file_num"`
	FileNum        int64                  `json:"file_num"`
	NextReadOffset int64                  `json:"next_read_offset"`
	MaxLength      int64                  `json:"max_length"`
}

//func AcquireMessage()*Message  {
//
//}

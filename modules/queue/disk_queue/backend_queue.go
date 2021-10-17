package queue

import "infini.sh/framework/lib/bytebufferpool"

// BackendQueue represents the behavior for the secondary message
// storage system
type BackendQueue interface {
	Put([]byte) error
	ReadChan() <-chan []byte // this is expected to be an *unbuffered* channel
	Close() error
	Delete() error
	Depth() int64
	ReadContext() Context
	Empty() error
}

type Message struct {
	Payload *bytebufferpool.ByteBuffer
	Context Context
}

type Context struct {
	File string  `json:"file_path"`
	Depth int64 `json:"queue_depth"`
	PartitionID int64 `json:"partition_id"`
	FileNum int64 `json:"file_num"`
	NextReadOffset int64 `json:"next_read_offset"`
	MaxLength int64 `json:"max_length"`
}

//func AcquireMessage()*Message  {
//
//}

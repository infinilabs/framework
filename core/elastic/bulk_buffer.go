package elastic

import (
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"sync"
)

type BulkBuffer struct {
	ID         string
	Queue      string
	Buffer     *bytebufferpool.ByteBuffer
	MessageIDs []string
	StatusCode map[int]int
}

var pool1 = bytebufferpool.NewPool(5*1024*1024, 50*1024*1024)

var bulkBufferPool = &sync.Pool{
	New: func() interface{} {
		v := new(BulkBuffer)
		v.ID = util.GetUUID()
		v.Reset()
		return v
	},
}

func AcquireBulkBuffer() *BulkBuffer {
	buff := bulkBufferPool.Get().(*BulkBuffer)
	buff.Buffer = pool1.Get("bulk_buffer")
	buff.Reset()
	return buff
}

func ReturnBulkBuffer(item *BulkBuffer) {
	item.Reset()
	pool1.Put("bulk_buffer", item.Buffer)
	bulkBufferPool.Put(item)
}

func (receiver *BulkBuffer) WriteByteBuffer(data []byte) {
	if data != nil && len(data) > 0 {
		receiver.Buffer.Write(data)
	}
}

func (receiver *BulkBuffer) WriteStringBuffer(data string) {
	if data != "" && len(data) > 0 {
		receiver.Buffer.WriteString(data)
	}
}

func (receiver *BulkBuffer) Add(id string, data []byte) {
	if data != nil && len(data) > 0 && len(id) != 0 {
		receiver.MessageIDs = append(receiver.MessageIDs, id)
		receiver.Buffer.Write(data)
	}
}

func (receiver *BulkBuffer) GetMessageCount() int {
	return len(receiver.MessageIDs)
}

func (receiver *BulkBuffer) GetMessageSize() int {
	return receiver.Buffer.Len()
}

func (receiver *BulkBuffer) WriteMessageID(id string) {
	if len(id) != 0 {
		receiver.MessageIDs = append(receiver.MessageIDs, id)
	}
}

func (receiver *BulkBuffer) GetMessageStatus(non2xxOnly bool) map[string]int {
	status := map[string]int{}
	for x, id := range receiver.MessageIDs {
		if non2xxOnly && (receiver.StatusCode[x] == 200 || receiver.StatusCode[x] == 201) {
			continue
		}
		status[id] = receiver.StatusCode[x]
	}
	return status
}

func (receiver *BulkBuffer) Reset() {
	if receiver.Buffer != nil {
		receiver.Buffer.Reset()
	}
	receiver.Queue = ""
	receiver.MessageIDs = receiver.MessageIDs[:0]
	receiver.StatusCode = map[int]int{}
}

func (receiver *BulkBuffer) SetResponseStatus(i int, status int) {
	receiver.StatusCode[i] = status
}

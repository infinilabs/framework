package elastic

import (
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"sync"
)

type BulkBuffer struct {
	ID         string
	Queue      string
	bytesBuffer     *bytebufferpool.ByteBuffer
	MessageIDs []string
}

var bulkBufferPool = &sync.Pool{
	New: func() interface{} {
		v := new(BulkBuffer)
		v.ID = util.ToString(util.GetIncrementID("bulk_buffer"))
		v.Reset()
		return v
	},
}

func AcquireBulkBuffer() *BulkBuffer {
	buff := bulkBufferPool.Get().(*BulkBuffer)
	buff.bytesBuffer = bytebufferpool.Get("bulk_request_docs")
	buff.Reset()
	return buff
}

func ReturnBulkBuffer(item *BulkBuffer) {
	item.Reset()
	if item.bytesBuffer!=nil{
		//bytebufferpool.Put("bulk_request_docs",item.bytesBuffer)
		item.bytesBuffer=nil
	}
	bulkBufferPool.Put(item)
}

func (receiver *BulkBuffer) Write(data []byte) {
	if data != nil && len(data) > 0 {
		receiver.bytesBuffer.Write(data)
	}
}

func (receiver *BulkBuffer) WriteByteBuffer(data []byte) {
	if data != nil && len(data) > 0 {
		if !util.IsBytesEndingWith(&receiver.bytesBuffer.B,NEWLINEBYTES){
			if !util.BytesHasPrefix(data,NEWLINEBYTES){
				receiver.bytesBuffer.Write(NEWLINEBYTES)
			}
		}
		receiver.bytesBuffer.Write(data)
	}
}

func (receiver *BulkBuffer) WriteNewByteBufferLine(tag string,data []byte) {
	if data != nil && len(data) > 0 {
		if receiver.bytesBuffer.Len()>0{
			if !util.IsBytesEndingWith(&receiver.bytesBuffer.B,NEWLINEBYTES){
				if !util.BytesHasPrefix(data,NEWLINEBYTES) {
					receiver.bytesBuffer.Write(NEWLINEBYTES)
				}
			}
		}
		receiver.bytesBuffer.Write(data)
	}
}

func (receiver *BulkBuffer) WriteStringBuffer(data string) {
	if data != "" && len(data) > 0 {
		if !util.IsBytesEndingWith(&receiver.bytesBuffer.B,NEWLINEBYTES){
			if !util.BytesHasPrefix(util.UnsafeStringToBytes(data),NEWLINEBYTES) {
				receiver.bytesBuffer.Write(NEWLINEBYTES)
			}
		}
		receiver.bytesBuffer.WriteString(data)
	}
}

func (receiver *BulkBuffer) Add(id string, data []byte) {
	if data != nil && len(data) > 0 && len(id) != 0 {
		receiver.MessageIDs = append(receiver.MessageIDs, id)
		if !util.IsBytesEndingWith(&data,NEWLINEBYTES){
			receiver.bytesBuffer.Write(NEWLINEBYTES)
		}
		receiver.bytesBuffer.Write(data)
	}
}

func (receiver *BulkBuffer) GetMessageCount() int {
	return len(receiver.MessageIDs)
}

func (receiver *BulkBuffer) GetMessageSize() int {
	return receiver.bytesBuffer.Len()
}

func (receiver *BulkBuffer) GetMessageBytes() []byte {
	return receiver.bytesBuffer.Bytes()
}

func (receiver *BulkBuffer) WriteMessageID(id string) {
	if len(id) != 0 {
		receiver.MessageIDs = append(receiver.MessageIDs, id)
	}
}

func (receiver *BulkBuffer) Reset() {
	if receiver.bytesBuffer != nil {
		receiver.bytesBuffer.Reset()
	}
	receiver.Queue = ""
	receiver.MessageIDs = receiver.MessageIDs[:0]
}

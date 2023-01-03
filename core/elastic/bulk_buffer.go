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
	buff.bytesBuffer = &bytebufferpool.ByteBuffer{}
	buff.Reset()
	return buff
}

func ReturnBulkBuffer(item *BulkBuffer) {
	item.Reset()
	if item.bytesBuffer!=nil{
		item.bytesBuffer=nil
	}
	bulkBufferPool.Put(item)
}

func (receiver *BulkBuffer) SafetyEndWithNewline() {
	if receiver.bytesBuffer.Len()>0{
		if !util.BytesHasSuffix(receiver.bytesBuffer.B,NEWLINEBYTES){
			receiver.bytesBuffer.Write(NEWLINEBYTES)
		}
	}
}

func (receiver *BulkBuffer) Write(data []byte) {
	if data != nil && len(data) > 0 {
		receiver.bytesBuffer.Write(data)
	}
}

func (receiver *BulkBuffer) WriteByteBuffer(data []byte) {
	if data != nil && len(data) > 0 {
		SafetyAddNewlineBetweenData(receiver.bytesBuffer, data)
	}
}

func (receiver *BulkBuffer) WriteNewByteBufferLine(tag string,data []byte) {
	if data != nil && len(data) > 0 {
		SafetyAddNewlineBetweenData(receiver.bytesBuffer, data)
	}
}

func (receiver *BulkBuffer) WriteStringBuffer(data string) {
	if data != "" && len(data) > 0 {
		SafetyAddNewlineBetweenData(receiver.bytesBuffer,[]byte(data))
	}
}

func SafetyAddNewlineBetweenData(buffer *bytebufferpool.ByteBuffer,data []byte){
	if len(data)<=0{
		return
	}

	if buffer.Len()>0{
		//previous data is not ending with \n
		if !util.BytesHasSuffix(buffer.B,NEWLINEBYTES){
			//new data is not start with \n
			if !util.BytesHasPrefix(data,NEWLINEBYTES){
				buffer.Write(NEWLINEBYTES)
			}
		}
	}
	buffer.Write(data)
}

func (receiver *BulkBuffer) Add(id string, data []byte) {
	if data != nil && len(data) > 0 && len(id) != 0 {
		receiver.MessageIDs = append(receiver.MessageIDs, id)
		SafetyAddNewlineBetweenData(receiver.bytesBuffer, data)
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

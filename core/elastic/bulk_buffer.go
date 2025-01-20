// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
)

type BulkBuffer struct {
	Queue       string
	bytesBuffer *bytebufferpool.ByteBuffer
	MessageIDs  []string
	Reason      []string
}

type BulkBufferPool struct {
	bulkBufferPool  *bytebufferpool.ObjectPool
	bulkBytesBuffer *bytebufferpool.Pool
}

func NewBulkBufferPool(tag string, maxSize, maxItems uint32) *BulkBufferPool {
	pool := BulkBufferPool{}
	pool.bulkBufferPool = bytebufferpool.NewObjectPool("bulk_buffer_objects_"+tag, func() interface{} {
		v := new(BulkBuffer)
		v.Reset()
		return v
	}, func() interface{} {
		return nil
	}, int(maxItems), int(maxSize))

	pool.bulkBytesBuffer = bytebufferpool.NewTaggedPool("bulk_buffer_"+tag, 0, maxSize, maxItems)
	return &pool
}

func (pool *BulkBufferPool) AcquireBulkBuffer() *BulkBuffer {
	buff := pool.bulkBufferPool.Get().(*BulkBuffer)
	if buff.bytesBuffer == nil {
		buff.bytesBuffer = pool.bulkBytesBuffer.Get()
	}
	buff.Reset()
	return buff
}

func (pool *BulkBufferPool) ReturnBulkBuffer(item *BulkBuffer) {
	item.Reset()
	if item.bytesBuffer != nil {
		//item.bytesBuffer.Reset()
		pool.bulkBytesBuffer.Put(item.bytesBuffer)
		item.bytesBuffer = nil
	}
	pool.bulkBufferPool.Put(item)
}

func (receiver *BulkBuffer) SafetyEndWithNewline() {
	if receiver.bytesBuffer.Len() > 0 {
		if !util.BytesHasSuffix(receiver.bytesBuffer.B, NEWLINEBYTES) {
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

func (receiver *BulkBuffer) WriteNewByteBufferLine(tag string, data []byte) {
	if data != nil && len(data) > 0 {
		SafetyAddNewlineBetweenData(receiver.bytesBuffer, data)
	}
}

func (receiver *BulkBuffer) WriteStringBuffer(data string) {
	if data != "" && len(data) > 0 {
		SafetyAddNewlineBetweenData(receiver.bytesBuffer, []byte(data))
	}
}

func SafetyAddNewlineBetweenData(buffer *bytebufferpool.ByteBuffer, data []byte) {
	if len(data) <= 0 {
		return
	}

	// Return early if buffer is empty or already ends with \n
	if buffer.Len() == 0 || util.BytesHasSuffix(buffer.B, NEWLINEBYTES) {
		buffer.Write(data)
		return
	}

	if buffer.Len() > 0 {
		//previous data is not ending with \n
		if !util.BytesHasSuffix(buffer.B, NEWLINEBYTES) {
			//new data is not start with \n
			if !util.BytesHasPrefix(data, NEWLINEBYTES) {
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
	} else {
		log.Error("invalid message id: ", id)
		panic("invalid message id")
	}
}

func (receiver *BulkBuffer) WriteErrorReason(reason string) {
	if len(reason) != 0 {
		receiver.Reason = append(receiver.Reason, reason)
	}
}

func (receiver *BulkBuffer) Reset() {
	receiver.ResetData()
	receiver.Queue = ""
}

func (receiver *BulkBuffer) ResetData() {
	if receiver.bytesBuffer != nil {
		receiver.bytesBuffer.Reset()
	}
	receiver.MessageIDs = receiver.MessageIDs[:0]
	receiver.Reason = receiver.Reason[:0]
}

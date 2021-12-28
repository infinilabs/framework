/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"errors"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	io "io"
	"os"
	log "github.com/cihub/seelog"
	"time"
)

//queue context cache

//每个 consumer 维护自己的元数据，part 自动切换，offset 表示文件级别的读取偏移，messageCount 表示消息的返回条数
func  (d *diskQueue)Consume(consumer string,part,readPos int64,messageCount int,timeout time.Duration) (ctx *queue.Context,messages []util.MapStr, isTimeout bool, err error) {
	messages=[]util.MapStr{}


	fileName:=d.GetFileName(part)

	log.Trace("reading file:",fileName)

	var msgSize int32
	readFile, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
	defer readFile.Close()
	if err != nil {
		return
	}

	var maxBytesPerFileRead int64= d.maxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		return
	}
	maxBytesPerFileRead = stat.Size()


	if readPos > 0 {
		_, err = readFile.Seek(readPos, 0)
		if err != nil {
			return
		}
	}
	var reader= bufio.NewReader(readFile)

	var messageOffset=0

	READ_MSG:

	//read message size
	err = binary.Read(reader, binary.BigEndian, &msgSize)
	if err != nil {
		return
	}

	if int32(msgSize) < d.minMsgSize || int32(msgSize) > d.maxMsgSize {
		err=errors.New("message is too big")
		return
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, readBuf)
	if err != nil {
		return ctx,messages,false,err
	}

	totalBytes := int64(4 + msgSize)
	nextReadPos := readPos + totalBytes
	previousPos:=readPos
	readPos=nextReadPos

	//if messageOffset<from{
	//	messageOffset++
	//	goto READ_MSG
	//}

	message:=util.MapStr{
		//"offset":messageOffset,
		"message":string(readBuf),
		"offset":previousPos,
		"next_offset":nextReadPos,
	}
	messages=append(messages,message)

	if len(messages)>=messageCount{
		return ctx,messages,false,err
	}

	if nextReadPos >= maxBytesPerFileRead{
		return ctx,messages,false,err
	}

	messageOffset++
	goto READ_MSG

}

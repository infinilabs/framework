/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"infini.sh/framework/core/queue"
	io "io"
	"os"
	"time"
)

//queue context cache

//每个 consumer 维护自己的元数据，part 自动切换，offset 表示文件级别的读取偏移，messageCount 表示消息的返回条数
func  (d *diskQueue)Consume(consumer string,part,readPos int64,messageCount int,timeout time.Duration) (ctx *queue.Context,messages []queue.Message, isTimeout bool, err error) {

	messages=[]queue.Message{}
	ctx=&queue.Context{}
	ctx.InitOffset=fmt.Sprintf("%v,%v",part,readPos)

	fileName:=d.GetFileName(part)

	var msgSize int32
	readFile, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
	defer readFile.Close()
	if err != nil {
		//log.Error(err)
		return ctx,messages,false,err
	}

	var maxBytesPerFileRead int64= d.maxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		//log.Error(err)
		return ctx,messages,false,err
	}
	maxBytesPerFileRead = stat.Size()

	if readPos > 0 {
		_, err = readFile.Seek(readPos, 0)
		if err != nil {
			//log.Error(err)
			return ctx,messages,false,err
		}
	}

	var reader= bufio.NewReader(readFile)

	var messageOffset=0

	READ_MSG:

	//read message size
	err = binary.Read(reader, binary.BigEndian, &msgSize)
	if err != nil {
		//log.Errorf("err:%v,%v,%v",err,msgSize,maxBytesPerFileRead)
		return ctx,messages,false,err
	}

	if int32(msgSize) < d.minMsgSize || int32(msgSize) > d.maxMsgSize {
		err=errors.New("message is too big")
		//log.Error(err)
		return ctx,messages,false,err
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, readBuf)
	if err != nil {
		//log.Error(err)
		return ctx,messages,false,err
	}

	totalBytes := int64(4 + msgSize)
	nextReadPos := readPos + totalBytes
	previousPos:=readPos
	readPos=nextReadPos

	message:=queue.Message{
		Data:readBuf,
		Size:totalBytes,
		Offset:fmt.Sprintf("%v,%v",part,previousPos),
	}

	ctx.NextOffset=fmt.Sprintf("%v,%v",part,nextReadPos)

	messages=append(messages,message)

	if len(messages)>=messageCount{
		//log.Error("len(messages)>=messageCount")
		return ctx,messages,false,err
	}

	if nextReadPos >= maxBytesPerFileRead{
		//log.Errorf("nextReadPos >= maxBytesPerFileRead,%v,%v",ctx,len(messages))
		return ctx,messages,false,err
	}

	messageOffset++
	goto READ_MSG

}

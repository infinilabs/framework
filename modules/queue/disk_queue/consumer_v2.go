/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"infini.sh/framework/core/errors"
	"fmt"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	"io"
	"os"
	log "github.com/cihub/seelog"
	"strings"
	"time"
)

type Consumer struct {
	ID string
	diskQueue *diskQueue

	mCfg *DiskQueueConfig
	cCfg *queue.ConsumerConfig

	fileName            string
	maxBytesPerFileRead int64

	reader              *bufio.Reader
	readFile            *os.File

	queue   string
	segment int64
	readPos int64
}

func (c *Consumer) getFileSize()(int64)  {
	var err error
	readFile, err:= os.OpenFile(c.fileName, os.O_RDONLY, 0600)
	if err != nil {
		log.Error(err)
		return -1
	}
	defer readFile.Close()
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		log.Error(err)
		return -1
	}
	return stat.Size()
}

func (d *diskQueue) AcquireConsumer(consumer *queue.ConsumerConfig, segment,readPos int64) (queue.ConsumerAPI,error){
	output:=Consumer{
		ID:util.GetUUID(),
		mCfg: d.cfg,
		diskQueue: d,
		cCfg: consumer,
		queue:d.name,
	}

	if global.Env().IsDebug{
		log.Infof("acquire consumer:%v, %v, %v, %v-%v",output.ID,d.name,consumer.Key(), segment,readPos)
	}
	err:=output.ResetOffset(segment,readPos)
	return &output, err
}

func (d *Consumer) FetchMessages(numOfMessages int) (ctx *queue.Context, messages []queue.Message, isTimeout bool, err error){
	var msgSize int32
	var messageOffset=0
	var totalMessageSize int = 0
	ctx = &queue.Context{}

	initOffset := fmt.Sprintf("%v,%v", d.segment, d.readPos)
	ctx.InitOffset = initOffset
	ctx.NextOffset = initOffset

	messages = []queue.Message{}

READ_MSG:

	//read message size
	err = binary.Read(d.reader, binary.BigEndian, &msgSize)
	if err != nil {
		if err.Error()=="EOF"{

			//current have changes, reload file with new position
			if d.getFileSize()>d.readPos{
				log.Debug("current file have changes, reload:",d.queue,",",d.getFileSize()," > ",d.readPos)
				time.Sleep(1*time.Second)
				ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
				err=d.ResetOffset(d.segment,d.readPos)
				if err!=nil{
					if strings.Contains(err.Error(),"not found"){
						return ctx, messages, false, nil
					}
					panic(err)
				}
				goto READ_MSG
			}


			nextFile,exists:= SmartGetFileName(d.mCfg,d.queue,d.segment+1)
			if exists||util.FileExists(nextFile){
				log.Trace("EOF, continue read:",nextFile)
				Notify(d.queue, ReadComplete,d.segment)
				ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
				err=d.ResetOffset(d.segment+1,0)
				if err!=nil{
					if strings.Contains(err.Error(),"not found"){
						return ctx, messages, false, nil
					}
					panic(err)
				}
				goto READ_MSG
			}else{
				log.Tracef("EOF, but next file [%v] not exists, pause and waiting for new data, messages:%v, newFile:%v", nextFile, len(messages), d.segment < d.diskQueue.writeSegmentNum)

				if d.diskQueue==nil{
					panic("queue can't be nil")
				}

				if d.segment < d.diskQueue.writeSegmentNum {
					oldPart := d.segment
					Notify(d.queue, ReadComplete, d.segment)
					log.Debugf("EOF, but current read segment_id [%v] is less than current write segment_id [%v], increase ++", oldPart, d.segment)
					ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
					err=d.ResetOffset(d.segment + 1,0)
					if err!=nil{
						if strings.Contains(err.Error(),"not found"){
							return ctx, messages, false, nil
						}
						panic(err)
					}

					ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
					return ctx, messages, false, err
				}

				if len(messages) == 0 {
					if global.Env().IsDebug {
						log.Tracef("no message found in queue: %v, sleep 1s", d.queue)
					}
					time.Sleep(1 * time.Second)
				}
			}
			//No error for EOF error
			err=nil
		}else{
			log.Debugf("[%v] err:%v,msgSizeDataRead:%v,maxPerFileRead:%v,msg:%v",d.fileName,err,msgSize,d.maxBytesPerFileRead,len(messages))
		}

		return ctx,messages,false,err
	}

	if int32(msgSize) < d.mCfg.MinMsgSize || int32(msgSize) > d.mCfg.MaxMsgSize {
		err=errors.Errorf("queue:%v,offset:%v,%v, invalid message size: %v, should between: %v TO %v",d.queue,d.segment,d.readPos,msgSize,d.mCfg.MinMsgSize,d.mCfg.MaxMsgSize)
		return ctx,messages,false,err
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(d.reader, readBuf)
	if err != nil {
		log.Debug(err)
		if err.Error()=="EOF" {
			err=nil
		}
		return ctx,messages,false,err
	}

	totalBytes := int(4 + msgSize)
	nextReadPos := d.readPos + int64(totalBytes)
	previousPos:=d.readPos
	d.readPos=nextReadPos

	if d.mCfg.Compress.Message.Enabled{
		newData,err:= zstd.ZSTDDecompress(nil,readBuf)
		if err!=nil{
			log.Debug(err)
			ctx.NextOffset=fmt.Sprintf("%v,%v",d.segment,nextReadPos)
			return ctx,messages,false,err
		}
		readBuf=newData
	}

	message := queue.Message{
		Data:       readBuf,
		Size:       totalBytes,
		Offset:     fmt.Sprintf("%v,%v", d.segment, previousPos),
		NextOffset: fmt.Sprintf("%v,%v", d.segment, nextReadPos),
	}

	ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, nextReadPos)

	messages = append(messages, message)
	totalMessageSize += message.Size

	if len(messages) >= d.cCfg.FetchMaxMessages {
		log.Tracef("queue:%v, consumer:%v, total messages count(%v)>=max message count(%v)", d.queue, d.cCfg.Name, len(messages), d.cCfg.FetchMaxMessages)
		return ctx, messages, false, err
	}

	if totalMessageSize > d.cCfg.FetchMaxBytes && d.cCfg.FetchMaxBytes > 0 {
		log.Tracef("queue:%v, consumer:%v, total messages size(%v)>=max message size(%v)", d.queue, d.cCfg.Name, util.ByteSize(uint64(totalMessageSize)), util.ByteSize(uint64(d.cCfg.FetchMaxBytes)))
		return ctx, messages, false, err
	}

	if nextReadPos >= d.maxBytesPerFileRead {
		nextFile,exists := SmartGetFileName(d.mCfg,d.queue, d.segment+1)
		if exists||util.FileExists(nextFile) {
			//current have changes, reload file with new position
			if d.getFileSize()>d.readPos{
				if global.Env().IsDebug{
					log.Debug("current file have changes, reload:",d.queue,",",d.getFileSize()," > ",d.readPos)
				}
				time.Sleep(1*time.Second)
				ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
				err=d.ResetOffset(d.segment,d.readPos)
				if err!=nil{
					if strings.Contains(err.Error(),"not found"){
						return ctx, messages, false, nil
					}
					panic(err)
				}
				goto READ_MSG
			}

			log.Trace("EOF, continue read:", nextFile)

			Notify(d.queue, ReadComplete, d.segment)
			ctx.NextOffset = fmt.Sprintf("%v,%v", d.segment, d.readPos)
			err=d.ResetOffset(d.segment+1,0)
			if err!=nil{
				if strings.Contains(err.Error(),"not found"){
					return ctx, messages, false, nil
				}
				panic(err)
			}
			goto READ_MSG
		}
		return ctx, messages, false, err
	}

	messageOffset++
	goto READ_MSG
}

func (d *Consumer) Close() error {
	if d.readFile!=nil{
		return d.readFile.Close()
	}
	return nil
}

func (d *Consumer) ResetOffset(segment,readPos int64)error {
	if d.segment!=segment{
		if global.Env().IsDebug{
			log.Debugf("start to switch segment, previous:%v,%v, now: %v,%v",d.segment,d.readPos,segment,readPos)
		}
		if d.readFile!=nil{
			d.readFile.Close()
		}
	}

	d.segment= segment
	d.readPos= readPos
	d.maxBytesPerFileRead=0

	fileName,exists := SmartGetFileName(d.mCfg,d.queue, segment)
	if !exists{
		if !util.FileExists(fileName) {
			return errors.New(fileName+" not found")
		}
	}

	var err error
	readFile, err:= os.OpenFile(fileName, os.O_RDONLY, 0600)
	if err != nil {
		log.Error(err)
		return err
	}
	d.readFile=readFile
	var maxBytesPerFileRead int64= d.mCfg.MaxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		log.Error(err)
		return err
	}
	maxBytesPerFileRead = stat.Size()

	if d.readPos > 0 {
		_, err = readFile.Seek(d.readPos, 0)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	d.maxBytesPerFileRead=maxBytesPerFileRead
	if d.reader!=nil{
		d.reader.Reset(d.readFile)
	}else{
		d.reader= bufio.NewReader(d.readFile)
	}
	d.fileName=fileName

	return nil
}
/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	io "io"
	"os"
	"time"
)

//if local file not found, try to download from s3
func (d *diskQueue)SmartGetFileName(queueID string,segmentID int64) string {
	filePath:=GetFileName(queueID,segmentID)
	if !util.FileExists(filePath){
		//download from s3 if that is possible
		lastFileNum:=GetLastS3UploadFileNum(queueID)
		if lastFileNum>=segmentID{
			s3Object:=getS3FileLocation(filePath)
			s3.SyncDownload(filePath,d.cfg.S3.Server,d.cfg.S3.Location,d.cfg.S3.Bucket,s3Object)
		}
	}
	return filePath
}

//每个 consumer 维护自己的元数据，part 自动切换，offset 表示文件级别的读取偏移，messageCount 表示消息的返回条数
func  (d *diskQueue)Consume(consumer string,part,readPos int64,messageCount int,timeout time.Duration) (ctx *queue.Context,messages []queue.Message, isTimeout bool, err error) {

	messages=[]queue.Message{}
	ctx=&queue.Context{}
	initOffset:=fmt.Sprintf("%v,%v",part,readPos)
	defer func() {
		ctx.InitOffset=initOffset
	}()

	RELOCATE_FILE:

	log.Tracef("[%v] consumer[%v] %v,%v, fetch count:%v",d.dataPath,consumer,part,readPos,messageCount)
	ctx.InitOffset=fmt.Sprintf("%v,%v",part,readPos)
	ctx.NextOffset=""

	fileName:= d.SmartGetFileName(d.name,part)

	if !util.FileExists(fileName){
		time.Sleep(10*time.Second)
		return ctx,messages,true,err
	}

	var msgSize int32
	readFile, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
	//defer readFile.Close()
	if err != nil {
		log.Debug(err)
		return ctx,messages,false,err
	}

	var maxBytesPerFileRead int64= d.cfg.MaxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		log.Debug(err)
		return ctx,messages,false,err
	}
	maxBytesPerFileRead = stat.Size()

	if readPos > 0 {
		_, err = readFile.Seek(readPos, 0)
		if err != nil {
			log.Debug(err)
			return ctx,messages,false,err
		}
	}

	var reader= bufio.NewReader(readFile)

	var messageOffset=0

	READ_MSG:

	//read message size
	err = binary.Read(reader, binary.BigEndian, &msgSize)
	if err != nil {
		if err.Error()=="EOF"{
			log.Tracef("[%v] EOF err:%v, move to next file,msgSizeDataRead:%v,maxPerFileRead:%v,msg:%v",fileName,err,msgSize,maxBytesPerFileRead,len(messages))
			nextFile:= d.SmartGetFileName(d.name,part+1)
			if util.FileExists(nextFile){
				log.Debug("EOF, continue read:",nextFile)

				Notify(d.name, ReadComplete,part)

				part=part+1
				readPos=0
				if readFile!=nil{
					readFile.Close()
				}
				goto RELOCATE_FILE
			}else{
				log.Debugf("EOF, next file [%v] not exists, pause and waiting for new data",nextFile)
				if len(messages)==0{
					time.Sleep(10*time.Second)
				}
			}
		}else{
			log.Debugf("[%v] err:%v,msgSizeDataRead:%v,maxPerFileRead:%v,msg:%v",fileName,err,msgSize,maxBytesPerFileRead,len(messages))
		}

		return ctx,messages,false,err
	}

	if int32(msgSize) < d.cfg.MinMsgSize || int32(msgSize) > d.cfg.MaxMsgSize {
		err=errors.Errorf("message is too big, %v/%v-%v",msgSize,d.cfg.MinMsgSize,d.cfg.MaxMsgSize)
		return ctx,messages,false,err
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, readBuf)
	if err != nil {
		log.Debug(err)
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
		NextOffset:fmt.Sprintf("%v,%v",part,nextReadPos),
	}

	ctx.NextOffset=fmt.Sprintf("%v,%v",part,nextReadPos)

	messages=append(messages,message)

	if len(messages)>=messageCount{
		log.Trace("len(messages)>=messageCount")
		return ctx,messages,false,err
	}

	if nextReadPos >= maxBytesPerFileRead{
		log.Tracef("nextReadPos >= maxBytesPerFileRead,%v,%v,%v",ctx,len(messages),err)
		nextFile:= d.SmartGetFileName(d.name,part+1)
		if util.FileExists(nextFile){
			log.Debug("EOF, continue read:",nextFile)
			part=part+1
			readPos=0
			if readFile!=nil{
				readFile.Close()
			}
			goto RELOCATE_FILE
		}
		return ctx,messages,false,err
	}

	messageOffset++
	goto READ_MSG

}


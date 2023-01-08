/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	io "io"
	"os"
)

//if local file not found, try to download from s3
func SmartGetFileName(cfg *DiskQueueConfig,queueID string,segmentID int64) (string,bool) {
	filePath:= GetFileName(queueID,segmentID)
	exists:=util.FileExists(filePath)
	if !exists{
		if cfg.Compress.Segment.Enabled{
			//check local compressed file
			compressedFile:=filePath+compressFileSuffix
			if util.FileExists(compressedFile){
				err:=zstd.DecompressFile(&compressLocker,compressedFile,filePath)
				if err!=nil&&err.Error()!="unexpected EOF"&&util.ContainStr(err.Error(),"exists"){
					panic(err)
				}
			}
		}

		if cfg.UploadToS3||cfg.AlwaysDownload{

			//download from s3 if that is possible
			lastFileNum:= GetLastS3UploadFileNum(queueID)
			if cfg.AlwaysDownload || lastFileNum>=segmentID{
				var fileToDownload=filePath
				//download compressed segments, check config, un-compress and rename
				if cfg.Compress.Segment.Enabled{
					fileToDownload=filePath+compressFileSuffix
				}
				s3Object:= getS3FileLocation(fileToDownload)

				// download remote file
				_,err:=s3.SyncDownload(fileToDownload,cfg.S3.Server,cfg.S3.Location,cfg.S3.Bucket,s3Object)
				if err!=nil{
					if util.ContainStr(err.Error(),"exist") && cfg.AlwaysDownload{
						return filePath,false
					}
					panic(err)
				}

				//uncompress after download
				if cfg.Compress.Segment.Enabled&&fileToDownload!=filePath{
					err:=zstd.DecompressFile(&compressLocker,fileToDownload,filePath)
					if err!=nil&&err.Error()!="unexpected EOF"&&util.ContainStr(err.Error(),"exists"){
						panic(err)
					}
				}
			}
		}

	}
	return filePath,exists
}

//每个 consumer 维护自己的元数据，part 自动切换，offset 表示文件级别的读取偏移，messageCount 表示消息的返回条数
func (d *DiskBasedQueue) Consume(consumer *queue.ConsumerConfig, part, readPos int64) (ctx *queue.Context, messages []queue.Message, isTimeout bool, err error) {
	messages = []queue.Message{}
	var totalMessageSize int = 0
	ctx = &queue.Context{}
	initOffset := fmt.Sprintf("%v,%v", part, readPos)
	defer func() {
		ctx.InitOffset = initOffset
	}()

RELOCATE_FILE:

	log.Tracef("[%v] consumer[%v] %v,%v, max fetch count:%v", d.dataPath, consumer.Name, part, readPos, consumer.FetchMaxMessages)
	ctx.InitOffset = fmt.Sprintf("%v,%v", part, readPos)
	ctx.NextOffset = ctx.InitOffset
	fileName,exists := SmartGetFileName(d.cfg,d.name, part)
	if !exists{
		if !util.FileExists(fileName) {
			return ctx, messages, false, err
		}
	}

	var msgSize int32
	readFile, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
	if readFile != nil {
		defer readFile.Close()
	}
	if err != nil {
		log.Debug(err)
		return ctx, messages, false, err
	}

	var maxBytesPerFileRead int64= d.cfg.MaxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err!=nil{
		log.Debug(err)
		if err.Error()=="EOF" {
			err=nil
		}
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
			nextFile,exists:= SmartGetFileName(d.cfg,d.name,part+1)
			if exists||util.FileExists(nextFile){
				log.Trace("EOF, continue read:",nextFile)
				Notify(d.name, ReadComplete,part)
				part=part+1
				readPos=0
				if readFile!=nil{
					readFile.Close()
				}
				goto RELOCATE_FILE
			}else{
				log.Tracef("EOF, but next file [%v] not exists, pause and waiting for new data, messages:%v, newFile:%v", nextFile, len(messages), part < d.writeSegmentNum)

				if part < d.writeSegmentNum {
					oldPart := part
					Notify(d.name, ReadComplete, part)
					part = part + 1
					readPos = 0
					log.Debugf("EOF, but current read segment_id [%v] is less than current write segment_id [%v], increase ++", oldPart, part)
					if readFile != nil {
						readFile.Close()
					}
					ctx.NextOffset = fmt.Sprintf("%v,%v", part, readPos)
					return ctx, messages, false, err
				}

				if len(messages) == 0 {
					if global.Env().IsDebug {
						log.Tracef("no message found in queue: %v, sleep 1s", d.name)
					}
				}
			}
			//No error for EOF error
			err=nil
		}else{
			log.Debugf("[%v] err:%v,msgSizeDataRead:%v,maxPerFileRead:%v,msg:%v",fileName,err,msgSize,maxBytesPerFileRead,len(messages))
		}

		return ctx,messages,false,err
	}

	if int32(msgSize) < d.cfg.MinMsgSize || int32(msgSize) > d.cfg.MaxMsgSize {
		err=errors.Errorf("queue:%v,offset:%v,%v, invalid message, size: %v, should between: %v TO %v",d.name,part,readPos,msgSize,d.cfg.MinMsgSize,d.cfg.MaxMsgSize)
		return ctx,messages,false,err
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(reader, readBuf)
	if err != nil {
		log.Debug(err)
		if err.Error()=="EOF" {
			err=nil
		}
		return ctx,messages,false,err
	}

	totalBytes := int(4 + msgSize)
	nextReadPos := readPos + int64(totalBytes)
	previousPos:=readPos
	readPos=nextReadPos

	if d.cfg.Compress.Message.Enabled{
		//option 1
		newData,err:= zstd.ZSTDDecompress(nil,readBuf)
		if err!=nil{
			log.Debug(err)
			ctx.NextOffset=fmt.Sprintf("%v,%v",part,nextReadPos)
			return ctx,messages,false,err
		}
		readBuf=newData

		//option 2
		////TODO release buffer in after context released
		//dataReader:=bytes.Reader{}
		//dataReader.Reset(readBuf)
		//dataWriter:=bytebufferpool.Get()
		//
		//err:= zstd.ZSTDReusedDecompress(dataWriter,&dataReader)
		//if err!=nil{
		//	log.Debug(err)
		//	return ctx,messages,false,err
		//}
		//readBuf=dataWriter.Bytes()

	}

	message := queue.Message{
		Data:       readBuf,
		Size:       totalBytes,
		Offset:     fmt.Sprintf("%v,%v", part, previousPos),
		NextOffset: fmt.Sprintf("%v,%v", part, nextReadPos),
	}

	ctx.NextOffset = fmt.Sprintf("%v,%v", part, nextReadPos)

	messages = append(messages, message)
	totalMessageSize += message.Size

	if len(messages) >= consumer.FetchMaxMessages {
		log.Tracef("queue:%v, consumer:%v, total messages count(%v)>=max message count(%v)", d.name, consumer.Name, len(messages), consumer.FetchMaxMessages)
		return ctx, messages, false, err
	}

	if totalMessageSize > consumer.FetchMaxBytes && consumer.FetchMaxBytes > 0 {
		log.Tracef("queue:%v, consumer:%v, total messages size(%v)>=max message size(%v)", d.name, consumer.Name, util.ByteSize(uint64(totalMessageSize)), util.ByteSize(uint64(consumer.FetchMaxBytes)))
		return ctx, messages, false, err
	}

	if nextReadPos >= maxBytesPerFileRead {
		//TODO checking the current file whether to have new changes or not during read
		nextFile,exists := SmartGetFileName(d.cfg,d.name, part+1)
		if exists||util.FileExists(nextFile) {
			log.Trace("EOF, continue read:", nextFile)
			Notify(d.name, ReadComplete, part)
			part = part + 1
			readPos = 0
			if readFile != nil {
				readFile.Close()
			}
			goto RELOCATE_FILE
		}
		return ctx, messages, false, err
	}

	messageOffset++
	goto READ_MSG

}


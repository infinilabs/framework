/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"infini.sh/framework/core/kv"
	"io"
	"os"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
)

// NOTE: Consumer is not thread-safe
type Consumer struct {
	ID        string
	diskQueue *DiskBasedQueue

	mCfg *DiskQueueConfig
	qCfg *queue.QueueConfig
	cCfg *queue.ConsumerConfig

	fileName            string
	maxBytesPerFileRead int64

	reader   *bufio.Reader
	readFile *os.File

	queue   string
	segment int64
	readPos int64

	lastFileSize int64
}

func (c *Consumer) getFileSize() int64 {
	var err error
	readFile, err := os.OpenFile(c.fileName, os.O_RDONLY, 0600)
	if err != nil {
		log.Error(c.diskQueue.writeSegmentNum, ",", err)
		return -1
	}
	defer readFile.Close()
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err != nil {
		log.Error(err)
		return -1
	}
	return stat.Size()
}

func (d *DiskBasedQueue) AcquireConsumer(qconfig *queue.QueueConfig,consumer *queue.ConsumerConfig, segment, readPos int64) (queue.ConsumerAPI, error) {
	output := Consumer{
		ID:        util.ToString(util.GetIncrementID("consumer")),
		mCfg:      d.cfg,
		diskQueue: d,
		qCfg:      qconfig,
		cCfg:      consumer,
		queue:     d.name,
	}

	if global.Env().IsDebug {
		log.Debugf("acquire consumer:%v, %v, %v, %v-%v", output.ID, d.name, consumer.Key(), segment, readPos)
	}
	err := output.ResetOffset(segment, readPos)
	return &output, err
}

func (this *Consumer) CommitOffset(offset queue.Offset) error{

	if global.Env().IsDebug{
		log.Debug("queue:",this.qCfg.ID,"(",this.qCfg.Name,"), commit offset:",offset.String())
	}
	return kv.AddValue(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(this.qCfg, this.cCfg)), []byte(offset.String()))
}

func (d *Consumer) FetchMessages(ctx *queue.Context, numOfMessages int) (messages []queue.Message, isTimeout bool, err error) {

	//log.Error("start fetch messages:", d.queue, ",", d.segment, ",", d.readPos, ",", numOfMessages)
	//defer log.Error("end fetch messages:", d.queue, ",", d.segment, ",", d.readPos, ",", numOfMessages)

	var msgSize int32
	var totalMessageSize int = 0
	ctx.MessageCount=0

	ctx.UpdateInitOffset(d.segment, d.readPos)
	ctx.NextOffset = ctx.InitOffset

	messages = []queue.Message{}

	var retryTimes int=0

READ_MSG:

	//log.Error("start read message:",d.segment,",", d.readPos)

	if global.ShuttingDown(){
		return messages, false, errors.New("shutting down")
	}

	if retryTimes>0{
		if d.cCfg.EOFMaxRetryTimes>0&& retryTimes >= d.cCfg.EOFMaxRetryTimes{
			return messages, false, errors.New("too many retry times")
		}

		if retryTimes>10{
			log.Warn("still retry:", d.queue, ",", d.lastFileSize, " > ", d.readPos,", retry times:",retryTimes)
		}
	}

	//read message size
	err = binary.Read(d.reader, binary.BigEndian, &msgSize)
	if err != nil {
		if global.Env().IsDebug{
			log.Trace(err)
		}
		errMsg := err.Error()
		if util.ContainStr(errMsg, "EOF") || util.ContainStr(errMsg, "file already closed") {
			//current have changes, reload file with new position
			newFileSize:=d.getFileSize()
			if d.lastFileSize!=newFileSize&&newFileSize > d.readPos {
				d.lastFileSize=newFileSize
				log.Debug("current file have changes, reload:", d.queue, ",", newFileSize, " > ", d.readPos)
				if d.cCfg.EOFRetryDelayInMs > 0 {
					time.Sleep(time.Duration(d.cCfg.EOFRetryDelayInMs) * time.Millisecond)
				}
				ctx.UpdateNextOffset(d.segment, d.readPos)
				err = d.ResetOffset(d.segment, d.readPos)
				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						return messages, false, nil
					}
					panic(err)
				}
				retryTimes++
				goto READ_MSG
			}

			nextFile, exists := SmartGetFileName(d.mCfg, d.queue, d.segment+1)
			if exists || util.FileExists(nextFile) {
				log.Trace("EOF, continue read:", nextFile)
				Notify(d.queue, ReadComplete, d.segment)
				ctx.UpdateNextOffset(d.segment, d.readPos)
				err = d.ResetOffset(d.segment+1, 0)
				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						return messages, false, nil
					}
					panic(err)
				}
				retryTimes=0
				goto READ_MSG
			} else {
				log.Tracef("EOF, but next file [%v] not exists, pause and waiting for new data, messages count: %v, readPos: %d, newFile:%v", nextFile, len(messages), d.readPos, d.segment < d.diskQueue.writeSegmentNum)

				if d.diskQueue == nil {
					panic("queue can't be nil")
				}

				if d.segment < d.diskQueue.writeSegmentNum {
					oldPart := d.segment
					Notify(d.queue, ReadComplete, d.segment)
					log.Debugf("EOF, but current read segment_id [%v] is less than current write segment_id [%v], increase ++", oldPart, d.segment)
					ctx.UpdateNextOffset(d.segment, d.readPos)
					err = d.ResetOffset(d.segment+1, 0)
					if err != nil {
						if strings.Contains(err.Error(), "not found") {
							return messages, false, nil
						}
						panic(err)
					}

					ctx.UpdateNextOffset(d.segment, d.readPos)
					return messages, false, err
				}

				if len(messages) == 0 {
					if global.Env().IsDebug {
						log.Tracef("no message found in queue: %v, sleep 1s", d.queue)
					}
					if d.cCfg.EOFRetryDelayInMs > 0 {
						time.Sleep(time.Duration(d.cCfg.EOFRetryDelayInMs) * time.Millisecond)
					}
				}
			}
			//No error for EOF error
			err = nil
		} else {
			log.Error("[%v] err:%v,msgSizeDataRead:%v,maxPerFileRead:%v,msg:%v", d.fileName, err, msgSize, d.maxBytesPerFileRead, len(messages))
		}
		return messages, false, err
	}

	if int32(msgSize) < d.mCfg.MinMsgSize || int32(msgSize) > d.mCfg.MaxMsgSize {

		//current have changes, reload file with new position
		newFileSize:=d.getFileSize()
		if d.lastFileSize!=newFileSize&&newFileSize > d.maxBytesPerFileRead {
			d.lastFileSize=newFileSize
			d.ResetOffset(d.segment, d.readPos)
			return messages, false, err
		}else{
			if d.diskQueue.cfg.AutoSkipCorruptFile{
				//invalid message size, assume current file is corrupted, try to read next file
				nextSegment:=d.segment+1
				nextFile, exists := SmartGetFileName(d.mCfg, d.queue, nextSegment)

				log.Warnf("queue:%v, offset:%v,%v, invalid message size: %v, should between: %v TO %v, skip to next file: %v, exists: %v", d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize,nextFile,exists)

				if exists || util.FileExists(nextFile) {
					//update offset
					ctx.UpdateNextOffset(nextSegment, 0)
					err = d.ResetOffset(nextSegment, 0)

					Notify(d.queue, ReadComplete, d.segment)
					if err != nil {
						if strings.Contains(err.Error(), "not found") {
							return messages, false, nil
						}
						panic(err)
					}
					retryTimes=0
					goto READ_MSG //reset since we moved to next file
				}else{
					if d.diskQueue.writeSegmentNum==d.segment{
						log.Errorf("need to skip to next file, but next file not exists, current write segment:%v, current read segment:%v",d.diskQueue.writeSegmentNum,d.segment)
						d.diskQueue.skipToNextRWFile(false)
						d.diskQueue.needSync = true
					}
					return messages, false, nil
				}
			}
		}

		err = errors.Errorf("queue:%v,offset:%v,%v, invalid message size: %v, should between: %v TO %v", d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize)
		return messages, false, err
	}

	//read message
	readBuf := make([]byte, msgSize)
	_, err = io.ReadFull(d.reader, readBuf)

	totalBytes := int(4 + msgSize)
	nextReadPos := d.readPos + int64(totalBytes)
	previousPos := d.readPos
	d.readPos = nextReadPos

	//check read error
	if err != nil {
		if util.ContainStr(err.Error(), "EOF") {
			if d.readPos >= d.maxBytesPerFileRead || d.maxBytesPerFileRead==d.getFileSize() {
				//next file exists, and current file is EOF
				log.Debug("EOF, relocate to next file: ",nextReadPos >= d.maxBytesPerFileRead,",",d.readPos,",",d.maxBytesPerFileRead,",",d.getFileSize())
				goto RELOAD_FILE
			}
			err = nil
		} else {
			log.Error(err)
		}
		return messages, false, err
	}else{
		if d.mCfg.Compress.Message.Enabled {
			newData, err := zstd.ZSTDDecompress(nil, readBuf)
			if err != nil {
				log.Error(err)
				ctx.UpdateNextOffset(d.segment, nextReadPos)
				return messages, false, err
			}
			readBuf = newData
		}

		message := queue.Message{
			Data:       readBuf,
			Size:       totalBytes,
			Offset:     queue.NewOffset(d.segment,previousPos),
			NextOffset: queue.NewOffset(d.segment,nextReadPos),
		}

		ctx.UpdateNextOffset(d.segment, nextReadPos)

		messages = append(messages, message)
		ctx.MessageCount++
		totalMessageSize += message.Size

		if len(messages) >= d.cCfg.FetchMaxMessages || (len(messages) >= numOfMessages&&numOfMessages>0){
			log.Tracef("queue:%v, consumer:%v, total messages count(%v)>=max message count(%v)", d.queue, d.cCfg.Name, len(messages), d.cCfg.FetchMaxMessages)
			return messages, false, err
		}

		if totalMessageSize > d.cCfg.FetchMaxBytes && d.cCfg.FetchMaxBytes > 0 {
			log.Tracef("queue:%v, consumer:%v, total messages size(%v)>=max message size(%v)", d.queue, d.cCfg.Name, util.ByteSize(uint64(totalMessageSize)), util.ByteSize(uint64(d.cCfg.FetchMaxBytes)))
			return messages, false, err
		}
	}

	RELOAD_FILE:
	if nextReadPos >= d.maxBytesPerFileRead {

		log.Trace("try to relocate to next file: ",nextReadPos >= d.maxBytesPerFileRead,",",d.readPos,",",d.maxBytesPerFileRead,",",d.getFileSize())

		nextFile, exists := SmartGetFileName(d.mCfg, d.queue, d.segment+1)
		if exists || util.FileExists(nextFile) {
			//current have changes, reload file with new position
			newFileSize:=d.getFileSize()
			if d.lastFileSize!=newFileSize&&newFileSize > d.readPos {

				//update last file size
				d.lastFileSize=newFileSize

				if global.Env().IsDebug {
					log.Debug("current file have changes, reload:", d.queue, ",", newFileSize, " > ", d.readPos)
				}
				ctx.UpdateNextOffset(d.segment, d.readPos)
				err = d.ResetOffset(d.segment, d.readPos)
				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						return messages, false, nil
					}
					panic(err)
				}

				if d.cCfg.EOFRetryDelayInMs > 0 {
					time.Sleep(time.Duration(d.cCfg.EOFRetryDelayInMs) * time.Millisecond)
				}

				retryTimes++
				goto READ_MSG
			}

			log.Trace("EOF, continue read:", nextFile)

			Notify(d.queue, ReadComplete, d.segment)
			ctx.UpdateNextOffset(d.segment, d.readPos)
			err = d.ResetOffset(d.segment+1, 0)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return messages, false, nil
				}
				panic(err)
			}
			retryTimes=0 //reset since we moved to next file
			goto READ_MSG
		}
		return messages, false, err
	}

	goto READ_MSG
}

func (d *Consumer) Close() error {
	d.diskQueue.DeleteSegmentConsumerInReading(d.ID)
	if d.readFile != nil {
		err := d.readFile.Close()
		if err != nil && !util.ContainStr(err.Error(), "already") {
			log.Error(err)
			panic(err)
		}
		d.readFile = nil
		return err
	}
	return nil
}

func (d *Consumer) ResetOffset(segment, readPos int64) error {

	if global.Env().IsDebug {
		log.Debugf("reset offset: %v,%v, file: %v, queue:%v", segment, readPos, d.fileName,d.queue)
	}

	if segment > d.diskQueue.writeSegmentNum {
		log.Errorf("reading segment [%v] is greater than writing segment [%v]", segment, d.diskQueue.writeSegmentNum)
		return io.EOF
	}

	if d.segment != segment {
		if global.Env().IsDebug {
			log.Debugf("start to switch segment, previous:%v,%v, now: %v,%v", d.segment, d.readPos, segment, readPos)
		}
		//potential file handler leak
		if d.readFile != nil {
			d.readFile.Close()
		}
	}

	d.segment = segment
	d.readPos = readPos
	d.maxBytesPerFileRead = 0

	d.diskQueue.UpdateSegmentConsumerInReading(d.ID, d.segment)

	fileName, exists := SmartGetFileName(d.mCfg, d.queue, segment)
	if !exists {
		if !util.FileExists(fileName) {
			if d.mCfg.AutoSkipCorruptFile{
				log.Warnf("queue:%v, offset:%v,%v, file missing: %v, auto skip to next file",
					d.queue, d.segment, d.readPos, fileName)
				nextSegment := d.segment + 1
			RETRY_NEXT_FILE:
				// there are segments in the middle
				if segment<d.diskQueue.writeSegmentNum{
					fileName, exists = SmartGetFileName(d.mCfg, d.queue, nextSegment)
					log.Debugf("try skip to next file: %v, exists: %v", fileName, exists)
					if exists || util.FileExists(fileName) {
						d.segment = nextSegment
						d.readPos=0
					}else{
						nextSegment++
						log.Debugf("move to next file: %v",nextSegment)
						goto RETRY_NEXT_FILE
					}
				}else{
					return errors.New(fileName + " not found")
				}
			}else{
				return errors.New(fileName + " not found")
			}
		}
	}

	var err error
	readFile, err := os.OpenFile(fileName, os.O_RDONLY, 0600)
	if err != nil {
		log.Error(err)
		return err
	}
	d.readFile = readFile
	var maxBytesPerFileRead int64 = d.mCfg.MaxBytesPerFile
	var stat os.FileInfo
	stat, err = readFile.Stat()
	if err != nil {
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

	d.maxBytesPerFileRead = maxBytesPerFileRead
	if d.reader != nil {
		d.reader.Reset(d.readFile)
	} else {
		d.reader = bufio.NewReader(d.readFile)
	}
	d.fileName = fileName
	return nil
}

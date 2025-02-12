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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"bufio"
	"encoding/binary"
	"infini.sh/framework/core/stats"
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
	version int64 //offset version

	lastFileSize      int64
	fileLoadCompleted bool
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

func (d *DiskBasedQueue) AcquireConsumer(qconfig *queue.QueueConfig, consumer *queue.ConsumerConfig, offset queue.Offset) (queue.ConsumerAPI, error) {
	output := Consumer{
		ID:        util.ToString(util.GetIncrementID("consumer")),
		mCfg:      d.cfg,
		diskQueue: d,
		qCfg:      qconfig,
		cCfg:      consumer,
		queue:     d.name,
		version:   offset.Version,
	}

	if global.Env().IsDebug {
		log.Debugf("acquire consumer:%v, %v, %v, %v", output.ID, d.name, consumer.Key(), offset.EncodeToString())
	}
	err := output.ResetOffset(offset.Segment, offset.Position)
	return &output, err
}

func (this *Consumer) CommitOffset(offset queue.Offset) error {
	_, err := saveOffset(this.qCfg, this.cCfg, offset)
	return err
}

func (d *Consumer) FetchMessages(ctx *queue.Context, numOfMessages int) (messages []queue.Message, isTimeout bool, err error) {

	var msgSize int32
	var totalMessageSize int = 0
	ctx.MessageCount = 0

	ctx.UpdateInitOffset(d.segment, d.readPos, d.version)
	ctx.NextOffset = ctx.InitOffset

	messages = []queue.Message{}

	//skip future segment
	if d.diskQueue.writeSegmentNum < d.segment {
		return messages, false, errors.New("segment not found")
	}

	var retryTimes int = 0

READ_MSG:

	if global.ShuttingDown() {
		return messages, false, errors.New("shutting down")
	}

	if retryTimes > 0 {
		if d.cCfg.EOFMaxRetryTimes > 0 && retryTimes >= d.cCfg.EOFMaxRetryTimes {
			return messages, false, errors.New("too many retry times")
		}

		if retryTimes > 10 {
			log.Warn("still retry:", d.queue, ",", d.lastFileSize, " > ", d.readPos, ", retry times:", retryTimes)
		}
	}

	//read message size
	err = binary.Read(d.reader, binary.BigEndian, &msgSize)
	if err != nil {
		if global.Env().IsDebug {
			log.Trace(err)
		}
		errMsg := err.Error()
		if util.ContainStr(errMsg, "EOF") || util.ContainStr(errMsg, "file already closed") {
			//current have changes, reload file with new position
			newFileSize := d.getFileSize()
			if d.lastFileSize != newFileSize && newFileSize > d.readPos {
				d.lastFileSize = newFileSize
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

			//check next file, if exists, read next file
			nextFile, exists, _ := SmartGetFileName(d.mCfg, d.queue, d.segment+1)
			if d.fileLoadCompleted && exists {
				if global.Env().IsDebug {
					log.Trace("EOF, continue read:", nextFile)
				}
				Notify(d.queue, ReadComplete, d.segment)
				ctx.UpdateNextOffset(d.segment, d.readPos) //update next offset
				err = d.ResetOffset(d.segment+1, 0)        //locate next file
				if err != nil {
					if strings.Contains(err.Error(), "not found") {
						return messages, false, nil
					}
					panic(err)
				}
				//try another segment, update next offset
				ctx.UpdateNextOffset(d.segment, d.readPos)
				retryTimes = 0
				goto READ_MSG
			} else {
				if global.Env().IsDebug {
					log.Tracef("EOF, but next file [%v] not exists, pause and waiting for new data, messages count: %v, readPos: %d, newFile:%v", nextFile, len(messages), d.readPos, d.segment < d.diskQueue.writeSegmentNum)
				}
				if d.diskQueue == nil {
					panic("queue can't be nil")
				}

				//if current segment is less than write segment, increase segment
				if d.fileLoadCompleted && d.segment < d.diskQueue.writeSegmentNum {
					oldPart := d.segment
					Notify(d.queue, ReadComplete, d.segment)
					ctx.UpdateNextOffset(d.segment, d.readPos) //update next offset
					log.Debugf("EOF, but current read segment_id [%v] is less than current write segment_id [%v], increase ++", oldPart, d.diskQueue.writeSegmentNum)
					err = d.ResetOffset(d.segment+1, 0) //locate next segment
					if err != nil {
						if strings.Contains(err.Error(), "not found") {
							return messages, false, nil
						}
						log.Errorf("queue:%v,offset:%v,%v, invalid message size: %v, should between: %v TO %v, error: %v", d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize, err)
						panic(err)
					}
					ctx.UpdateNextOffset(d.segment, d.readPos)

					return messages, false, err
				}

				if len(messages) == 0 {
					if global.Env().IsDebug {
						log.Tracef("no message found in queue: %v, at offset: %v,%v, sleep %v ms", d.queue, d.segment, d.readPos, d.cCfg.EOFRetryDelayInMs)
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
	log.Debugf("queue:%v, offset:%v,%v, msgSize:%v", d.queue, d.segment, d.readPos, msgSize)
	if int32(msgSize) < d.mCfg.MinMsgSize || int32(msgSize) > d.mCfg.MaxMsgSize {
		//current have changes, reload file with new position
		newFileSize := d.getFileSize()
		if d.lastFileSize != newFileSize && newFileSize > d.maxBytesPerFileRead {
			d.lastFileSize = newFileSize
			err = d.ResetOffset(d.segment, d.readPos)
			if err != nil {
				log.Errorf("queue:%v,offset:%v,%v, invalid message size: %v, should between: %v TO %v, error: %v", d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize, err)
			}
			return messages, false, err
		} else {
			//invalid message size, assume current file is corrupted, try to read next file
			if d.diskQueue.cfg.AutoSkipCorruptFile {
				log.Warnf("queue:%v, offset:%v,%v, invalid message size: %v, should between: %v TO %v, offset: %v,%v",
					d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize, d.segment, d.readPos)
				nextSegment := d.segment + 1
			RETRY_NEXT_FILE:
				nextFile, exists, _ := SmartGetFileName(d.mCfg, d.queue, nextSegment)
				log.Debugf("retry skip to next file: %v, exists: %v", nextFile, exists)
				if exists || util.FileExists(nextFile) {
					//update offset
					err = d.ResetOffset(nextSegment, 0)
					Notify(d.queue, ReadComplete, d.segment)
					if err != nil {
						if strings.Contains(err.Error(), "not found") {
							return messages, false, nil
						}
						log.Errorf("queue:%v,offset:%v,%v, invalid message size: %v, should between: %v TO %v, error: %v", d.queue, d.segment, d.readPos, msgSize, d.mCfg.MinMsgSize, d.mCfg.MaxMsgSize, err)
						panic(err)
					}
					ctx.UpdateNextOffset(nextSegment, 0)
					retryTimes = 0
					goto READ_MSG //reset since we moved to next file
				} else {
					//can't read ahead before current write file
					if nextSegment >= d.diskQueue.writeSegmentNum {
						log.Debugf("need to skip to next file, but next file not exists, current write segment:%v, current read segment:%v", d.diskQueue.writeSegmentNum, d.segment)
						d.diskQueue.skipToNextRWFile(false)
						d.diskQueue.needSync = true
					} else {
						//let's continue move to next file
						nextSegment++
						log.Debugf("fetch messages move to next file: %v", nextSegment)
						goto RETRY_NEXT_FILE
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
			if d.readPos >= d.maxBytesPerFileRead || d.maxBytesPerFileRead == d.getFileSize() {
				//next file exists, and current file is EOF
				log.Debug("EOF, relocate to next file: ", nextReadPos >= d.maxBytesPerFileRead, ",", d.readPos, ",", d.maxBytesPerFileRead, ",", d.getFileSize())
				goto RELOAD_FILE
			}
			err = nil
		} else {
			log.Error(err)
		}
		return messages, false, err
	} else {

		//validate read position
		if nextReadPos > d.maxBytesPerFileRead || (d.diskQueue.writeSegmentNum == d.segment && nextReadPos > d.diskQueue.writePos) {
			stats.Increment("consumer", d.qCfg.ID, d.cCfg.ID, "invalid_message_read")

			//error only when complete loaded file
			if d.fileLoadCompleted && nextReadPos > d.maxBytesPerFileRead {
				err = errors.Errorf("the read position(%v,%v) exceed max_bytes_to_read: %v, current_write:(%v,%v)", d.segment, nextReadPos, d.maxBytesPerFileRead, d.diskQueue.writeSegmentNum, d.diskQueue.writePos)
				return messages, true, err
			}

			//file was known to not loaded completed

			//still working on the same file
			if d.diskQueue.writeSegmentNum == d.segment {
				time.Sleep(100 * time.Millisecond) // Prevent catching up too quickly.
				log.Debugf("invalid message size detected. this might be due to a dirty read as the file was being written while open. reloading segment: %d", d.segment)
			} else {
				log.Debugf("invalid message size detected. this might be due to a partial file load. reloading segment: %d", d.segment)
			}

			stats.Increment("consumer", d.qCfg.ID, d.cCfg.ID, "reload_partial_file")
			goto RELOAD_FILE
		}

		if d.mCfg.Compress.Message.Enabled {
			if global.Env().IsDebug {
				log.Tracef("decompress message: %v %v", d.fileName, d.segment)
			}
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
			Offset:     queue.NewOffsetWithVersion(d.segment, previousPos, d.version),
			NextOffset: queue.NewOffsetWithVersion(d.segment, nextReadPos, d.version),
		}

		ctx.UpdateNextOffset(d.segment, nextReadPos)

		messages = append(messages, message)
		ctx.MessageCount++
		totalMessageSize += message.Size

		if len(messages) >= d.cCfg.FetchMaxMessages || (len(messages) >= numOfMessages && numOfMessages > 0) {
			if global.Env().IsDebug {
				log.Tracef("queue:%v, consumer:%v, total messages count(%v)>=max message count(%v)", d.queue, d.cCfg.Name, len(messages), d.cCfg.FetchMaxMessages)
			}
			return messages, false, err
		}

		if totalMessageSize > d.cCfg.FetchMaxBytes && d.cCfg.FetchMaxBytes > 0 {
			if global.Env().IsDebug {
				log.Tracef("queue:%v, consumer:%v, total messages size(%v)>=max message size(%v)", d.queue, d.cCfg.Name, util.ByteSize(uint64(totalMessageSize)), util.ByteSize(uint64(d.cCfg.FetchMaxBytes)))
			}
			return messages, false, err
		}
	}

RELOAD_FILE:
	if global.Env().IsDebug {
		log.Debugf("load queue file: %v/%v, read at: %v", d.queue, d.segment, d.readPos)
	}
	if nextReadPos >= d.maxBytesPerFileRead {

		if !d.fileLoadCompleted {
			if global.Env().IsDebug {
				log.Tracef("file was load completed: %v, reload", d.fileName)
			}
			d.ResetOffset(d.segment, d.readPos)
			return messages, false, nil
		}

		if global.Env().IsDebug {
			log.Trace("try to relocate to next file: ", nextReadPos >= d.maxBytesPerFileRead, ",", d.readPos, ",", d.maxBytesPerFileRead, ",", d.getFileSize())
		}

		//check next file exists, current file is done
		nextFile, exists, _ := SmartGetFileName(d.mCfg, d.queue, d.segment+1)
		if exists || util.FileExists(nextFile) {
			//current have changes, reload file with new position
			newFileSize := d.getFileSize()
			if d.lastFileSize != newFileSize && newFileSize > d.readPos {

				//update last file size
				d.lastFileSize = newFileSize

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
			if global.Env().IsDebug {
				log.Trace("EOF, continue read:", nextFile)
			}
			Notify(d.queue, ReadComplete, d.segment)
			ctx.UpdateNextOffset(d.segment, d.readPos)
			err = d.ResetOffset(d.segment+1, 0)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					return messages, false, nil
				}
				panic(err)
			}
			retryTimes = 0 //reset since we moved to next file
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
		log.Debugf("reset offset: %v,%v, file: %v, queue:%v", segment, readPos, d.fileName, d.queue)
	}

	if segment > d.diskQueue.writeSegmentNum {
		log.Errorf("reading segment [%v] is greater than writing segment [%v]", segment, d.diskQueue.writeSegmentNum)
		return io.EOF
	}

	if segment == d.diskQueue.writeSegmentNum && readPos > d.diskQueue.writePos {
		log.Errorf("reading position [%v] is greater than writing position [%v]", readPos, d.diskQueue.writePos)
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

	fileName, exists, next_file_exists := SmartGetFileName(d.mCfg, d.queue, segment)

	//TODO, only if next file exists, and current file is not the last file, we should reload the file
	//before move to next file, make sure, the current file is loaded completely, otherwise, we may lost some messages
	if !exists {
		//double check, but next file exists
		if !util.FileExists(fileName) {
			if d.mCfg.AutoSkipCorruptFile {
				nextSegment := d.segment + 1
				if nextSegment > d.diskQueue.writeSegmentNum {
					return errors.New(fileName + " not found and next segment greater than current write segment ")
				}
				log.Warnf("queue:%v,%v, consumer:%v, offset:%v,%v, file missing: %v, auto skip to next file",
					d.qCfg.Name, d.queue, d.cCfg.Key(), d.segment, d.readPos, fileName)
			RETRY_NEXT_FILE:
				// there are segments in the middle
				if nextSegment < d.diskQueue.writeSegmentNum {
					fileName, exists, next_file_exists = SmartGetFileName(d.mCfg, d.queue, nextSegment)
					if exists || util.FileExists(fileName) {
						log.Debugf("retry skip to next file: %v, exists", fileName)
						d.segment = nextSegment
						d.readPos = 0
						d.diskQueue.UpdateSegmentConsumerInReading(d.ID, d.segment)
						goto FIND_NEXT_FILE
					} else {
						nextSegment++
						log.Debugf("retry skip to next file: %v, not exists", fileName)
						goto RETRY_NEXT_FILE
					}
				} else {
					return errors.New(fileName + " not found, next segment greater than current write segment")
				}
			} else {
				return errors.New(fileName + " not found and auto_skip_corrupt_file not enabled.")
			}
		}
		return errors.Errorf("current file: %v not found, and next_file_exists: %v.", fileName, next_file_exists)
	}

FIND_NEXT_FILE:
	//if next file exists, and current file is not the last file, the file should be completed loaded
	if next_file_exists || d.diskQueue.writeSegmentNum > segment {
		d.fileLoadCompleted = true
	} else {
		d.fileLoadCompleted = false
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

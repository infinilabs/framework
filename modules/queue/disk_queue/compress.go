/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	"os"
	"sync"
)

const queueCompressLastFileNum = "last_compress_file_for_queue"

func GetLastCompressFileNum(queueID string) int64 {
	b, err := kv.GetValue(queueCompressLastFileNum, util.UnsafeStringToBytes(queueID))
	if err != nil {
		panic(err)
	}
	if b == nil || len(b) == 0 {
		return -1
	}
	return util.BytesToInt64(b)
}

var compressFileSuffix = ".zstd"
var compressLocker =sync.RWMutex{}
func (module *DiskQueue) prepareFilesToRead(queueID string, fileNum int64) int64{
	if !module.cfg.Compress.Segment.Enabled {
		log.Tracef("segment compress for queue %v was not enabled, skip", queueID)
		return -1
	}

	var lastFile int64
	for i:=int64(1);i<=module.cfg.Compress.NumOfFilesDecompressAhead;i++{
		lastFile=int64(fileNum+int64(i))
		_,exists,_:=SmartGetFileName(module.cfg,queueID,lastFile)
		if !exists{
			return fileNum
		}
	}

	//check local
	//download if not exists

	//decompress the file if that is necessary

	return lastFile
}

func (module *DiskQueue) compressFiles(queueID string, fileNum int64) {

	//本地如果只有不到${10}个文件，文件存量太少，则不进行主动进行压缩
	//本地文件如果超过 10 个，说明堆积的比较多，可能占用太多磁盘，需要考虑压缩

	//如果开启了上传，则主动上传之前进行压缩，并删除压缩文件
	//如果没有开启上传，则只是压缩，删除原始文件，保留压缩文件
	if !module.cfg.Compress.Segment.Enabled {
		log.Tracef("segment compress for queue %v was not enabled, skip", queueID)
		return
	}

	if module.cfg.Compress.IdleThreshold < 1 {
		module.cfg.Compress.IdleThreshold = 5
	}

	//start
	consumers, earliestConsumedSegmentFileNum := module.GetEarlierOffsetByQueueID(queueID)
	fileStartToCompress := fileNum - int64(module.cfg.Compress.IdleThreshold)
	lastCompressedFileNum := GetLastCompressFileNum(queueID)

	if global.Env().IsDebug {
		log.Debugf("fileNum:%v, files start to compress:%v, last compress:%v, consumers:%v, last consumed file:%v",
			fileNum, fileStartToCompress, lastCompressedFileNum, consumers, earliestConsumedSegmentFileNum)
	}

	//skip compress file
	if fileStartToCompress <= 0 || (module.cfg.SkipZeroConsumers&&consumers <= 0) || fileStartToCompress <= lastCompressedFileNum {
		log.Debugf("skip compress %v", queueID)
		return
	}

	start:=lastCompressedFileNum
	end:=fileStartToCompress

	//has consumers
	log.Debug(queueID, " start to compress:", start,"->",end, ",consumers:", consumers, ",segment:", earliestConsumedSegmentFileNum)

	for x := start+1; x <= end; x++ {
		file := GetFileName(queueID, x)
		nextFile := GetFileName(queueID, x+1)
		if util.FileExists(file)&&util.FileExists(nextFile) {
			log.Debug("start compress queue file:", file)
			toFile := file + compressFileSuffix
			if !util.FileExists(toFile){
				//compress
				err := zstd.CompressFile(file, toFile)
				if err != nil {
					log.Error(err)
					continue
				}
				log.Debugf("queue [%v][%v] compressed", queueID, x)
			}else{
				log.Debugf("queue [%v][%v] already compressed, skip", queueID, x)
			}

			//update last mark
			err := kv.AddValue(queueCompressLastFileNum, util.UnsafeStringToBytes(queueID), util.Int64ToBytes(x))
			if err != nil {
				panic(err)
			}

			if !module.cfg.Compress.DeleteAfterCompress{
				continue
			}

			//TODO, make sure on one read this file before delete

			//if compress ahead of compressed, delete original file
			_, earliestConsumedSegmentFileNum = module.GetEarlierOffsetByQueueID(queueID)
			_, latestConsumedSegmentFileNum:= module.GetLatestOffsetByQueueID(queueID)

			log.Tracef("try to delete original file: %v, file:%v,earliest:%v,latest:%v", file,x,earliestConsumedSegmentFileNum,latestConsumedSegmentFileNum)

			//gap: delete-able| threshold+earliest| gap| latest+ threshold | delete-able
			left:=earliestConsumedSegmentFileNum-module.cfg.Compress.IdleThreshold
			right:=latestConsumedSegmentFileNum+module.cfg.Compress.IdleThreshold
			if x<left||x>right{
				log.Debugf("start to delete original file: %v, file:%v,earliest:%v,latest:%v", file,x,earliestConsumedSegmentFileNum,latestConsumedSegmentFileNum)
				err := os.Remove(file)
				if err != nil {
					log.Error(err)
					break
				}
			}
		} else {
			log.Tracef("file %v not found or next file is not ready, skip compress %v", file, queueID)
			//skip
			return
		}
	}
}
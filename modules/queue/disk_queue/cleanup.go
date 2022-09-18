/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"os"
	log "github.com/cihub/seelog"
)

func (module *DiskQueue) deleteUnusedFiles(queueID string, fileNum int64) {

	//no consumers or consumer/s3 already ahead of this file
	//TODO add config to configure none-consumers queue, to enable upload to s3 or not

	//check consumers offset
	consumers, segmentNum, _ := queue.GetEarlierOffsetByQueueID(queueID)
	fileStartToDelete := fileNum - module.cfg.Retention.MaxNumOfLocalFiles

	if fileStartToDelete <= 0 || consumers <= 0|| segmentNum<0 {
		return
	}

	//_, eSegmentNum, _ := queue.GetEarlierOffsetByQueueID(queueID) //delete files old than earliest file
	_, lSegmentNum, _ := queue.GetLatestOffsetByQueueID(queueID) //delete saved file to latest offset(keep 5 distance)


	if module.cfg.UploadToS3 {
		//check last uploaded mark
		var lastSavedFileNum = GetLastS3UploadFileNum(queueID)
		log.Trace("delete ",queueID,",",fileNum,",",consumers,",",segmentNum,",",fileStartToDelete,",",lastSavedFileNum,fileStartToDelete >= lastSavedFileNum)

		if lastSavedFileNum<0{
			return
		}

		if global.Env().IsDebug {
			log.Tracef("files start to delete:%v, consumer_on:%v, last_saved:%v", fileStartToDelete, segmentNum, lastSavedFileNum)
		}

		if fileStartToDelete >= lastSavedFileNum {
			fileStartToDelete=lastSavedFileNum-module.cfg.Compress.IdleThreshold
		}

		if  lastSavedFileNum - lSegmentNum > module.cfg.Compress.IdleThreshold{
			log.Tracef("files start to saved:%v, latest:%v", lastSavedFileNum, lSegmentNum)
			//TODO foreach delete files
		}


	}

	//has consumers
	if consumers > 0 && fileStartToDelete < segmentNum && segmentNum>0 {
		log.Trace(queueID, " start to delete:", fileStartToDelete, ",consumers:", consumers, ",consumer_on:", segmentNum)

		//TODO do not wall all files, when numbers growing, it will be slow to check each file exists or not
		for x := fileStartToDelete; x >= 0; x-- {
			file := GetFileName(queueID, x)
			log.Trace(queueID, " start to delete:", file)
			if util.FileExists(file) {
				log.Trace("delete queue file:", file)
				err := os.Remove(file)
				if err != nil {
					panic(err)
				}
			}
			compressedFile := file + compressFileSuffix
			if util.FileExists(compressedFile) {
				log.Trace("delete compressed queue file:", compressedFile)
				err := os.Remove(compressedFile)
				if err != nil {
					panic(err)
				}
			}

		}
	} else {
		//FIFO queue, need to delete old files
		//log.Errorf("queue:%v, fileID:%v, file start to delete:%v , segment num:%v",queueID,fileNum,fileStartToDelete,segmentNum)
		//check current read depth and file num
	}

}

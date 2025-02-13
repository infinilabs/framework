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
	"os"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

func (module *DiskQueue) GetEarlierOffsetByQueueID(queueID string) (int, int64) {
	consumers, eSegmentNum, _, _ := queue.GetEarlierOffsetByQueueID(queueID)
	q, ok := module.queues.Load(queueID)
	if ok {
		var c = 0
		(q.(*DiskBasedQueue)).consumersInReading.Range(func(key, value any) bool {
			seg := value.(int64)
			c++
			if seg < eSegmentNum {
				if global.Env().IsDebug {
					log.Debug(queueID, ",", seg, " < ", eSegmentNum, " use:", seg)
				}
				eSegmentNum = seg
			}
			return true
		})
		if c > consumers {
			consumers = c
		}
	}
	return consumers, eSegmentNum
}

func (module *DiskQueue) GetLatestOffsetByQueueID(queueID string) (int, int64) {
	consumers, eSegmentNum, _ := queue.GetLatestOffsetByQueueID(queueID)
	q, ok := module.queues.Load(queueID)
	if ok {
		var c = 0
		(q.(*DiskBasedQueue)).consumersInReading.Range(func(key, value any) bool {
			seg := value.(int64)
			c++
			if seg > eSegmentNum {
				if global.Env().IsDebug {
					log.Trace(queueID, ",", seg, " > ", eSegmentNum, " use:", seg)
				}
				eSegmentNum = seg
			}
			return true
		})
		if c > consumers {
			consumers = c
		}
	}
	return consumers, eSegmentNum
}

func (module *DiskQueue) deleteUnusedFiles(queueID string, fileNum int64) {

	//no consumers or consumer/s3 already ahead of this file
	//TODO add config to configure none-consumers queue, to enable upload to s3 or not

	//check consumers offset
	consumers, eSegmentNum := module.GetEarlierOffsetByQueueID(queueID)
	fileStartToDelete := fileNum - module.cfg.Retention.MaxNumOfLocalFiles

	if fileStartToDelete <= 0 || consumers <= 0 || eSegmentNum < 0 {
		log.Debugf("queue: %v, no consumers or consumer/s3 already ahead of this file, %v, %v, %v", queueID, fileStartToDelete, consumers, eSegmentNum)
		return
	}

	_, lSegmentNum := module.GetLatestOffsetByQueueID(queueID) //delete saved file to latest offset(keep 5 distance)

	if module.cfg.UploadToS3 {
		//check last uploaded mark
		var lastSavedFileNum = GetLastS3UploadFileNum(queueID)
		log.Trace("disk, delete ", queueID, ",", fileNum, ",", consumers, ",", eSegmentNum, ",", fileStartToDelete, ",", lastSavedFileNum, fileStartToDelete >= lastSavedFileNum)

		if lastSavedFileNum < 0 {
			return
		}

		if global.Env().IsDebug {
			log.Debugf("disk, files start to delete:%v, consumer_on:%v, last_saved:%v", fileStartToDelete, eSegmentNum, lastSavedFileNum)
		}

		if fileStartToDelete >= lastSavedFileNum {
			fileStartToDelete = lastSavedFileNum - module.cfg.Compress.IdleThreshold
		}

		if lastSavedFileNum-lSegmentNum > module.cfg.Compress.IdleThreshold {
			log.Tracef("disk, files start to saved:%v, latest:%v", lastSavedFileNum, lSegmentNum)
			//TODO foreach delete files
		}
	}

	if eSegmentNum > 0 && fileStartToDelete > eSegmentNum {
		fileStartToDelete = eSegmentNum - module.cfg.Retention.MaxNumOfLocalFiles
	}

	//has consumers
	if consumers > 0 && fileStartToDelete > 0 && fileStartToDelete < eSegmentNum && eSegmentNum > 0 {
		log.Trace(queueID, " start to delete:", fileStartToDelete, ",consumers:", consumers, ",consumer_on:", eSegmentNum)

		//TODO do not wall all files, when numbers growing, it will be slow to check each file exists or not
		for x := fileStartToDelete; x >= 0; x-- {

			//if file greater than earliest segment, skip
			if x > eSegmentNum {
				continue
			}

			file := GetFileName(queueID, x)
			log.Trace(queueID, " start to delete:", file)
			var exists = false
			if util.FileExists(file) {
				exists = true
				log.Trace("delete queue file:", file)
				err := os.Remove(file)
				if err != nil {
					log.Error(err)
					break
				}
			}
			compressedFile := file + compressFileSuffix
			if util.FileExists(compressedFile) {
				exists = true
				log.Trace("delete compressed queue file:", compressedFile)
				err := os.Remove(compressedFile)
				if err != nil {
					log.Error(err)
					break
				}
			}

			//no compress or flat file exists
			if !exists {
				log.Tracef("continue further delete, missing queue file:", file)
				continue
			}
		}
	} else {
		//FIFO queue, need to delete old files
		log.Debugf("skip delete, queue:%v, consumers:%v, fileID:%v, file start to delete:%v , segment num:%v", queueID, consumers, fileNum, fileStartToDelete, eSegmentNum)
		//check current read depth and file num
	}

}

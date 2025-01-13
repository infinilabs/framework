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
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	"path"
	"sync"
)

const queueS3LastFileNum = "last_success_file_for_queue"

func GetLastS3UploadFileNum(queueID string) int64 {
	b, err := kv.GetValue(queueS3LastFileNum, util.UnsafeStringToBytes(queueID))
	if err != nil {
		panic(err)
	}
	if b == nil || len(b) == 0 {
		return -1
	}
	return util.BytesToInt64(b)
}

func getS3FileLocation(fileName string) string {
	return path.Join(global.Env().SystemConfig.NodeConfig.ID, util.TrimLeftStr(fileName, global.Env().GetDataDir()))
}

var s3uploaderLocker sync.RWMutex

func (module *DiskQueue) uploadToS3(queueID string, fileNum int64) {

	log.Trace("try uploaded id:", queueID, ",", fileNum)

	s3uploaderLocker.Lock()
	defer s3uploaderLocker.Unlock()

	//send s3 upload signal
	if module.cfg.UploadToS3 {

		consumers, _ := module.GetEarlierOffsetByQueueID(queueID)
		if module.cfg.SkipZeroConsumers && consumers == 0 {
			//skip upload queue without any consumers
			return
		}

		//skip uploaded file
		lastFileNum := GetLastS3UploadFileNum(queueID)
		log.Tracef("queue:%v, last upload:%v, fileNum:%v", queueID, lastFileNum, fileNum)
		if fileNum <= lastFileNum {
			//skip old queue file, no need to upload
			return
		}

		log.Trace(queueID, " last uploaded id:", lastFileNum)

		if module.cfg.S3.Server != "" && module.cfg.S3.Bucket != "" {
			for i := lastFileNum + 1; i <= fileNum; i++ {
				//TODO skip recent file
				log.Trace(queueID, " upload id:", i)
				fileName := GetFileName(queueID, i)
				if module.cfg.Compress.Segment.Enabled {
					tempFile := fileName + compressFileSuffix
					if util.FileExists(tempFile) {
						fileName = tempFile
					} else {
						//compress before upload
						log.Debug("compressed file should exists, maybe is not yet ready, ", tempFile)
						continue
					}
				}

				toFile := getS3FileLocation(fileName)

				var success = false
				var err error
				if module.cfg.S3.Async {
					err := s3.AsyncUpload(fileName, module.cfg.S3.Server, module.cfg.S3.Location, module.cfg.S3.Bucket, toFile)
					if err != nil {
						log.Error(err)
					} else {
						success = true
					}
				} else {
					var ok bool
					ok, err = s3.SyncUpload(fileName, module.cfg.S3.Server, module.cfg.S3.Location, module.cfg.S3.Bucket, toFile)
					if err != nil {
						log.Error(err)
					} else if ok {
						success = true
					}
				}
				//update last mark
				if success {
					err = kv.AddValue(queueS3LastFileNum, util.UnsafeStringToBytes(queueID), util.Int64ToBytes(i))
					if err != nil {
						panic(err)
					}
					log.Debugf("queue [%v][%v] uploaded to s3", queueID, i)
				} else {
					log.Debugf("failed to upload queue [%v][%v] to s3, %v", queueID, i, err)
				}
			}
		} else {
			log.Errorf("invalid s3 config:%v", module.cfg.S3)
		}
	}
}

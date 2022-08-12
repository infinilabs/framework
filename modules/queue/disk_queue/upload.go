/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"path"
	log "github.com/cihub/seelog"
	"sync"
	"infini.sh/framework/core/s3"
)

const queueS3LastFileNum ="last_success_file_for_queue"

func GetLastS3UploadFileNum(queueID string)int64  {
	b,err:=kv.GetValue(queueS3LastFileNum,util.UnsafeStringToBytes(queueID))
	if err!=nil{
		panic(err)
	}
	if b==nil||len(b)==0{
		return -1
	}
	//log.Errorf("bytes to int64: %v",b)
	return util.BytesToInt64(b)
}

func getS3FileLocation(fileName string) string {
	return path.Join(global.Env().SystemConfig.NodeConfig.ID,util.TrimLeftStr(fileName,global.Env().GetDataDir()))
}

var s3uploaderLocker sync.RWMutex

func (module *DiskQueue)uploadToS3(queueID string,fileNum  int64){

	//TODO move to channel, async
	s3uploaderLocker.Lock()
	defer s3uploaderLocker.Unlock()

	//send s3 upload signal
	if module.cfg.UploadToS3{

		consumers,_,_:=queue.GetEarlierOffsetByQueueID(queueID)
		if consumers==0{
			//skip upload queue without any consumers
			return
		}

		//skip uploaded file
		lastFileNum:= GetLastS3UploadFileNum(queueID)
		log.Tracef("last upload:%v, fileNum:%v",lastFileNum, fileNum)
		if fileNum<=lastFileNum{
			//skip old queue file, no need to upload
			return
		}

		if module.cfg.S3.Server!=""&&module.cfg.S3.Bucket!=""{
			fileName:= GetFileName(queueID,fileNum)
			toFile:= getS3FileLocation(fileName)
			var success=false
			var err error
			if module.cfg.S3.Async{
				err:=s3.AsyncUpload(fileName,module.cfg.S3.Server,module.cfg.S3.Location,module.cfg.S3.Bucket,toFile)
				if err!=nil{
					log.Error(err)
				}else{
					success=true
				}
			}else{
				var ok bool
				ok,err=s3.SyncUpload(fileName,module.cfg.S3.Server,module.cfg.S3.Location,module.cfg.S3.Bucket,toFile)
				if err!=nil{
					log.Error(err)
				}else if ok{
					success=true
				}
			}
			//update last mark
			if success{
				err=kv.AddValue(queueS3LastFileNum,util.UnsafeStringToBytes(queueID),util.Int64ToBytes(fileNum))
				if err!=nil{
					panic(err)
				}
				log.Debugf("queue [%v][%v] uploaded to s3",queueID,fileNum)
			}else{
				log.Debugf("failed to upload queue [%v][%v] to s3, %v",queueID,fileNum,err)
			}
		}else{
			log.Errorf("invalid s3 config:%v",module.cfg.S3)
		}
	}

}


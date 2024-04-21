/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"infini.sh/framework/core/s3"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	log "github.com/cihub/seelog"
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
				log.Tracef("decompress file: %v", compressedFile)
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
					log.Tracef("decompress file: %v", fileToDownload)
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
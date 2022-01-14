/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package s3

import (
	"infini.sh/framework/core/errors"
)

type S3 interface {
	SyncUpload(filePath,location,bucketName,objectName string)(bool,error)
	AsyncUpload(filePath,location,bucketName,objectName string) error
}

var s3Uploader = map[string]S3{}

func Register(serverID string,s3 S3)  {
	s3Uploader[serverID]=s3
}

func SyncUpload(filePath,serverID,location,bucketName,objectName string)(bool,error){
	handler, ok := s3Uploader[serverID]
	if ok {
		return handler.SyncUpload(filePath,location,bucketName,objectName)
	}
	panic(errors.Errorf("s3 server [%v] was not found",serverID))
}

func AsyncUpload(filePath,serverID,location,bucketName,objectName string) error {
	handler, ok := s3Uploader[serverID]
	if ok {
		return handler.AsyncUpload(filePath,location,bucketName,objectName)
	}
	panic(errors.Errorf("s3 server [%v] was not found",serverID))
}

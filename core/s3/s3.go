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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package s3

import (
	"infini.sh/framework/core/errors"
)

type S3 interface {
	SyncDownload(filePath, location, bucketName, objectName string) (bool, error)
	SyncUpload(filePath, location, bucketName, objectName string) (bool, error)
	AsyncUpload(filePath, location, bucketName, objectName string) error
}

var s3Uploader = map[string]S3{}

func Register(serverID string, s3 S3) {
	s3Uploader[serverID] = s3
}

func SyncUpload(filePath, serverID, location, bucketName, objectName string) (bool, error) {
	handler, ok := s3Uploader[serverID]
	if ok {
		return handler.SyncUpload(filePath, location, bucketName, objectName)
	}
	panic(errors.Errorf("s3 server [%v] was not found", serverID))
}

func AsyncUpload(filePath, serverID, location, bucketName, objectName string) error {
	handler, ok := s3Uploader[serverID]
	if ok {
		return handler.AsyncUpload(filePath, location, bucketName, objectName)
	}
	panic(errors.Errorf("s3 server [%v] was not found", serverID))
}

func SyncDownload(filePath, serverID, location, bucketName, objectName string) (bool, error) {
	handler, ok := s3Uploader[serverID]
	if ok {
		return handler.SyncDownload(filePath, location, bucketName, objectName)
	}
	panic(errors.Errorf("s3 server [%v] was not found", serverID))
}

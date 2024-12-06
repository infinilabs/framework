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

package common

import (
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/queue"
	"github.com/rubyniu105/framework/core/util"
	"os"
	"path"
	"sync"
)

func GetLocalQueueConfigPath() string {
	err := os.MkdirAll(path.Join(global.Env().GetDataDir(), "queue"), 0755)
	if err != nil {
		panic(err)
	}
	return path.Join(global.Env().GetDataDir(), "queue", "configs")
}

var persistentLocker sync.RWMutex

func PersistQueueMetadata() {
	persistentLocker.Lock()
	defer persistentLocker.Unlock()

	//persist configs to local store
	bytes := queue.GetAllConfigBytes()
	path1 := GetLocalQueueConfigPath()
	if util.FileExists(path1) {
		_, err := util.CopyFile(path1, path1+".bak")
		if err != nil {
			panic(err)
		}
	}
	_, err := util.FilePutContentWithByte(path1, bytes)
	if err != nil {
		panic(err)
	}
}

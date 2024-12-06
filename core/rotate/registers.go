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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package rotate

import (
	"sync"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/global"
)

var fileHandlers = map[string]*RotateWriter{}
var lock sync.RWMutex
var callbackRegistered bool

type RotateConfig struct {
	Compress     bool `config:"compress_after_rotate"`
	MaxFileAge   int  `config:"max_file_age"`
	MaxFileCount int  `config:"max_file_count"`
	MaxFileSize  int  `config:"max_file_size_in_mb"`
}

var DefaultConfig = RotateConfig{
	Compress:     true,
	MaxFileAge:   0,
	MaxFileCount: 10,
	MaxFileSize:  100,
}

func GetFileHandler(path string, config RotateConfig) *RotateWriter {
	lock.Lock()
	defer lock.Unlock()

	if !callbackRegistered {
		global.RegisterShutdownCallback(func() {
			Close()
		})
		callbackRegistered = true
	}
	v, ok := fileHandlers[path]
	if !ok {
		v = &RotateWriter{
			Filename:         path,
			Compress:         config.Compress,
			MaxFileAge:       config.MaxFileAge,
			MaxRotationCount: config.MaxFileCount,
			MaxFileSize:      config.MaxFileSize,
		}
		fileHandlers[path] = v
	}
	return v
}

func ReleaseFileHandler(path string) {
	lock.Lock()
	defer lock.Unlock()

	v, ok := fileHandlers[path]
	if ok {
		v.Close()
		delete(fileHandlers, path)
	}
}
func Close() {
	for k, v := range fileHandlers {
		log.Trace("closing rotate writer: ", k)
		err := v.Close()
		if err != nil {
			log.Error(err)
		}
	}
}

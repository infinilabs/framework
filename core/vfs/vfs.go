/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vfs

import (
	"net/http"
	"os"
	"sync"
)

func VFS() http.FileSystem {
	return VirtualFS{}
}

type VirtualFS struct{}

var vfs []http.FileSystem
var lock sync.Mutex

func RegisterFS(fs http.FileSystem) {
	lock.Lock()
	vfs = append([]http.FileSystem{fs}, vfs...)
	lock.Unlock()
}

func (VirtualFS) Open(name string) (http.File, error) {

	for _, v := range vfs {
		f1, err := v.Open(name)
		if err == nil {
			return f1, err
		}
	}
	return nil, os.ErrNotExist
}

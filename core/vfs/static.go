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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
)

func (fs StaticFS) prepare(name string) (*VFile, error) {
	name = path.Clean(name)
	f, present := data[name]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	fs.once.Do(func() {
		f.FileName = path.Base(name)

		if f.FileSize == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			log.Error(err)
			return
		}
		f.Data, err = ioutil.ReadAll(gr)

	})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return f, nil
}

func (fs StaticFS) Open(name string) (http.File, error) {

	name = path.Clean(name)

	if fs.CheckLocalFirst {

		name = util.TrimLeftStr(name, fs.TrimLeftPath)

		localFile := path.Join(fs.StaticFolder, name)

		log.Trace("check local file, ", localFile)

		if util.FileExists(localFile) {

			f2, err := os.Open(localFile)
			if err == nil {
				return f2, err
			}
		}

		log.Debug("local file not found,", localFile)
	}

	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

type StaticFS struct {
	once            sync.Once
	StaticFolder    string
	TrimLeftPath    string
	CheckLocalFirst bool
}

var data = map[string]*VFile{}

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

package fs

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
)

var once sync.Once

func (StaticFS) prepare(name string) (*VFile, error) {
	f, present := data[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	once.Do(func() {
		f.FileName = path.Base(name)
		if f.FileSize == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.Data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs StaticFS) Open(name string) (http.File, error) {

	if fs.CheckLocalFirst {
		p := path.Join(fs.BaseFolder, ".", path.Clean(name))
		f2, err := os.Open(p)
		if err == nil {
			return f2, err
		}
	}

	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

type StaticFS struct {
	BaseFolder      string
	CheckLocalFirst bool
}

var data = map[string]*VFile{

	"/": {
		IsFolder: true,
		FileName: "/",
	},
}

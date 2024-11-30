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

// copied from: https://godoc.org/github.com/mjibson/esc

package vfs

import (
	"bytes"
	"net/http"
	"os"
	"time"
)

type VDirectory struct {
	fs   http.FileSystem
	name string
}

type VFile struct {
	Compressed string
	FileSize   int64
	ModifyTime int64
	IsFolder   bool

	Data     []byte
	FileName string
}

func (dir VDirectory) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *VFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*VFile
	}
	return &httpFile{
		Reader: bytes.NewReader(f.Data),
		VFile:  f,
	}, nil
}

func (f *VFile) Close() error {
	return nil
}

func (f *VFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *VFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *VFile) Name() string {
	return f.FileName
}

func (f *VFile) Size() int64 {
	return f.FileSize
}

func (f *VFile) Mode() os.FileMode {
	return 0
}

func (f *VFile) ModTime() time.Time {
	return time.Unix(f.ModifyTime, 0)
}

func (f *VFile) IsDir() bool {
	return f.IsFolder
}

func (f *VFile) Sys() interface{} {
	return f
}

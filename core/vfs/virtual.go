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

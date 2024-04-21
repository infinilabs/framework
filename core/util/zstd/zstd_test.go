/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package zstd

import (
	"infini.sh/framework/core/util"
	"os"
	"path"
	"sync"
	"testing"
)

func TestZSTDBytes(t *testing.T) {
	src:="Hello, World!"
	compressed:=CompressBytes([]byte(src))
	decompressed,err:=DecompressBytes(compressed)
	if err!=nil{
		t.Error(err)
	}
	if string(decompressed)!=src{
		t.Error("Decompression failed")
	}
}

func TestZSTDFiles(t *testing.T) {
	src:="Hello, World!"
	srcFile:=path.Join(os.TempDir(),util.GetUUID()+"zstd_test.txt")
	_,err:=util.FilePutContent(srcFile,src)
	if err!=nil{
		t.Error(err)
	}

	compressedFile:=path.Join(os.TempDir(),util.GetUUID()+"zstd_test.zst")
	err=CompressFile(srcFile,compressedFile)
	if err!=nil{
		t.Error(err)
	}

	decompressedFile:=path.Join(os.TempDir(),util.GetUUID()+"zstd_test.txt")
	locker := &sync.RWMutex{}
	err=DecompressFile(locker,compressedFile,decompressedFile)
	if err!=nil{
		t.Error(err)
	}

	filedContent,err:=util.FileGetContent(decompressedFile)
	if err!=nil{
		t.Error(err)
	}
	println(string(filedContent))
	if string(filedContent)!=src{
		t.Error("Decompression failed")
	}

	err=os.Remove(srcFile)
	if err!=nil{
		t.Error(err)
	}
	err=os.Remove(compressedFile)
	if err!=nil{
		t.Error(err)
	}

}
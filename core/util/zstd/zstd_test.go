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
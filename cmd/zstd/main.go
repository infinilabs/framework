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

//https://github.com/dgraph-io/badger/pull/1706/files

package main

import (
	"flag"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	log "github.com/cihub/seelog"
	"sync"
)



func main() {

	path := flag.String("path", "data", "the data path")
	op := flag.String("op", "compress", "compress or decompress")
	flag.Parse()

	files,err:=util.ListAllFiles(*path)
	if err != nil {
		panic(err)
	}

	if *op=="compress"{
		for _, file := range files {
			if !util.SuffixStr(file,suffix){
				err:=zstd.CompressFile(file,file+suffix)
				if err!=nil{
					log.Error(err)
				}
			}
		}
	}else{
		for _, file := range files {
			if util.SuffixStr(file,suffix){
				err:=zstd.DecompressFile(&locker,file,util.TrimRightStr(file,suffix))
				if err!=nil{
					log.Error(err)
				}
			}
		}
	}

}
var suffix=".zstd"
var locker=sync.RWMutex{}
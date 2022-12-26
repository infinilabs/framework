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
	"time"
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
				log.Error(err)
			}
		}
	}else{
		for _, file := range files {
			if util.SuffixStr(file,suffix){
				err:=zstd.DecompressFile(&locker,file,util.TrimRightStr(file,suffix))
				log.Error(err)
			}
		}
	}

	time.Sleep(time.Second)
}
var suffix=".zstd"
var locker=sync.RWMutex{}
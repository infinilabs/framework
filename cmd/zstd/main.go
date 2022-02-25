/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

//https://github.com/dgraph-io/badger/pull/1706/files

package main

import (
	"fmt"
	"infini.sh/framework/core/util"
)

func main() {

	file:="/Users/medcl/Downloads/c8boc1hu46lgcqap7jq0.diskqueue.000200.dat"
	rawBytes,err:=util.FileGetContent(file)
	if err!=nil{
		panic(err)
	}
	newBytes,err:=util.ZSTDCompress(nil,rawBytes,11)
	if err!=nil{
		panic(err)
	}
	fmt.Println(len(rawBytes))
	fmt.Println(len(newBytes))
}


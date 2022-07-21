/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

//https://github.com/dgraph-io/badger/pull/1706/files

package main

import (
	"bytes"
	"fmt"
	"infini.sh/framework/core/util"
	"infini.sh/framework/core/util/zstd"
	"infini.sh/framework/lib/bytebufferpool"
)

func main() {

	file := "/Users/medcl/Downloads/c8boc1hu46lgcqap7jq0.diskqueue.000200.dat"
	rawBytes, err := util.FileGetContent(file)
	if err != nil {
		panic(err)
	}
	newBytes, err := zstd.ZSTDCompress(nil, rawBytes, 11)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(rawBytes))
	fmt.Println(len(newBytes))

	fmt.Println("compression ratio:", len(rawBytes)/len(newBytes))

	//decompress
	r := bytes.Reader{}
	r.Reset(newBytes)
	writer := bytebufferpool.Get("zstd")
	zstd.ZSTDReusedDecompress(writer, &r)

	fmt.Println("decompressed:", len(writer.Bytes()))

	r = bytes.Reader{}
	r.Reset(rawBytes)
	writer = bytebufferpool.Get("zstd")
	zstd.ZSTDReusedCompress(writer, &r)
	fmt.Println("compressed:", len(writer.Bytes()))

	fmt.Println("compression ratio:", len(rawBytes)/len(writer.Bytes()))

}

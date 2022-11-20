/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"github.com/OneOfOne/xxhash"
)

func XXHash(data string)uint32  {
	hash := xxhash.New32()
	hash.Write(UnsafeStringToBytes(data))
	return hash.Sum32()
}

func ModString(data string,max int) int {
	hash:=int(XXHash(data))
	return int(hash%max)
}
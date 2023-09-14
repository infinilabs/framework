/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"fmt"
	"src/github.com/magiconair/properties/assert"
	"testing"
)

func TestSortKeyValueArray(t *testing.T) {

	kv:=[]KeyValue{}
	kv=append(kv,KeyValue{"a",1})
	kv=append(kv,KeyValue{"b",2})
	kv=append(kv,KeyValue{"c",3})

	kv=SortKeyValueArray(kv,false)

	fmt.Println(kv)
	assert.Equal(t, kv[0].Key, "c")

	kv=SortKeyValueArray(kv,true)

	fmt.Println(kv)
	assert.Equal(t, kv[0].Key, "a")
}

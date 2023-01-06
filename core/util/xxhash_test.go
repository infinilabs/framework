/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/OneOfOne/xxhash"
	"sync"
	"testing"
)

func BenchmarkReuseHash(b *testing.B) {
	var xxHashPool= sync.Pool{
		New: func() interface{} {
			return xxhash.New32()
		},
	}
	hash := xxHashPool.Get().(*xxhash.XXHash32)
	defer xxHashPool.Put(hash)

	//assert.Equal(t, p, 2)

	for i := 0; i < b.N; i++ {
		hash.Reset()
		hash.WriteString(fmt.Sprintf("my-%v",i))
		h:= int(hash.Sum32())
		Mod(h,5)
	}

}

func TestModString(t *testing.T) {
	str:="abc"
	p:=ModString(str,5)
	fmt.Println(p)
	assert.Equal(t, p, 2)
}

func BenchmarkModString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ModString(fmt.Sprintf("my-%v",i), 5)
	}
}
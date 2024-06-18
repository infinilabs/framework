/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package bulk_indexing

import (
	"github.com/OneOfOne/xxhash"
	"github.com/stretchr/testify/assert"
	"testing"
)


func TestXXHash(t *testing.T) {
	inputs := []string{
		"cpk3nk78mrlpjqkd72rg",
		"cpk3njv8mrlpjqkd7170",
		"cpk3nk78mrlpjqkd72t0",
		"cpk3nkf8mrlpjqkd7310",
		"cpk3nkf8mrlpjqkd73dg",
		"cpk3nkn8mrlpjqkd7460",
		"cpk3nkn8mrlpjqkd7461",
		"cpk3nkn8mrlpjqkd7462",
		"cpk3nkn8mrlpjqkd7463",
		"cpk3nkn8mrlpjqkd7464",
		"cpk3nkn8mrlpjqkd7465",
		"cpk3nkn8mrlpjqkd7466",
		"A",
		"B",
		"C",
		"D",
		"E",
	}
	hash := []int{
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		2,
		2,
		1,
		1,
		0,
		0,
		1,
		2,
		1,
		0,
	}
	xxHash := xxHashPool.Get().(*xxhash.XXHash32)
	defer xxHashPool.Put(xxHash)
	for o,i:=range inputs{
		xxHash.Reset()
		xxHash.WriteString(i)
		hashValue:= int(xxHash.Sum32())
		println(hashValue)
		println(hashValue%3)
		assert.Equal(t,hashValue%3,hash[o])
	}

}
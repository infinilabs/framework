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

package util

import (
	"fmt"
	"github.com/OneOfOne/xxhash"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func BenchmarkReuseHash(b *testing.B) {
	var xxHashPool = sync.Pool{
		New: func() interface{} {
			return xxhash.New32()
		},
	}
	hash := xxHashPool.Get().(*xxhash.XXHash32)
	defer xxHashPool.Put(hash)

	//assert.Equal(t, p, 2)

	for i := 0; i < b.N; i++ {
		hash.Reset()
		hash.WriteString(fmt.Sprintf("my-%v", i))
		h := int(hash.Sum32())
		Mod(h, 5)
	}

}

func TestModString(t *testing.T) {
	str := "abc"
	p := ModString(str, 5)
	fmt.Println(p)
	assert.Equal(t, p, 2)
}

func BenchmarkModString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ModString(fmt.Sprintf("my-%v", i), 5)
	}
}

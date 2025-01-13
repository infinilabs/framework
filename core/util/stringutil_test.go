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

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMergeSpacke(t *testing.T) {
	str := "hello world     !"
	str = MergeSpace(str)
	assert.Equal(t, "hello world !", str)

	str = " hello world  \n \r  !"
	str = MergeSpace(str)
	assert.Equal(t, "hello world !", str)
}

func TestTrimSpaces(t *testing.T) {
	str := " left"
	assert.Equal(t, "left", TrimSpaces(str))

	str = "right "
	assert.Equal(t, "right", TrimSpaces(str))

	str = " side "
	assert.Equal(t, "side", TrimSpaces(str))

	str = "midd le"
	assert.Equal(t, "middle", RemoveSpaces(str))
}

func TestRemoveSpaces(t *testing.T) {
	str := " left"
	assert.Equal(t, "left", RemoveSpaces(str))

	str = "right "
	assert.Equal(t, "right", RemoveSpaces(str))

	str = " side "
	assert.Equal(t, "side", RemoveSpaces(str))

	str = "midd le"
	assert.Equal(t, "middle", RemoveSpaces(str))
}

func TestTrimLeft(t *testing.T) {
	str := "static://off"
	new := TrimLeftStr(str, "static:")
	assert.Equal(t, "//off", new)

	str = "/static/static://off"
	new = TrimLeftStr(str, "/static")
	assert.Equal(t, "/static://off", new)
}

func TestToInt64(t *testing.T) {
	str := "6393600409"
	new, _ := ToInt64(str)
	fmt.Println(new)
}

func TestContainsAnyInAnyIntArray(t *testing.T) {
	str := "key"
	strs := []interface{}{}
	strs = append(strs, "key", "key1")
	ok := ContainsAnyInAnyIntArray(str, strs)
	fmt.Println(ok)
	assert.Equal(t, true, ok)

	str = "key3"
	ok = ContainsAnyInAnyIntArray(str, strs)
	fmt.Println(ok)
	assert.Equal(t, false, ok)

	str1 := 5
	strs2 := []interface{}{}
	strs2 = append(strs2, 5, 6)
	ok2 := ContainsAnyInAnyIntArray(str1, strs2)
	fmt.Println(ok2)
	assert.Equal(t, true, ok2)

	str1 = 7
	ok2 = ContainsAnyInAnyIntArray(str1, strs2)
	fmt.Println(ok2)
	assert.Equal(t, false, ok2)
}

func TestContainsString(t *testing.T) {
	a := "{\"error\":true,\"message\":\"timeout\"}"
	b := "\"message\":\"timeout\""
	ok := ContainStr(a, b)
	fmt.Println(ok)
	assert.Equal(t, true, ok)
}

func TestInt64ToString(t *testing.T) {
	i := 6393600409
	new := Int64ToString(int64(i))
	fmt.Println(new)
	assert.Equal(t, "6393600409", new)

	i = 63
	new = Int64ToString(int64(i))
	fmt.Println(new)
	assert.Equal(t, "63", new)
}

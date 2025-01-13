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
	"path"
	"path/filepath"
	"testing"
)

func TestJoinPath1(t *testing.T) {
	path1 := "wwww.baidu.com"
	path2 := "/blog/"
	path3 := "/comments/1.html"
	str := path.Join(path1, path2, path3)
	fmt.Println(str)
	assert.Equal(t, "wwww.baidu.com/blog/comments/1.html", str)
}

func TestJoinPath2(t *testing.T) {
	path1 := "wwww.baidu.com"
	path2 := "/blog/"
	path3 := "/comments/1.html"
	str := JoinPath(path1, path2, path3)
	fmt.Println(str)
	assert.Equal(t, "wwww.baidu.com/blog/comments/1.html", str)
}

func TestIsFileWithinFolder(t *testing.T) {
	file := "xxx.exe"
	folder := "/blog/"
	assert.Equal(t, true, IsFileWithinFolder(path.Join(folder, file), folder))

	file = "/danger/xxx.exe"
	folder = "/blog/"
	assert.Equal(t, true, IsFileWithinFolder(path.Join(folder, file), folder))

	file = "../../../danger/xxx.exe"
	folder = "/blog/"

	v1 := path.Join(folder, file)
	v2 := folder
	fmt.Println(filepath.Abs(v1))
	fmt.Println(filepath.Abs(v2))

	assert.Equal(t, false, IsFileWithinFolder(v1, folder))

}

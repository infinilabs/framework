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
	"sync"
	"testing"
	"time"
)

func BenchmarkGetIncrementID(b *testing.B) {
	fmt.Println(GetIncrementID("a"))
	fmt.Println(GetIncrementID("a"))

	for i := 0; i < b.N; i++ {
		GetIncrementID("a")
	}
}

func TestIDGenerator(t *testing.T) {
	var set = map[string]interface{}{}
	var s = sync.RWMutex{}
	for j := 0; j < 50; j++ {
		go func() {
			for i := 0; i < 5000000; i++ {
				id := GetUUID()
				s.Lock()
				if _, ok := set[id]; ok {
					panic(id)
				} else {
					set[id] = true
				}
				s.Unlock()
			}
		}()
	}
	time.Sleep(3 * time.Second)

}

func BenchmarkGetUUID(t *testing.B) {

	for i := 0; i < t.N; i++ {
		GetUUID()
	}
}

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

package stats

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestGoroutinesInfo(t *testing.T) {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)

	stacks := strings.Split(string(buf[:n]), "\n\n")
	grouped := make(map[string]int)
	patternMem, err := regexp.Compile("\\+?0x[\\d\\w]+")
	if err != nil {
		panic(err)
	}
	patternID, err := regexp.Compile("^goroutine \\d+")
	if err != nil {
		panic(err)
	}

	patternNewID, err := regexp.Compile("^goroutine ID")
	if err != nil {
		panic(err)
	}
	for _, stack := range stacks {
		newStack := patternMem.ReplaceAll([]byte(stack), []byte("_address_"))
		newStack = patternID.ReplaceAll([]byte(newStack), []byte("goroutine ID"))
		grouped[string(newStack)]++
	}

	for funcPath, count := range grouped {
		str := patternNewID.ReplaceAllString(funcPath, fmt.Sprintf("%v same instance of goroutines", count))
		fmt.Printf("%v\n", str)
	}
}

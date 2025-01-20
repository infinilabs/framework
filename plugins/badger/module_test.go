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

package badger

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

const filterKey = "testFilter"

func Test(t *testing.T) {
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = "/tmp/filter_" + util.PickRandomName()
	os.RemoveAll(env1.SystemConfig.PathConfig.Data)
	env1.IsDebug = true
	global.RegisterEnv(env1)

	m := Module{}
	m.Setup()
	m.Start()
	b, _ := filter.CheckThenAdd(filterKey, []byte("key"))
	assert.Equal(t, false, b)

	//err=filter.Add(filterKey,[]byte("key"))
	//fmt.Println(err)
	//ok:=filter.Exists(filterKey,[]byte("key"))
	//fmt.Println(ok)

	//Memory pressure test
	for i := 0; i < 1; i++ {
		go run(i, t)
	}

	time.Sleep(10 * time.Second)

	//For BoltDB KV filter, 19k unique will consume 100MB memory, 40K:230MB
}

func run(seed int, t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	for i := 0; i < 100000000; i++ {
		fmt.Println(i)
		k := fmt.Sprintf("key-%v-%v", seed, i)
		b := filter.Exists(filterKey, []byte(k))
		assert.Equal(t, false, b)
		b, _ = filter.CheckThenAdd(filterKey, []byte(k))
		assert.Equal(t, false, b)
		b = filter.Exists(filterKey, []byte(k))
		assert.Equal(t, true, b)
		if !b {
			fmt.Print("not exists")
		}
	}
	fmt.Println("done", seed)
}

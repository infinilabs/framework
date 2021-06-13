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

package filter

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/boltdb"
	"os"
	"testing"
	"time"
)

const filterKey = "testFilter"

func Test(t *testing.T) {
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = "/tmp/filter_" + util.PickRandomName()
	os.RemoveAll(env1.SystemConfig.PathConfig.Data)
	env1.IsDebug = true
	global.RegisterEnv(env1)

	storage := boltdb.StorageModule{}
	storage.Start()

	m := FilterModule{}
	m.Start()
	b, _ := filter.CheckThenAdd(filterKey, []byte("key"))
	assert.Equal(t, false, b)

	//Memory pressure test
	for i := 0; i < 1; i++ {
		go run(i, t)
	}

	time.Sleep(10 * time.Second)

	//For BoltDB KV filter, 19k unique will consume 100MB memory, 40K:230MB
}

func run(seed int, t *testing.T) {
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

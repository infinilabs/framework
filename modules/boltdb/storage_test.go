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

package boltdb

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
	"os"
	"testing"
)

func Test(t *testing.T) {
	var storage StorageModule
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = "/tmp/filter_" + util.PickRandomName()
	os.RemoveAll(env1.SystemConfig.PathConfig.Data)
	env1.IsDebug = true
	global.RegisterEnv(env1)

	storage = StorageModule{}
	storage.Start()

	//Memory pressure test
	//for i := 0; i < 1; i++ {
	//	go run(i, t)
	//}
	//
	//time.Sleep(10 * time.Minute)
}

func run(seed int, t *testing.T) {
	KVBucketKey := "kv"
	for i := 0; i < 10000000; i++ {
		//fmt.Println(i)
		k := fmt.Sprintf("key-%v-%v", seed, i)
		v := []byte("A")
		b, _ := kv.GetValue(KVBucketKey, []byte(k))
		assert.Equal(t, false, b != nil)
		kv.AddValue(KVBucketKey, []byte(k), v)
		b, _ = kv.GetValue(KVBucketKey, []byte(k))
		assert.Equal(t, true, b != nil)
		if b == nil {
			fmt.Print("not exists")
		}
	}
	fmt.Println("done", seed)
}

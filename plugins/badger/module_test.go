/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
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

	time.Sleep(100 * time.Second)

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

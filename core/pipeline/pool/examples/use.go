package main

import (
	"fmt"
	"infini.sh/framework/core/pipeline/pool"
	"time"
)

func main() {
	pool1, err := pool.NewPool(10)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 20; i++ {
		pool1.Put(&pool.Task{
			Handler: func(v ...interface{}) {
				fmt.Println(v)
			},
			Params: []interface{}{i},
		})
	}

	time.Sleep(1e9)
}

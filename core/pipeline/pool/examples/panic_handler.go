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

	pool1.PanicHandler = func(r interface{}) {
		fmt.Printf("Warning!!! %s", r)
	}

	pool1.Put(&pool.Task{
		Handler: func(v ...interface{}) {
			panic("somthing wrong!")
		},
	})

	time.Sleep(1e9)
}

package main

import (
	"fmt"
	"infini.sh/framework/core/dag/pool"
	"sync"
)

func main() {
	// 创建容量为 10 的任务池
	pool1, err := pool.NewPool(10)
	if err != nil {
		panic(err)
	}

	wg := new(sync.WaitGroup)
	// 创建任务
	task := &pool.Task{
		Handler: func(v ...interface{}) {
			wg.Done()
			fmt.Println(v)
		},
	}

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		// 添加任务函数的参数
		task.Params = []interface{}{i, i * 2, "hello"}
		// 将任务放入任务池
		pool1.Put(task)
	}

	wg.Add(1)
	// 再创建一个任务
	pool1.Put(&pool.Task{
		Handler: func(v ...interface{}) {
			wg.Done()
			fmt.Println(v)
		},
		Params: []interface{}{"hi!"}, // 也可以在创建任务时设置参数
	})

	wg.Wait()

	// 安全关闭任务池（保证已加入池中的任务被消费完）
	pool1.Close()
	// 如果任务池已经关闭, Put() 方法会返回 ErrPoolAlreadyClosed 错误
	err = pool1.Put(&pool.Task{
		Handler: func(v ...interface{}) {},
	})
	if err != nil {
		fmt.Println(err) // print: pool already closed
	}
}

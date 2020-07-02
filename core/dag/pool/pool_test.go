/*
Copyright Medcl (m AT medcl.net)

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

package pool

import (
	"sync"
	"sync/atomic"
	"testing"
)

var sum int64
var runTimes = 1000000

var wg = sync.WaitGroup{}

func demoTask(v ...interface{}) {
	for i := 0; i < 100; i++ {
		atomic.AddInt64(&sum, 1)
	}
}

func demoTask2(v ...interface{}) {
	defer wg.Done()
	for i := 0; i < 100; i++ {
		atomic.AddInt64(&sum, 1)
	}
}

func BenchmarkGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		go demoTask()
	}
}

func BenchmarkPut(b *testing.B) {
	pool, err := NewPool(10)
	if err != nil {
		b.Error(err)
	}

	task := &Task{
		Handler: demoTask,
	}

	for i := 0; i < b.N; i++ {
		pool.Put(task)
	}
}

func BenchmarkGoroutineTimelife(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go demoTask2()
	}
	wg.Wait()
}

func BenchmarkPutTimelife(b *testing.B) {
	pool, err := NewPool(10)
	if err != nil {
		b.Error(err)
	}

	task := &Task{
		Handler: demoTask2,
	}

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		pool.Put(task)
	}
	wg.Wait()

}

func BenchmarkGoroutineSetTimes(b *testing.B) {

	for i := 0; i < runTimes; i++ {
		go demoTask()
	}
}

func BenchmarkPoolPutSetTimes(b *testing.B) {
	pool, err := NewPool(20)
	if err != nil {
		b.Error(err)
	}

	task := &Task{
		Handler: demoTask,
	}

	for i := 0; i < runTimes; i++ {
		pool.Put(task)
	}
}

func BenchmarkGoroutineTimeLifeSetTimes(b *testing.B) {

	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		go demoTask2()
	}
	wg.Wait()
}

func BenchmarkPoolTimeLifeSetTimes(b *testing.B) {
	pool, err := NewPool(20)
	if err != nil {
		b.Error(err)
	}

	task := &Task{
		Handler: demoTask2,
	}

	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		pool.Put(task)
	}

	wg.Wait()
}

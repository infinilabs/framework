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

// MIT License

// Copyright (c) 2018 Andy Pan

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pipeline

import (
	"log"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	_   = 1 << (10 * iota)
	KiB // 1024
	MiB // 1048576
	// GiB // 1073741824
	// TiB // 1099511627776             (超过了int32的范围)
	// PiB // 1125899906842624
	// EiB // 1152921504606846976
	// ZiB // 1180591620717411303424    (超过了int64的范围)
	// YiB // 1208925819614629174706176
)

const (
	Param    = 100
	AntsSize = 1000
	TestSize = 10000
	n        = 100000
)

var curMem uint64

// TestAntsPoolWithFuncWaitToGetWorker is used to test waiting to get worker.
func TestAntsPoolWithFuncWaitToGetWorker(t *testing.T) {
	var wg sync.WaitGroup
	p, _ := NewPoolWithFunc(AntsSize, func(i interface{}) {
		demoPoolFunc(i)
		wg.Done()
	})
	defer p.Release()

	for i := 0; i < n; i++ {
		wg.Add(1)
		_ = p.Invoke(Param)
	}
	wg.Wait()
	t.Logf("pool with func, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestAntsPoolWithFuncWaitToGetWorkerPreMalloc(t *testing.T) {
	var wg sync.WaitGroup
	p, _ := NewPoolWithFunc(AntsSize, func(i interface{}) {
		demoPoolFunc(i)
		wg.Done()
	}, WithPreAlloc(true))
	defer p.Release()

	for i := 0; i < n; i++ {
		wg.Add(1)
		_ = p.Invoke(Param)
	}
	wg.Wait()
	t.Logf("pool with func, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// TestAntsPoolGetWorkerFromCache is used to test getting worker from sync.Pool.
func TestAntsPoolGetWorkerFromCache(t *testing.T) {
	p, _ := NewPool(TestSize)
	defer p.Release()

	for i := 0; i < AntsSize; i++ {
		_ = p.Submit(demoFunc)
	}
	time.Sleep(2 * DefaultCleanIntervalTime)
	_ = p.Submit(demoFunc)
	t.Logf("pool, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// TestAntsPoolWithFuncGetWorkerFromCache is used to test getting worker from sync.Pool.
func TestAntsPoolWithFuncGetWorkerFromCache(t *testing.T) {
	dur := 10
	p, _ := NewPoolWithFunc(TestSize, demoPoolFunc)
	defer p.Release()

	for i := 0; i < AntsSize; i++ {
		_ = p.Invoke(dur)
	}
	time.Sleep(2 * DefaultCleanIntervalTime)
	_ = p.Invoke(dur)
	t.Logf("pool with func, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestAntsPoolWithFuncGetWorkerFromCachePreMalloc(t *testing.T) {
	dur := 10
	p, _ := NewPoolWithFunc(TestSize, demoPoolFunc, WithPreAlloc(true))
	defer p.Release()

	for i := 0; i < AntsSize; i++ {
		_ = p.Invoke(dur)
	}
	time.Sleep(2 * DefaultCleanIntervalTime)
	_ = p.Invoke(dur)
	t.Logf("pool with func, running workers number:%d", p.Running())
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

//-------------------------------------------------------------------------------------------
// Contrast between goroutines without a pool and goroutines with ants pool.
//-------------------------------------------------------------------------------------------

func TestNoPool(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			demo(nil, nil)
			wg.Done()
		}()
	}

	wg.Wait()
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestAntsPool(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}
	defer Release()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		_ = Submit(demoFunc)
	}
	wg.Wait()

	t.Logf("pool, capacity:%d", Cap())
	t.Logf("pool, running workers number:%d", Running())
	t.Logf("pool, free workers number:%d", Free())

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestNonblockingSubmitWithFunc(t *testing.T) {
	poolSize := 10
	var wg sync.WaitGroup
	p, err := NewPoolWithFunc(poolSize, func(i interface{}) {
		longRunningPoolFunc(i)
		wg.Done()
	}, WithNonblocking(true))
	assert.NoError(t, err, "create TimingPool failed: %v", err)
	defer p.Release()
	ch := make(chan struct{})
	wg.Add(poolSize)
	for i := 0; i < poolSize-1; i++ {
		assert.NoError(t, p.Invoke(ch), "nonblocking submit when pool is not full shouldn't return error")
	}
	// p is full now.
	assert.NoError(t, p.Invoke(ch), "nonblocking submit when pool is not full shouldn't return error")
	assert.EqualError(t, p.Invoke(nil), ErrPoolOverload.Error(),
		"nonblocking submit when pool is full should get an ErrPoolOverload")
	// interrupt f to get an available worker
	close(ch)
	wg.Wait()
	assert.NoError(t, p.Invoke(nil), "nonblocking submit when pool is not full shouldn't return error")
}

func TestInfinitePoolWithFunc(t *testing.T) {
	c := make(chan struct{})
	p, _ := NewPoolWithFunc(-1, func(i interface{}) {
		demoPoolFunc(i)
		<-c
	})
	_ = p.Invoke(10)
	_ = p.Invoke(10)
	c <- struct{}{}
	c <- struct{}{}
	if n := p.Running(); n != 2 {
		t.Errorf("expect 2 workers running, but got %d", n)
	}
	if n := p.Free(); n != -1 {
		t.Errorf("expect -1 of free workers by unlimited pool, but got %d", n)
	}
	p.Tune(10)
	if capacity := p.Cap(); capacity != -1 {
		t.Fatalf("expect capacity: -1 but got %d", capacity)
	}
	var err error
	_, err = NewPoolWithFunc(-1, demoPoolFunc, WithPreAlloc(true))
	if err != ErrInvalidPreAllocSize {
		t.Errorf("expect ErrInvalidPreAllocSize but got %v", err)
	}
}

func TestReleaseWhenRunningPoolWithFunc(t *testing.T) {
	var wg sync.WaitGroup
	p, _ := NewPoolWithFunc(1, func(i interface{}) {
		t.Log("do task", i)
		time.Sleep(1 * time.Second)
	})
	wg.Add(2)
	go func() {
		t.Log("start aaa")
		defer func() {
			wg.Done()
			t.Log("stop aaa")
		}()
		for i := 0; i < 30; i++ {
			_ = p.Invoke(i)
		}
	}()

	go func() {
		t.Log("start bbb")
		defer func() {
			wg.Done()
			t.Log("stop bbb")
		}()
		for i := 100; i < 130; i++ {
			_ = p.Invoke(i)
		}
	}()

	time.Sleep(3 * time.Second)
	p.Release()
	t.Log("wait for all goroutines to exit...")
	wg.Wait()
}

func TestRestCodeCoverage(t *testing.T) {
	_, err := NewPool(-1, WithExpiryDuration(-1))
	t.Log(err)
	_, err = NewPool(1, WithExpiryDuration(-1))
	t.Log(err)
	_, err = NewPoolWithFunc(-1, demoPoolFunc, WithExpiryDuration(-1))
	t.Log(err)
	_, err = NewPoolWithFunc(1, demoPoolFunc, WithExpiryDuration(-1))
	t.Log(err)

	options := Options{}
	options.ExpiryDuration = time.Duration(10) * time.Second
	options.Nonblocking = true
	options.PreAlloc = true
	poolOpts, _ := NewPool(1, WithOptions(options))
	t.Logf("Pool with options, capacity: %d", poolOpts.Cap())

	p0, _ := NewPool(TestSize, WithLogger(log.New(os.Stderr, "", log.LstdFlags)))
	defer func() {
		_ = p0.Submit(demoFunc)
	}()
	defer p0.Release()
	for i := 0; i < n; i++ {
		_ = p0.Submit(demoFunc)
	}
	t.Logf("pool, capacity:%d", p0.Cap())
	t.Logf("pool, running workers number:%d", p0.Running())
	t.Logf("pool, free workers number:%d", p0.Free())
	p0.Tune(TestSize)
	p0.Tune(TestSize / 10)
	t.Logf("pool, after tuning capacity, capacity:%d, running:%d", p0.Cap(), p0.Running())

	pprem, _ := NewPool(TestSize, WithPreAlloc(true))
	defer func() {
		_ = pprem.Submit(demoFunc)
	}()
	defer pprem.Release()
	for i := 0; i < n; i++ {
		_ = pprem.Submit(demoFunc)
	}
	t.Logf("pre-malloc pool, capacity:%d", pprem.Cap())
	t.Logf("pre-malloc pool, running workers number:%d", pprem.Running())
	t.Logf("pre-malloc pool, free workers number:%d", pprem.Free())
	pprem.Tune(TestSize)
	pprem.Tune(TestSize / 10)
	t.Logf("pre-malloc pool, after tuning capacity, capacity:%d, running:%d", pprem.Cap(), pprem.Running())

	p, _ := NewPoolWithFunc(TestSize, demoPoolFunc)
	defer func() {
		_ = p.Invoke(Param)
	}()
	defer p.Release()
	for i := 0; i < n; i++ {
		_ = p.Invoke(Param)
	}
	time.Sleep(DefaultCleanIntervalTime)
	t.Logf("pool with func, capacity:%d", p.Cap())
	t.Logf("pool with func, running workers number:%d", p.Running())
	t.Logf("pool with func, free workers number:%d", p.Free())
	p.Tune(TestSize)
	p.Tune(TestSize / 10)
	t.Logf("pool with func, after tuning capacity, capacity:%d, running:%d", p.Cap(), p.Running())

	ppremWithFunc, _ := NewPoolWithFunc(TestSize, demoPoolFunc, WithPreAlloc(true))
	defer func() {
		_ = ppremWithFunc.Invoke(Param)
	}()
	defer ppremWithFunc.Release()
	for i := 0; i < n; i++ {
		_ = ppremWithFunc.Invoke(Param)
	}
	time.Sleep(DefaultCleanIntervalTime)
	t.Logf("pre-malloc pool with func, capacity:%d", ppremWithFunc.Cap())
	t.Logf("pre-malloc pool with func, running workers number:%d", ppremWithFunc.Running())
	t.Logf("pre-malloc pool with func, free workers number:%d", ppremWithFunc.Free())
	ppremWithFunc.Tune(TestSize)
	ppremWithFunc.Tune(TestSize / 10)
	t.Logf("pre-malloc pool with func, after tuning capacity, capacity:%d, running:%d", ppremWithFunc.Cap(),
		ppremWithFunc.Running())
}

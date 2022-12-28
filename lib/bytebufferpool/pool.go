package bytebufferpool

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	minBitSize = 6 // 2**6=64 is a CPU cache line size
	steps      = 20

	minSize = 1 << minBitSize
	maxSize = 1 << (minBitSize + steps - 1)

	calibrateCallsThreshold = 42000
	maxPercentile           = 0.95
)

// Pool represents byte buffer pool.
//
// Distinct pools may be used for distinct types of byte buffers.
// Properly determined byte buffer types with their own pools may help reducing
// memory waste.
type Pool struct {
	Tag         string
	calls       [steps]uint32
	calibrating uint32

	defaultSize     uint32
	maxDataByteSize uint32

	maxItemCount uint32

	//stats
	allocate, acquire, returned, notReturn, throttle, cap uint32

	//stats
	inuse, poolByteSize int32

	pool         sync.Pool
	sequenceID   uint32
	throttleTime *time.Time
}

func NewTaggedPool(tag string, defaultSize, maxSize uint32, maxItems uint32) *Pool {
	pool := NewPool(defaultSize, maxSize)
	pool.Tag = tag
	pool.maxItemCount = maxItems
	pools.Store(tag, pool)
	return pool
}

func NewPool(defaultSize, maxSize uint32) *Pool {
	p := Pool{defaultSize: defaultSize, maxItemCount: 0, maxDataByteSize: maxSize}
	return &p
}

//var defaultPool Pool

var pools = sync.Map{}
var lock =sync.RWMutex{}
func getPoolByTag(tag string) (pool *Pool) {

	if x, ok := pools.Load(tag); ok {
		pool = x.(*Pool)
		if pool != nil {
			return pool
		}
	} else {
		lock.Lock()
		if x,ok:=pools.Load(tag);!ok{
			pool = NewTaggedPool(tag, 0, 100*1024*1024, 1000000)
		}else{
			pool = x.(*Pool)
		}
		lock.Unlock()
	}
	return pool
}

// Get returns an empty byte buffer from the pool.
//
// Got byte buffer may be returned to the pool via Put call.
// This reduces the number of memory allocations required for byte buffer
// management.
func Get(tag string) *ByteBuffer {
	return getPoolByTag(tag).Get()
}

// Get returns new byte buffer with zero length.
//
// The byte buffer may be returned to the pool via Put after the use
// in order to minimize GC overhead.
func (p *Pool) Get() *ByteBuffer {
	if p.maxItemCount > 0 && p.inuse > int32(p.maxItemCount) {
		time.Sleep(1 * time.Second)
		atomic.AddUint32(&p.throttle, 1)

		var t1 time.Time
		if p.throttleTime == nil {
			t1 = time.Now()
			p.throttleTime = &t1
		}

		if time.Since(t1) > 10*time.Second {
			panic(errors.New(fmt.Sprintf("running out of bytes buffer [%v][%v]",p.Tag,p.inuse)))
		}

		return p.Get()
	}

	if p.throttleTime != nil {
		p.throttleTime = nil
	}

	atomic.AddInt32(&p.inuse, 1)
	atomic.AddUint32(&p.acquire, 1)

	v := p.pool.Get()
	if v != nil {
		x := v.(*ByteBuffer)
		x.Reset()
		return x
	}

	id := atomic.AddUint32(&p.sequenceID, 1)
	x := &ByteBuffer{
		ID: id,
		B:  make([]byte, 0, atomic.LoadUint32(&p.defaultSize)),
	}

	atomic.AddUint32(&p.allocate, 1)
	return x
}

func SetMaxBufferCount(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxItemCount, size)
}

func SetMaxBufferSize(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxDataByteSize, size)
}

func BuffStats() map[string]interface{} {

	var result = map[string]interface{}{}
	var pool *Pool
	pools.Range(func(key, value any) bool {
		pool = value.(*Pool)
		if pool == nil {
			return false
		}

		item := map[string]interface{}{}
		item["allocated"] = pool.allocate
		item["acquired"] = pool.acquire
		item["returned"] = pool.returned
		item["dropped"] = pool.notReturn
		item["throttled"] = pool.throttle

		item["inuse"] = pool.inuse

		item["pool_size"] = pool.poolByteSize

		item["max_size"] = pool.maxDataByteSize
		item["max_count"] = pool.maxItemCount

		result[key.(string)] = item

		return true
	})

	return result
}

// Put returns byte buffer to the pool.
//
// ByteBuffer.B mustn't be touched after returning it to the pool.
// Otherwise data races will occur.
func Put(tag string, b *ByteBuffer) {
	b.Reset()
	getPoolByTag(tag).Put(b)
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) Put(b *ByteBuffer) {

	atomic.AddInt32(&p.inuse, -1)

	idx := index(len(b.B))

	if atomic.AddUint32(&p.calls[idx], 1) > calibrateCallsThreshold {
		p.calibrate()
	}

	if p.maxItemCount>0 && p.inuse>int32(p.maxItemCount){
		atomic.AddInt32(&p.poolByteSize, int32((b.Cap())*-1))
		atomic.AddUint32(&p.notReturn, 1)
		b=nil
		return
	}

	maxSize := int(atomic.LoadUint32(&p.maxDataByteSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		b.Reset()
		p.pool.Put(b)
		atomic.AddUint32(&p.returned, 1)
		atomic.AddInt32(&p.poolByteSize, int32(b.Cap()))
	} else {
		atomic.AddInt32(&p.poolByteSize, int32((b.Cap())*-1))
		atomic.AddUint32(&p.notReturn, 1)
		b = nil
	}
}

func (p *Pool) calibrate() {

	if !atomic.CompareAndSwapUint32(&p.calibrating, 0, 1) {
		return
	}

	a := make(callSizes, 0, steps)
	var callsSum uint32
	for i := uint32(0); i < steps; i++ {
		calls := atomic.SwapUint32(&p.calls[i], 0)
		callsSum += calls
		a = append(a, callSize{
			calls: calls,
			size:  minSize << i,
		})
	}
	sort.Sort(a)

	defaultSize := a[0].size
	maxSize := defaultSize

	maxSum := uint32(float32(callsSum) * maxPercentile)
	callsSum = 0
	for i := 0; i < steps; i++ {
		if callsSum > maxSum {
			break
		}
		callsSum += a[i].calls
		size := a[i].size
		if size > maxSize {
			maxSize = size
		}
	}

	atomic.StoreUint32(&p.defaultSize, defaultSize)
	atomic.StoreUint32(&p.maxDataByteSize, maxSize)

	atomic.StoreUint32(&p.calibrating, 0)
}

type callSize struct {
	calls uint32
	size  uint32
}

type callSizes []callSize

func (ci callSizes) Len() int {
	return len(ci)
}

func (ci callSizes) Less(i, j int) bool {
	return ci[i].calls > ci[j].calls
}

func (ci callSizes) Swap(i, j int) {
	ci[i], ci[j] = ci[j], ci[i]
}

func index(n int) int {
	n--
	n >>= minBitSize
	idx := 0
	for n > 0 {
		n >>= 1
		idx++
	}
	if idx >= steps {
		idx = steps - 1
	}
	return idx
}

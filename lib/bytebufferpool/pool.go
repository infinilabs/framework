package bytebufferpool

import (
	"infini.sh/framework/core/stats"
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
	calls       [steps]uint64
	calibrating uint64

	defaultSize     uint64
	maxDataByteSize uint64

	maxItemCount int64

	//stats
	allocate, acquire, returned, notReturn, throttle, cap uint64

	//stats
	inuse, poolByteSize int64

	pool        sync.Pool
	sequenceID  int64
	inUseBuffer sync.Map //map[int64]*ByteBuffer
}

func NewTaggedPool(tag string, defaultSize, maxSize uint64, maxItems int64) *Pool {
	pool := NewPool(defaultSize, maxSize)
	pool.Tag = tag
	pool.maxItemCount = maxItems
	pools.Store(tag, pool)
	return pool
}

func NewPool(defaultSize, maxSize uint64) *Pool {
	p := Pool{defaultSize: defaultSize, maxItemCount: 100, maxDataByteSize: maxSize}
	p.inUseBuffer = sync.Map{}
	return &p
}

//var defaultPool Pool

var pools = sync.Map{}

func getPoolByTag(tag string) (pool *Pool) {

	if x, ok := pools.Load(tag); ok {
		pool = x.(*Pool)
		if pool != nil {
			return pool
		}
	} else {
		pool = NewTaggedPool(tag, 0, 0, 100)
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
	if p.maxItemCount > 0 && p.inuse > p.maxItemCount {
		time.Sleep(1 * time.Second)
		atomic.AddUint64(&p.throttle, 1)
		return p.Get()
	}

	atomic.AddInt64(&p.inuse, 1)
	atomic.AddUint64(&p.acquire, 1)

	v := p.pool.Get()
	if v != nil {
		x := v.(*ByteBuffer)
		x.Reset()
		atomic.AddInt64(&p.poolByteSize, int64((x.Cap())*-1))

		p.inUseBuffer.Store(x.ID, x)
		return x
	}

	id := atomic.AddInt64(&p.sequenceID, 1)
	x := &ByteBuffer{
		ID: id,
		B:  make([]byte, 0, atomic.LoadUint64(&p.defaultSize)),
	}

	atomic.AddUint64(&p.allocate, 1)

	p.inUseBuffer.Store(x.ID, x)
	return x
}

func SetMaxBufferCount(tag string, size int64) {
	atomic.StoreInt64(&getPoolByTag(tag).maxItemCount, size)
}

func SetMaxBufferSize(tag string, size uint64) {
	atomic.StoreUint64(&getPoolByTag(tag).maxDataByteSize, size)
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

		var inuse = 0
		pool.inUseBuffer.Range(func(key, value any) bool {

			x, ok := value.(*ByteBuffer)
			if ok {
				inuse += x.Cap()
			}

			return true
		})

		item["inuse_size"] = inuse

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
	stats.Gauge("buffer.put", tag, int64(b.Cap()))
	b.Reset()
	getPoolByTag(tag).Put(b)
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) Put(b *ByteBuffer) {

	atomic.AddInt64(&p.inuse, -1)
	p.inUseBuffer.Delete(b.ID)

	idx := index(len(b.B))

	if atomic.AddUint64(&p.calls[idx], 1) > calibrateCallsThreshold {
		p.calibrate()
	}

	maxSize := int(atomic.LoadUint64(&p.maxDataByteSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		b.Reset()
		p.pool.Put(b)
		atomic.AddUint64(&p.returned, 1)
		atomic.AddInt64(&p.poolByteSize, int64(b.Cap()))
	} else {
		atomic.AddUint64(&p.notReturn, 1)
		b = nil
	}
}

func (p *Pool) calibrate() {

	if !atomic.CompareAndSwapUint64(&p.calibrating, 0, 1) {
		return
	}

	a := make(callSizes, 0, steps)
	var callsSum uint64
	for i := uint64(0); i < steps; i++ {
		calls := atomic.SwapUint64(&p.calls[i], 0)
		callsSum += calls
		a = append(a, callSize{
			calls: calls,
			size:  minSize << i,
		})
	}
	sort.Sort(a)

	defaultSize := a[0].size
	maxSize := defaultSize

	maxSum := uint64(float64(callsSum) * maxPercentile)
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

	atomic.StoreUint64(&p.defaultSize, defaultSize)
	atomic.StoreUint64(&p.maxDataByteSize, maxSize)

	atomic.StoreUint64(&p.calibrating, 0)
}

type callSize struct {
	calls uint64
	size  uint64
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

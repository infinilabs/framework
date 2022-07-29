package bytebufferpool

import (
	log "github.com/cihub/seelog"
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
	calls       [steps]uint64
	calibrating uint64

	defaultSize     uint64
	maxDataByteSize uint64

	maxItemCount uint64

	//stats
	allocate, get, put, throttle uint64

	pool sync.Pool
}

func NewPool(defaultSize, maxSize uint64) *Pool {
	p := Pool{defaultSize: defaultSize, maxDataByteSize: maxSize}
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
		pool = &Pool{}
		pools.Store(tag, pool)
	}

	return pool
}

// Get returns an empty byte buffer from the pool.
//
// Got byte buffer may be returned to the pool via Put call.
// This reduces the number of memory allocations required for byte buffer
// management.
func Get(tag string) *ByteBuffer { return getPoolByTag(tag).Get(tag) }

// Get returns new byte buffer with zero length.
//
// The byte buffer may be returned to the pool via Put after the use
// in order to minimize GC overhead.
func (p *Pool) Get(tag string) *ByteBuffer {

	atomic.AddUint64(&p.get, 1)

	v := p.pool.Get()
	if v != nil {
		x := v.(*ByteBuffer)
		x.Reset()
		return x
	}

	if p.maxItemCount > 0 && p.allocate > p.maxItemCount {
		time.Sleep(1 * time.Second)
		atomic.AddUint64(&p.throttle, 1)
		return p.Get(tag)
	}

	x := &ByteBuffer{
		B: make([]byte, 0, atomic.LoadUint64(&p.defaultSize)),
	}

	atomic.AddUint64(&p.allocate, 1)
	return x
}

func SetMaxBufferCount(tag string, size uint64) {
	atomic.StoreUint64(&getPoolByTag(tag).maxItemCount, size)
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
		item["allocate"] = pool.allocate
		item["get"] = pool.get
		item["put"] = pool.put
		item["throttle"] = pool.throttle

		result[key.(string)] = item

		//size := 0
		//count := 0
		//for _, v := range pool. {
		//	count++
		//	size += v.Cap()
		//}

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
	getPoolByTag(tag).Put(tag, b)
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) Put(tag string, b *ByteBuffer) {

	atomic.AddUint64(&p.put, 1)

	idx := index(len(b.B))

	if atomic.AddUint64(&p.calls[idx], 1) > calibrateCallsThreshold {
		p.calibrate()
	}

	maxSize := int(atomic.LoadUint64(&p.maxDataByteSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		b.Reset()
		p.pool.Put(b)
	}
}

func (p *Pool) calibrate() {

	log.Debug("call calibrate")

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

	log.Debug("max size:", maxSize)

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

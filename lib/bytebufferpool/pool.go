package bytebufferpool

import (
	"infini.sh/framework/core/global"
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
	calls       [steps]uint64
	calibrating uint64

	defaultSize uint64
	maxSize     uint64

	pool sync.Pool
}

func NewPool(defaultSize, maxSize uint64) *Pool {
	p := Pool{defaultSize: defaultSize, maxSize: maxSize}
	return &p
}

var defaultPool Pool

// Get returns an empty byte buffer from the pool.
//
// Got byte buffer may be returned to the pool via Put call.
// This reduces the number of memory allocations required for byte buffer
// management.
func Get(tag string) *ByteBuffer { return defaultPool.Get(tag) }

// Get returns new byte buffer with zero length.
//
// The byte buffer may be returned to the pool via Put after the use
// in order to minimize GC overhead.
func (p *Pool) Get(tag string) *ByteBuffer {

	atomic.AddInt64(&get, 1)

	if global.Env().IsDebug() {
		stats.Increment("buffer_"+tag, "get")
	}

	v := p.pool.Get()
	if v != nil {
		x := v.(*ByteBuffer)
		x.Reset()
		return x
	}

	if new > maxBufferCount {
		time.Sleep(1 * time.Second)
		atomic.AddInt64(&throttle, 1)
		if global.Env().IsDebug() {
			stats.Increment("buffer_"+tag, "throttle")
		}
		return p.Get(tag)
	}

	x := &ByteBuffer{
		B: make([]byte, 0, atomic.LoadUint64(&p.defaultSize)),
	}

	//add obj to stats
	if global.Env().IsDebug() {
		stats.Increment("buffer_"+tag, "new")
	}
	atomic.AddInt64(&new, 1)
	buffers = append(buffers, x)
	return x
}

var buffers = []*ByteBuffer{}
var new, get, put, throttle int64

var maxBufferCount int64 = 5000
var maxBufferSize int64

func SetMaxBufferCount(size int64) {
	atomic.StoreInt64(&maxBufferCount, size)
}
func SetMaxBufferSize(size int64) {
	atomic.StoreInt64(&maxBufferSize, size)
}

func BuffStats() (int64, int64, int64, int64, int, int) {
	size := 0
	count := 0
	for _, v := range buffers {
		count++
		size += v.Len()
	}
	return new, get, put, throttle, count, size
}

// Put returns byte buffer to the pool.
//
// ByteBuffer.B mustn't be touched after returning it to the pool.
// Otherwise data races will occur.
func Put(tag string, b *ByteBuffer) {
	b.Reset()
	defaultPool.Put(tag, b)
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) Put(tag string, b *ByteBuffer) {

	if global.Env().IsDebug() {
		stats.Increment("buffer_"+tag, "put")
	}

	atomic.AddInt64(&put, 1)

	idx := index(len(b.B))

	if atomic.AddUint64(&p.calls[idx], 1) > calibrateCallsThreshold {
		p.calibrate()
	}

	maxSize := int(atomic.LoadUint64(&p.maxSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		b.Reset()
		p.pool.Put(b)
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
	atomic.StoreUint64(&p.maxSize, maxSize)

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

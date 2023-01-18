package bytebufferpool

import (
	"fmt"
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

	defaultMaxPoolSize = 100*1024*1024
	defaultMaxItemCount = 1000000
	calibrateCallsThreshold = 42000
	maxPercentile           = 0.95
)

type ObjectPool struct {
	tag                                                 string
	maxBytes int
	pool                                                chan interface{}
	New                                                 func()interface{}
	returnCallback                                      func()interface{}
	allocate, acquire, returned, dropped uint32
}
var objectPool =sync.Map{}
func NewObjectPool(tag string,allocateFunc func()interface{},returnCallback func()interface{},maxItems,maxBytes int) *ObjectPool {
	p:=&ObjectPool{tag: tag}
	if maxItems<=0{
		maxItems=defaultMaxItemCount
	}
	if maxBytes<=0{
		maxBytes=defaultMaxPoolSize
	}
	p.pool= make(chan interface{}, maxItems)
	p.maxBytes=maxBytes
	if allocateFunc==nil{
		panic("allocate function must be set")
	}
	p.New =allocateFunc
	p.returnCallback=returnCallback
	objectPool.Store(tag,p)
	return p
}

func (p *ObjectPool) Get()interface{}{
	var x interface{}
	select {
	case x = <-p.pool: // Try to get one from the pool
		atomic.AddUint32(&p.acquire, 1)
	default: // All in use, create a new, temporary:
		x = p.New()
		atomic.AddUint32(&p.allocate, 1)
	}
	return x
}

func (p *ObjectPool) Put(o interface{}){
	select {
	case p.pool <- o: // Try to put back into the pool
		atomic.AddUint32(&p.returned, 1)
	default: // Pool is full, will be garbage collected
		atomic.AddUint32(&p.dropped, 1)
	}
	if p.returnCallback!=nil{
		p.returnCallback()
	}
}


// Pool represents byte buffer pool.
//
// Distinct pools may be used for distinct types of byte buffers.
// Properly determined byte buffer types with their own pools may help reducing
// memory waste.
type Pool struct {
	Tag         string
	calls       [steps]uint32

	//https://www.jianshu.com/p/a730d095ae51
	//https://segmentfault.com/a/1190000039969499
	enableCalibrate bool
	calibrating uint32

	defaultSize     uint32
	maxItemSize      uint32

	maxDataByteSize uint32

	maxItemCount uint32

	//stats
	allocate, acquire, returned, dropped, throttle, cap uint32

	//stats
	invalid,inuse, poolByteSize,poolItems int32

	items sync.Map

	pool  chan *ByteBuffer
	//pool         sync.Pool

	sequenceID   uint32

	throttleTime *time.Time
}

func NewTaggedPool(tag string, defaultSize, maxSize uint32, maxItems uint32) *Pool {
	pool := newPool(defaultSize, maxSize,maxItems)
	pool.Tag = tag
	pools.Store(tag, pool)
	return pool
}

func (p *Pool)newBuffer() *ByteBuffer {
	id := atomic.AddUint32(&p.sequenceID, 1)
	x := &ByteBuffer{
		ID: id,
		B:  make([]byte, 0, atomic.LoadUint32(&p.defaultSize)),
		LastAccess: time.Now(),
	}

	atomic.AddUint32(&p.allocate, 1)
	atomic.AddInt32(&p.poolItems,1)
	p.items.Store(x.ID,x)
	return x
}

func newPool(defaultSize, maxSize,maxItems uint32) *Pool {

	if maxSize<=0{
		maxSize=defaultMaxPoolSize
	}

	if maxItems<=0{
		maxItems=defaultMaxItemCount
	}

	p := Pool{defaultSize: defaultSize, maxItemCount: maxItems, maxDataByteSize: maxSize}
	//p.pool = sync.Pool{
	//	//New: p.newBuffer,
	//}

	p.pool= make(chan *ByteBuffer, maxItems)

	p.items=sync.Map{}

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
	//if p.maxItemCount > 0 && p.inuse > int32(p.maxItemCount) {
	//	time.Sleep(1 * time.Second)
	//	atomic.AddUint32(&p.throttle, 1)
	//
	//	var t1 time.Time
	//	if p.throttleTime == nil {
	//		t1 = time.Now()
	//		p.throttleTime = &t1
	//	}
	//
	//	if time.Since(t1) > 10*time.Second {
	//		panic(errors.New(fmt.Sprintf("running out of bytes buffer [%v][%v]",p.Tag,p.inuse)))
	//	}
	//
	//	return p.Get()
	//}
	//
	//if p.throttleTime != nil {
	//	p.throttleTime = nil
	//}

	atomic.AddInt32(&p.inuse, 1)
	atomic.AddUint32(&p.acquire, 1)

	var x *ByteBuffer
	select {
	case x = <-p.pool: // Try to get one from the pool
	default: // All in use, create a new, temporary:
		x = p.newBuffer()
	}

	return x
}

func SetMaxBufferCount(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxItemCount, size)
}

func SetMaxBufferSize(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxDataByteSize, size)
}

func addIfNotZero(m map[string]interface{},k string,v uint32)  {
	if v>0{
		m[k] = v
	}
}

func BuffStats() map[string]interface{} {

	var result = map[string]interface{}{}
	var pool *Pool
	bytesBuffer:=map[string]interface{}{}
	pools.Range(func(key, value any) bool {
		pool = value.(*Pool)
		if pool == nil {
			return false
		}

		item := map[string]interface{}{}
		addIfNotZero(item,"allocated",pool.allocate)
		addIfNotZero(item,"acquired",pool.acquire)
		addIfNotZero(item,"returned",pool.returned)
		addIfNotZero(item,"dropped",pool.dropped)
		addIfNotZero(item,"throttled",pool.throttle)
		addIfNotZero(item,"inuse", uint32(pool.inuse))
		addIfNotZero(item,"invalid", uint32(pool.invalid))
		addIfNotZero(item,"pool_size", uint32(pool.getPoolByteSize()))
		addIfNotZero(item,"pool_items", uint32(pool.poolItems))
		//item["max_size"] = pool.maxDataByteSize
		//item["max_count"] = pool.maxItemCount
		if len(item)>0{
			bytesBuffer[key.(string)] = item
		}
		return true
	})

	if len(bytesBuffer)>0{
		result["bytes"]=bytesBuffer
	}

	var pool1 *ObjectPool
	objectBuffer:=map[string]interface{}{}
	objectPool.Range(func(key, value any) bool {
		pool1 = value.(*ObjectPool)
		if pool1 == nil {
			return false
		}
		item := map[string]interface{}{}
		addIfNotZero(item,"allocated",pool1.allocate)
		addIfNotZero(item,"acquired",pool1.acquire)
		addIfNotZero(item,"returned",pool1.returned)
		addIfNotZero(item,"dropped",pool1.dropped)
		if len(item)>0{
			objectBuffer[key.(string)] = item
		}
		return true
	})

	if len(objectBuffer)>0{
		result["objects"]=objectBuffer
	}

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

func PutWithoutReset(tag string, b *ByteBuffer) {
	getPoolByTag(tag).Put(b)
}

func (p *Pool)Drop(b *ByteBuffer){
	atomic.AddUint32(&p.dropped, 1)
	atomic.AddInt32(&p.poolItems,-1)
	p.items.Delete(b.ID)
	b=nil
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) Put(b *ByteBuffer) {

	atomic.AddInt32(&p.inuse, -1)

	if p.maxItemCount>0 && p.inuse>int32(p.maxItemCount){
		//log.Warnf("%v hit max items, dropping, %v>%v",p.Tag,p.inuse,p.maxItemCount)
		p.Drop(b)
		return
	}

	if p.maxDataByteSize>0 && p.poolByteSize>int32(p.maxDataByteSize){
		//log.Warnf("%v hit max data size, dropping, %v>%v",p.Tag,p.poolByteSize,p.maxDataByteSize)
		p.Drop(b)
		return
	}

	if p.enableCalibrate{
		idx := index(len(b.B))
		if atomic.AddUint32(&p.calls[idx], 1) > calibrateCallsThreshold {
			p.calibrate()
		}
	}

	maxSize := int(atomic.LoadUint32(&p.maxItemSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		b.Reset()
		select {
			case p.pool <- b: // Try to put back into the pool
				atomic.AddUint32(&p.returned, 1)
			default: // Pool is full, will be garbage collected
				p.Drop(b)
		}
	} else {
		if cap(b.B)>maxSize{
			log.Warnf("buffer %v too large (%v>%v), skip return",p.Tag,cap(b.B),maxSize)
		}
		p.Drop(b)
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
			log.Warnf("%v update max_size %v",p.Tag,maxSize)
		}
	}

	atomic.StoreUint32(&p.maxItemSize, maxSize)

	atomic.StoreUint32(&p.calibrating, 0)
}

func DumpBufferItemSize()map[string]interface{} {
	obj:=map[string]interface{}{}
	pools.Range(func(key, value any) bool {
		pool:= value.(*Pool)
		v:=pool.getPoolItemSize()
		obj[pool.Tag]=v
		return true
	})
	return obj
}

func (p *Pool) getPoolByteSize() int {
	total:=0
	p.items.Range(func(key, value any) bool {
		total+= value.(*ByteBuffer).Cap()
		return true
	})

	atomic.StoreInt32(&p.poolByteSize, int32(total))
	return total
}

func (p *Pool) getPoolItemSize() map[string]int {
	obj:=map[string]int{}
	p.items.Range(func(key, value any) bool {
		obj[fmt.Sprintf("%v", key)]= value.(*ByteBuffer).Cap()
		return true
	})
	obj["total"]=p.getPoolByteSize()
	return obj
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

var lastCleanup time.Time
func CleanupIdleCachedBytesBuffer() {
	if time.Since(lastCleanup)>1*time.Minute{
		lastCleanup=time.Now()
		pools.Range(func(key, value any) bool {
			v:=value.(*Pool)
			if v!=nil{
				limit:=int(v.poolItems)
				for i:=0;i<=limit;i++{
					x:=v.Get()
					if x!=nil{
						if time.Since(x.LastAccess)>1*time.Minute{
							log.Trace("cleanup old buffer item:",v.Tag,",id:",x.ID,",length:",x.Len(),",cap:",x.Cap())
							v.Drop(x)
						}else{
							v.Put(x)
						}
					}
				}
			}
			return true
		})
	}
}
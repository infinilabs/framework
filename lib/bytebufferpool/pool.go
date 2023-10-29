package bytebufferpool

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
)

const (
	minBitSize = 6 // 2**6=64 is a CPU cache line size
	steps      = 20

	minSize = 1 << minBitSize
	maxSize = 1 << (minBitSize + steps - 1)

	defaultMaxPoolSize      = 100 * 1024 * 1024
	defaultItemSize         = 0
	defaultMaxItemCount     = 100000
	calibrateThreshold      = 100
	maxPercentile           = 0.95
	defaultSizeInPercentile = 0.5
)

type ObjectPool struct {
	tag                                  string
	maxBytes                             int
	pool                                 chan interface{}
	New                                  func() interface{}
	returnCallback                       func() interface{}
	allocate, acquire, returned, dropped uint32
}

var objectPool = sync.Map{}

func NewObjectPool(tag string, allocateFunc func() interface{}, returnCallback func() interface{}, maxItems, maxBytes int) *ObjectPool {
	p := &ObjectPool{tag: tag}
	if maxItems <= 0 {
		maxItems = defaultMaxItemCount
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxPoolSize
	}
	p.pool = make(chan interface{}, maxItems)
	p.maxBytes = maxBytes
	if allocateFunc == nil {
		panic("allocate function must be set")
	}
	p.New = allocateFunc
	p.returnCallback = returnCallback
	objectPool.Store(tag, p)
	return p
}

func (p *ObjectPool) Get() interface{} {
	var x interface{}
	select {
	case x = <-p.pool: // Try to get one from the pool
	default: // All in use, create a new, temporary:
		x = p.New()
		atomic.AddUint32(&p.allocate, 1)
	}
	atomic.AddUint32(&p.acquire, 1)
	return x
}

func (p *ObjectPool) Put(o interface{}) {
	select {
	case p.pool <- o: // Try to put back into the pool
		atomic.AddUint32(&p.returned, 1)
	default: // Pool is full, will be garbage collected
		atomic.AddUint32(&p.dropped, 1)
	}
	if p.returnCallback != nil {
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
	calibrating uint32

	defaultItemSize uint32
	maxItemSize     uint32

	maxTotalDataByteSize uint32
	maxTotalItemCount    uint32

	//stats
	allocate, acquire, returned, dropped, throttle, cap, invalid uint32

	//stats
	inuse, poolByteSize, poolItems int32

	items *ItemMap

	pool chan *ByteBuffer

	sequenceID uint32

	throttleTime *time.Time
}

type ItemMap struct {
	sync.RWMutex
	data map[uint32]*ByteBuffer
}

func NewItemMap() *ItemMap {
	i := ItemMap{}
	i.data = map[uint32]*ByteBuffer{}
	return &i
}

func (this *ItemMap) Store(id uint32, buffer *ByteBuffer) {
	this.Lock()
	defer this.Unlock()

	this.data[id] = buffer
}

func (this *ItemMap) Delete(id uint32) {
	this.Lock()
	defer this.Unlock()

	delete(this.data, id)
}

func (this *ItemMap) Range(f func(key uint32, value *ByteBuffer) bool) {
	this.RLock()
	defer this.RUnlock()

	for k, v := range this.data {
		if !f(k, v) {
			break
		}
	}
}

func NewTaggedPool(tag string, defaultSize, maxSize uint32, maxItems uint32) *Pool {
	pool := newPool(defaultSize, maxSize, maxItems)
	pool.Tag = tag
	pools.Store(tag, pool)
	return pool
}

func (p *Pool) newBuffer() *ByteBuffer {
	id := atomic.AddUint32(&p.sequenceID, 1)
	x := &ByteBuffer{
		ID:         id,
		Used:       0,
		B:          make([]byte, 0, atomic.LoadUint32(&p.defaultItemSize)),
		LastAccess: time.Now(),
	}
	atomic.AddUint32(&p.allocate, 1)
	atomic.AddInt32(&p.poolItems, 1)
	p.items.Store(x.ID, x)
	return x
}

func newPool(defaultSize, maxSize, maxItems uint32) *Pool {
	if maxSize <= 0 {
		maxSize = defaultMaxPoolSize
	}

	if maxItems <= 0 {
		maxItems = defaultMaxItemCount
	}

	if defaultSize < 0 {
		defaultSize = defaultItemSize
	}

	p := Pool{defaultItemSize: defaultSize, maxTotalItemCount: maxItems, maxTotalDataByteSize: maxSize}
	p.pool = make(chan *ByteBuffer, maxItems)
	p.items = NewItemMap()
	return &p
}

var pools = sync.Map{}
var lock = sync.RWMutex{}

func getPoolByTag(tag string) (pool *Pool) {

	if x, ok := pools.Load(tag); ok {
		pool = x.(*Pool)
		if pool != nil {
			return pool
		}
	} else {
		lock.Lock()
		if x, ok := pools.Load(tag); !ok {
			pool = NewTaggedPool(tag, 0, 1024*1024*1024, 100000)
		} else {
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
	//if p.maxTotalItemCount > 0 && p.inuse > int32(p.maxTotalItemCount) {
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

	var x *ByteBuffer
	select {
	case x = <-p.pool: // Try to get one from the pool
	default: // All in use, create a new, temporary:
		x = p.newBuffer()
	}
	x.Used++
	atomic.AddInt32(&p.inuse, 1)
	atomic.AddUint32(&p.acquire, 1)
	x.Reset() //reset on get only
	return x
}

func (p *Pool) getOnly() *ByteBuffer {
	var x *ByteBuffer
	select {
	case x = <-p.pool: // Try to get one from the pool
	default: // All in use, create a new, temporary:
		return nil
	}
	return x
}

func SetMaxBufferCount(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxTotalItemCount, size)
}

func SetMaxBufferSize(tag string, size uint32) {
	atomic.StoreUint32(&getPoolByTag(tag).maxTotalDataByteSize, size)
}

func addIfNotZero(m map[string]interface{}, k string, v uint32) {
	if v > 0 {
		m[k] = v
	}
}

func BuffStats() map[string]interface{} {

	var result = map[string]interface{}{}
	var pool *Pool
	bytesBuffer := map[string]interface{}{}

	allItem := map[string]interface{}{}

	pools.Range(func(key, value any) bool {
		pool = value.(*Pool)
		if pool == nil {
			return false
		}

		item := map[string]interface{}{}
		addIfNotZero(item, "allocated", pool.allocate)
		addIfNotZero(item, "acquired", pool.acquire)
		addIfNotZero(item, "returned", pool.returned)
		addIfNotZero(item, "dropped", pool.dropped)
		addIfNotZero(item, "throttled", pool.throttle)
		addIfNotZero(item, "inuse", uint32(pool.inuse))
		addIfNotZero(item, "invalid", uint32(pool.invalid))

		poolSize:=uint32(pool.getPoolByteSize())
		addIfNotZero(item, "pool_size", poolSize)
		addIfNotZero(allItem, "pool_size", poolSize)

		poolItems:=uint32(atomic.LoadInt32(&pool.poolItems))
		addIfNotZero(item, "pool_items", poolItems)
		addIfNotZero(allItem, "pool_items", poolItems)

		if len(item) > 0 {
			bytesBuffer[key.(string)] = item
		}
		return true
	})

	bytesBuffer["_all"]=allItem

	if len(bytesBuffer) > 0 {
		result["bytes"] = bytesBuffer
	}

	var pool1 *ObjectPool
	objectBuffer := map[string]interface{}{}
	objectPool.Range(func(key, value any) bool {
		pool1 = value.(*ObjectPool)
		if pool1 == nil {
			return false
		}
		item := map[string]interface{}{}
		addIfNotZero(item, "allocated", pool1.allocate)
		addIfNotZero(item, "acquired", pool1.acquire)
		addIfNotZero(item, "returned", pool1.returned)
		addIfNotZero(item, "dropped", pool1.dropped)
		if len(item) > 0 {
			objectBuffer[key.(string)] = item
		}
		return true
	})

	if len(objectBuffer) > 0 {
		result["objects"] = objectBuffer
	}

	return result
}

// Put returns byte buffer to the pool.
//
// ByteBuffer.B mustn't be touched after returning it to the pool.
// Otherwise data races will occur.
func Put(tag string, b *ByteBuffer) {
	//b.Reset()
	getPoolByTag(tag).Put(b)
}

func PutWithoutReset(tag string, b *ByteBuffer) {
	getPoolByTag(tag).Put(b)
}

func (p *Pool) Drop(b *ByteBuffer) {
	atomic.AddUint32(&p.dropped, 1)
	atomic.AddInt32(&p.poolItems, -1)
	p.items.Delete(b.ID)
	b.B = nil
	b = nil
}

// Put releases byte buffer obtained via Get to the pool.
//
// The buffer mustn't be accessed after returning to the pool.
func (p *Pool) putOnly(b *ByteBuffer) {
	select {
	case p.pool <- b: // Try to put back into the pool
	default: // Pool is full, will be garbage collected
		p.Drop(b)
	}
}

func (p *Pool) Put(b *ByteBuffer) {

	atomic.AddInt32(&p.inuse, -1)
	length := b.Len()
	if length > 1024 && (length*10) < b.Cap() || (p.maxItemSize > 0 && (int(p.maxItemSize*10)) < b.Cap()) {
		if (p.defaultItemSize > 0 && uint32(length) > p.defaultItemSize) || p.maxItemSize > 0 && (uint32(length) > p.maxItemSize) {
			atomic.AddUint32(&p.invalid, 1)
			log.Debugf("pool:%v, buffer is far too large, dropping, default:%v max:%v, len:%v ~< cap:%v", p.Tag, p.defaultItemSize, p.maxItemSize, length, b.Cap())
			p.Drop(b)
			return
		}
	}

	if p.maxTotalItemCount > 0 && p.inuse > int32(p.maxTotalItemCount) {
		log.Tracef("%v hit max items, dropping, %v>%v", p.Tag, p.inuse, p.maxTotalItemCount)
		p.Drop(b)
		return
	}

	if p.maxTotalDataByteSize > 0 && p.poolByteSize > int32(p.maxTotalDataByteSize) {
		log.Tracef("%v hit max data size, dropping, %v>%v", p.Tag, p.poolByteSize, p.maxTotalDataByteSize)
		p.Drop(b)
		return
	}

	maxSize := int(atomic.LoadUint32(&p.maxItemSize))
	if maxSize == 0 || cap(b.B) <= maxSize {
		//b.Reset()
		select {
		case p.pool <- b: // Try to put back into the pool
			atomic.AddUint32(&p.returned, 1)
		default: // Pool is full, will be garbage collected
			p.Drop(b)
		}
	} else {
		//if cap(b.B)>maxSize{
		//	log.Warnf("buffer %v too large (%v>%v), skip return",p.Tag,cap(b.B),maxSize)
		//}
		p.Drop(b)
	}
}

func (p *Pool) calibrate() {

	if !atomic.CompareAndSwapUint32(&p.calibrating, 0, 1) {
		return
	}

	limit := atomic.LoadInt32(&p.poolItems)
	if limit > 10 {
		var list []int
		p.items.Range(func(key uint32, value *ByteBuffer) bool {
			if value.Len() > 0 {
				list = append(list, value.Len()) //prefer to use length over cap
			} else {
				list = append(list, value.Cap())
			}
			return true
		})

		if len(list) > 0 {
			sort.Ints(list)
			maxSize := 0
			maxSum := int(float32(len(list)) * maxPercentile)

			if maxSum > 0 && maxSum <= (len(list)-1) {
				maxSize = list[maxSum]
				if p.maxItemSize != uint32(maxSize) {
					log.Debugf("%v update max item size from:%v to:%v", p.Tag, p.maxItemSize, maxSize)
					atomic.StoreUint32(&p.maxItemSize, uint32(maxSize))
				}
			}

			defaultSizeIndex := int(float32(len(list)) * defaultSizeInPercentile)
			if defaultSizeIndex > 0 && defaultSizeIndex <= (len(list)-1) {
				defaultSize := uint32(list[defaultSizeIndex])
				if p.defaultItemSize != defaultSize && defaultSize < p.maxItemSize {
					log.Debugf("%v update default item size from:%v to:%v", p.Tag, p.defaultItemSize, defaultSize)
					atomic.StoreUint32(&p.defaultItemSize, defaultSize)
				}
			}
		}
	}

	atomic.StoreUint32(&p.calibrating, 0)
}

func DumpBufferItemSize() map[string]interface{} {
	obj := map[string]interface{}{}
	pools.Range(func(key, value any) bool {
		pool := value.(*Pool)
		v := pool.getPoolItemSize()
		obj[pool.Tag] = v
		return true
	})
	return obj
}

func (p *Pool) getPoolByteSize() int {
	total := 0
	p.items.Range(func(key uint32, value *ByteBuffer) bool {
		total += value.Cap()
		return true
	})

	atomic.StoreInt32(&p.poolByteSize, int32(total))
	return total
}

func (p *Pool) getPoolItemSize() map[string]interface{} {
	obj := map[string]interface{}{}
	items := map[string]string{}
	p.items.Range(func(key uint32, value *ByteBuffer) bool {
		items[fmt.Sprintf("%v-%v-%v", key, value.Used, int(time.Since(value.LastAccess).Seconds()))] = fmt.Sprintf("%v,%v", value.Len(), value.Cap())
		return true
	})

	addIfNotZero(obj, "allocated", p.allocate)
	addIfNotZero(obj, "acquired", p.acquire)
	addIfNotZero(obj, "returned", p.returned)
	addIfNotZero(obj, "dropped", p.dropped)
	addIfNotZero(obj, "throttled", p.throttle)
	addIfNotZero(obj, "inuse", uint32(p.inuse))
	addIfNotZero(obj, "invalid", uint32(p.invalid))
	addIfNotZero(obj, "pool_size", uint32(p.getPoolByteSize()))
	addIfNotZero(obj, "pool_items", uint32(atomic.LoadInt32(&p.poolItems)))

	addIfNotZero(obj, "max_item_size", uint32(p.maxItemSize))
	addIfNotZero(obj, "default_item_size", uint32(p.defaultItemSize))

	addIfNotZero(obj, "max_count", uint32(p.maxTotalItemCount))
	addIfNotZero(obj, "max_size", uint32(p.maxTotalDataByteSize))

	obj["items"] = items

	return obj
}

var lastCleanup time.Time

func CleanupIdleCachedBytesBuffer() {
	if time.Since(lastCleanup) > 1*time.Minute {
		lastCleanup = time.Now()
		pools.Range(func(key, value any) bool {
			defer log.Tracef("end cleanup:%v", key)
			v := value.(*Pool)
			log.Tracef("start cleanup:%v, %v", key, v != nil)
			if v != nil {
				limit := int(atomic.LoadInt32(&v.poolItems))
				log.Tracef("cleanup:%v, items:%v", key, limit)
				if limit > 0 {
					for i := 0; i <= limit; i++ {
						x := v.getOnly()
						if x != nil {
							if time.Since(x.LastAccess) > 1*time.Minute {
								log.Trace("dropping old buffer item:", v.Tag, ",id:", x.ID, ",length:", x.Len(), ",cap:", x.Cap())
								v.Drop(x)
							} else {
								v.putOnly(x)
							}
						}
					}
				}

				if limit > calibrateThreshold {
					v.calibrate()
				}

				if limit < 10 || atomic.CompareAndSwapUint32(&v.invalid, 1000, 0) {
					if v.defaultItemSize > 0 {
						log.Debugf("tag: %v, reset default item size", v.Tag)
						atomic.StoreUint32(&v.defaultItemSize, 0)
					}
					if v.maxItemSize > 0 {
						log.Debugf("tag: %v, reset max item size", v.Tag)
						atomic.StoreUint32(&v.maxItemSize, 0)
					}
				}
			}
			return true
		})
	}
}

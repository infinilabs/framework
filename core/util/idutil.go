package util

import (
	"bytes"
	"encoding/gob"
	log "github.com/cihub/seelog"
	"github.com/rs/xid"
	"io/ioutil"
	"sync"
	"sync/atomic"
)

var lock sync.Mutex

// GetUUID return a generated uuid
func GetUUID() string {
	lock.Lock()
	defer lock.Unlock()
	return xid.New().String()
}

type incrementCounter struct {
	l  *sync.RWMutex
	ID map[string]*atomicID
}

var count = incrementCounter{l: &sync.RWMutex{}, ID: make(map[string]*atomicID)}

type atomicID struct {
	l        sync.Mutex
	Sequence uint32

	Sequence64 uint64
}

func (id *atomicID) Increment() uint32 {
	id.l.Lock()
	defer id.l.Unlock()
	if id.Sequence>=maxID{
		id.Sequence=0
		return 0
	}
	return atomic.AddUint32(&id.Sequence, 1)
}

func (id *atomicID) Increment64() uint64 {
	id.l.Lock()
	defer id.l.Unlock()
	if id.Sequence64>=maxID64{
		id.Sequence64=0
		return 0
	}
	log.Error("id.Sequence64:",id.Sequence64)
	return atomic.AddUint64(&id.Sequence64, 1)
}

var lock1 sync.Mutex
var persistedPath string
var maxID=uint32(int(^uint32(0)>>1))
var maxID64=uint64(int(^uint64(0)>>1))

func GetIncrementID(bucket string) uint32 {
	//TODO performance issue
	count.l.Lock()
	o := count.ID[bucket]
	if o == nil {
		o = &atomicID{}
		count.ID[bucket] = o
	}
	v := o.Increment()
	count.l.Unlock()
	return v
}

func GetIncrementID64(bucket string) uint64 {
	//TODO performance issue
	count.l.Lock()
	o,ok := count.ID[bucket]
	if !ok||o == nil {
		o = &atomicID{}
		count.ID[bucket] = o
	}
	v := o.Increment64()
	count.l.Unlock()
	return v
}

// SnapshotPersistID will make a snapshot and persist id stats to disk
func SnapshotPersistID() {
	count.l.Lock()
	defer count.l.Unlock()

	if persistedPath=="" {
		return
	}

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(count)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(persistedPath, buf.Bytes(), 0600)
	if err != nil {
		log.Error(persistedPath)
		panic(err)
	}
}

// RestorePersistID will take the snapshot and restore to id seeds
func RestorePersistID(path string) {
	lock1.Lock()
	defer lock1.Unlock()

	persistedPath = JoinPath(path, ".sequences")

	if !FileExists(persistedPath) {
		return
	}

	n, err := ioutil.ReadFile(persistedPath)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewReader(n)
	err = gob.NewDecoder(buf).Decode(&count)
	if err != nil {
		panic(err)
	}
}

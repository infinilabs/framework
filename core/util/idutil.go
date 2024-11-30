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
	ID map[string]*AtomicID
}

var count = incrementCounter{l: &sync.RWMutex{}, ID: make(map[string]*AtomicID)}

type AtomicID struct {
	l        sync.Mutex
	Sequence uint32
	MaxSequence uint32

	Sequence64 uint64
	MaxSequence64 uint64
}

func (id *AtomicID) Increment() uint32 {
	id.l.Lock()
	defer id.l.Unlock()

	if id.MaxSequence>0{
		if id.Sequence>=id.MaxSequence{
			id.Sequence=0
			return 0
		}
	}else{
		if id.Sequence>=maxID{
			id.Sequence=0
			return 0
		}
	}
	return atomic.AddUint32(&id.Sequence, 1)
}

func (id *AtomicID) Increment64() uint64 {
	id.l.Lock()
	defer id.l.Unlock()

	if id.MaxSequence64>0{
		if id.Sequence64>=id.MaxSequence64{
			id.Sequence64=0
			return 0
		}
	}else{
		if id.Sequence64>=maxID64{
			id.Sequence64=0
			return 0
		}
	}
	return atomic.AddUint64(&id.Sequence64, 1)
}

var lock1 sync.Mutex
var persistedPath string

var maxID=uint32(^uint32(0)>>1)
var maxID64=uint64(^uint64(0)>>1)

func GetAutoIncrement32ID(bucket string,rangeFrom,rangeTo uint32) *AtomicID {
	count.l.Lock()
	o := count.ID[bucket]
	if o == nil {
		o = &AtomicID{}
		o.Sequence=rangeFrom
		o.MaxSequence=rangeTo
		count.ID[bucket] = o
	}
	count.l.Unlock()
	return o
}

func GetAutoIncrement64ID(bucket string,rangeFrom,rangeTo uint64) *AtomicID {
	count.l.Lock()
	o := count.ID[bucket]
	if o == nil {
		o = &AtomicID{}
		o.Sequence64=rangeFrom
		o.MaxSequence64=rangeTo
		count.ID[bucket] = o
	}
	count.l.Unlock()
	return o
}

func GetIncrementID(bucket string) uint32 {
	o:=GetAutoIncrement32ID(bucket,0,0)
	v := o.Increment()
	return v
}

func GetIncrementID64(bucket string) uint64 {
	o:=GetAutoIncrement64ID(bucket,0,0)
	v := o.Increment64()
	return v
}

func ClearAllID() {
	count.l.Lock()
	count.ID= make(map[string]*AtomicID)
	count.l.Unlock()
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

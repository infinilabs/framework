package kv

import (
	"infini.sh/framework/core/kv"
	"sync"
)

type KVFilter struct {
}

var v = []byte("true")
var l sync.RWMutex

func (filter KVFilter) Open() error {
	return nil
}

func (filter KVFilter) Close() error {
	return nil
}

func (filter KVFilter) Exists(bucket string, key []byte) bool {
	b, _ := kv.GetValue(bucket, key)
	return b != nil
}

func (filter KVFilter) Add(bucket string, key []byte) error {
	return kv.AddValue(bucket, key, v)
}

func (filter KVFilter) Delete(bucket string, key []byte) error {
	return kv.DeleteKey(bucket, key)
}

func (filter KVFilter) CheckThenAdd(bucket string, key []byte) (b bool, err error) {
	l.Lock()
	defer l.Unlock()
	b = filter.Exists(bucket, key)
	if !b {
		err = filter.Add(bucket, key)
	}
	return b, err
}

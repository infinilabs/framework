package kv

import (
	"github.com/xujiajun/nutsdb"
	"infini.sh/framework/core/global"
	"path"
	"sync"
)

type KVFilter struct {
}

var v = []byte("true")
var l sync.RWMutex
var handler *nutsdb.DB

func (filter KVFilter) Open() error {
	l.Lock()
	defer l.Unlock()

	opt := nutsdb.Options{
		EntryIdxMode:         nutsdb.HintKeyValAndRAMIdxMode,
		SegmentSize:          8 * 1024 * 1024,
		NodeNum:              1,
		RWMode:               nutsdb.FileIO,
		SyncEnable:           true,
		StartFileLoadingMode: nutsdb.FileIO,
	}

	opt.Dir = path.Join(global.Env().GetDataDir(),"filter")
	var err error
	h, err := nutsdb.Open(opt)
	if err != nil {
		panic(err)
	}
	handler=h
	return nil
}

func (filter KVFilter) Close() error {
	if handler!=nil{
		handler.Close()
	}
	return nil
}

func (filter KVFilter) Exists(bucket string, key []byte) bool {

	var entry *nutsdb.Entry
	if err := handler.View(
		func(tx *nutsdb.Tx) error {
			if e, err := tx.Get(bucket, key); err != nil {
				return err
			} else {
				entry=e
			}
			return nil
		}); err != nil {
	}

	if entry!=nil{
		return true
	}
		return false
}

func (filter KVFilter) Add(bucket string, key []byte) error {
	err := handler.Update(
		func(tx *nutsdb.Tx) error {
			val := []byte("0")
			if err := tx.Put(bucket, key, val, 0); err != nil {
				return err
			}
			return nil
		})

	return err
}

func (filter KVFilter) Delete(bucket string, key []byte) error {
	return handler.Update(
		func(tx *nutsdb.Tx) error {
			key := []byte("name1")
			bucket := "bucket1"
			if err := tx.Delete(bucket, key); err != nil {
				return err
			}
			return nil
		})
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

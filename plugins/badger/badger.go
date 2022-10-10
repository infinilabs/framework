/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"github.com/bkaradzic/go-lz4"
	log "github.com/cihub/seelog"
	"github.com/dgraph-io/badger/v3"
	"infini.sh/framework/core/global"
	"path"
	"sync"
)

type BadgerFilter struct {
	Path string  `config:"path"`
}

var v = []byte("true")

var record sync.RWMutex

var l sync.RWMutex
var buckets =sync.Map{}
func (filter *BadgerFilter) Open() error {
	if filter.Path==""{
		filter.Path= path.Join(global.Env().GetDataDir(),"badger")
	}
	return nil
}

func (filter *BadgerFilter)getOrInitBucket(bucket string)*badger.DB  {
	item,ok:=buckets.Load(bucket)
	if ok{
		db,ok:=item.(*badger.DB)
		if ok{
			if db!=nil{
				return db
			}
		}
	}

	l.Lock()
	defer l.Unlock()

	log.Debugf("init badger database [%v]",bucket)

	//double check after lock
	item,ok=buckets.Load(bucket)
	if ok{
		return item.(*badger.DB)
	}

	dir := path.Join(filter.Path,bucket)

	var err error
	option:=badger.DefaultOptions(dir)

	if !global.Env().IsDebug{
		option.Logger=nil
	}

	h, err := badger.Open(option)
	if err != nil {
		panic(err)
	}
	buckets.Store(bucket,h)
	return h
}

func (filter *BadgerFilter) Close() error {
	buckets.Range(func(key, value any) bool {
		db,ok:=value.(*badger.DB)
		if ok{
			err:=db.Close()
			if err!=nil{
				panic(err)
			}
		}
		return true
	})
	return nil
}

func (filter *BadgerFilter) Exists(bucket string, key []byte) bool {

	var exists=false
	_:filter.getOrInitBucket(bucket).View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if item!=nil&&err==nil{
			exists=true
		}
		return nil
	})
	return exists
}

var zeroVal = []byte("0")

func (filter *BadgerFilter) Add(bucket string, key []byte) error {
	return filter.AddValue(bucket,key,zeroVal)
}

func (filter *BadgerFilter) Delete(bucket string, key []byte) error {
	var err error
	err=filter.getOrInitBucket(bucket).View(func(txn *badger.Txn) error {
		 err = txn.Delete(key)
		return nil
	})
	return err
}

func (filter *BadgerFilter) CheckThenAdd(bucket string, key []byte) (b bool, err error) {
	//TODO remove this lock
	record.Lock()
	defer record.Unlock()
	b = filter.Exists(bucket, key)
	if !b {
		err = filter.Add(bucket, key)
	}
	return b, err
}

//for kv implementation
func (filter *BadgerFilter) GetValue(bucket string, key []byte) ([]byte, error) {
	var valCopy []byte
	var err error
	var item *badger.Item
	err=filter.getOrInitBucket(bucket).View(func(txn *badger.Txn) error {
			item, err= txn.Get(key)
				if item!=nil&&err==nil{
				err = item.Value(func(val []byte) error {
					valCopy = append([]byte{}, val...)
					return nil
				})
		}
		return nil
	})
	return valCopy,err
}

func (filter *BadgerFilter) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	d,err:=filter.GetValue(bucket,key)
	if err!=nil{
		return d, err
	}
	data, err := lz4.Decode(nil, d)
	if err != nil {
		log.Error("Failed to decode:", err)
		return nil, err
	}
	return data,err
}

func (filter *BadgerFilter) AddValueCompress(bucket string, key []byte, value []byte) error {
	value, err := lz4.Encode(nil, value)
	if err != nil {
		log.Error("Failed to encode:", err)
		return err
	}
	return filter.AddValue(bucket, key, value)
}

func (filter *BadgerFilter) AddValue(bucket string, key []byte, value []byte) error {
	err := filter.getOrInitBucket(bucket).Update(func(txn *badger.Txn) error {
		err := txn.Set(key, value)
		return err
	})
	return err
}

func (filter *BadgerFilter) ExistsKey(bucket string, key []byte) (bool, error) {
	ok:= filter.Exists(bucket,key)
	return ok,nil
}

func (filter *BadgerFilter) DeleteKey(bucket string, key []byte) error {
	return filter.Delete(bucket,key)
}
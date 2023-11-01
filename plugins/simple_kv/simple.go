/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package simple_kv

import (
	"errors"
	"sync"

	"github.com/bkaradzic/go-lz4"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/util"
)

var record sync.RWMutex


func (filter *SimpleKV) Open() error {
	return nil
}

func (filter *SimpleKV) Close() error {
	return nil
}

func (filter *SimpleKV) Exists(bucket string, key []byte) bool {

	valCopy, err:=filter.kvstore.Get(joinKey(bucket, key))
	if err != nil {
		return false
	}

	if len(valCopy) != 0 {
		return true
	}

	return false
}

var zeroVal = []byte("0")

func (filter *SimpleKV) Add(bucket string, key []byte) error {
	return filter.AddValue(bucket, key, zeroVal)
}

func (filter *SimpleKV) Delete(bucket string, key []byte) error {
	return filter.kvstore.Delete(joinKey(bucket, key))
}

func (filter *SimpleKV) CheckThenAdd(bucket string, key []byte) (b bool, err error) {
	record.Lock()
	defer record.Unlock()
	b = filter.Exists(bucket, key)
	if !b {
		err = filter.Add(bucket, key)
	}
	return b, err
}

func (filter *SimpleKV) GetValue(bucket string, key []byte) ([]byte, error) {

	if filter.closed {
		return nil, errors.New("module closed")
	}

	valCopy, err:=filter.kvstore.Get(joinKey(bucket, key))
	return valCopy, err
}

func (filter *SimpleKV) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	d, err := filter.GetValue(bucket, key)
	if err != nil {
		return d, err
	}
	if len(d) == 0 {
		return nil, nil
	}

	data, err := lz4.Decode(nil, d)
	if err != nil {
		log.Error("Failed to decode:", err)
		return nil, err
	}
	return data, err
}

func (filter *SimpleKV) AddValueCompress(bucket string, key []byte, value []byte) error {
	value, err := lz4.Encode(nil, value)
	if err != nil {
		log.Error("Failed to encode:", err)
		return err
	}

	return filter.AddValue(bucket, key, value)
}

func joinKey(bucket string, key []byte) string {
	return bucket + "," + util.UnsafeBytesToString(key)
}

func (filter *SimpleKV) AddValue(bucket string, key []byte, value []byte) error {
	if filter.closed {
		return errors.New("module closed")
	}
	return filter.kvstore.Set(joinKey(bucket, key), value)
}

func (filter *SimpleKV) ExistsKey(bucket string, key []byte) (bool, error) {
	ok := filter.Exists(bucket, key)
	return ok, nil
}

func (filter *SimpleKV) DeleteKey(bucket string, key []byte) error {
	return filter.Delete(bucket, key)
}

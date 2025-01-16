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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"errors"
	"infini.sh/framework/core/stats"
	"path"
	"sync"
	"time"

	"github.com/bkaradzic/go-lz4"
	log "github.com/cihub/seelog"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

var record sync.RWMutex
var l sync.RWMutex

var buckets = sync.Map{}

func (filter *Module) Open() error {
	if filter.cfg.Path == "" {
		filter.cfg.Path = path.Join(global.Env().GetDataDir(), "badger")
	}

	if filter.cfg.SingleBucketMode {
		filter.bucket = filter.getOrInitBucket("default")
	}

	if filter.cfg.ValueLogGCEnabled {
		global.RegisterBackgroundCallback(&global.BackgroundTask{Tag: "badger_gc", Func: func() {
			buckets.Range(func(key, value interface{}) bool {
				if db, ok := value.(*badger.DB); ok {
					err := db.RunValueLogGC(filter.cfg.ValueLogDiscardRatio)
					if err != nil && err != badger.ErrNoRewrite {
						log.Errorf("running value log gc failed: %v", err)
					}
				}
				return true
			})
		}, Interval: time.Duration(filter.cfg.ValueLogGCIntervalInSeconds) * time.Second})
	}

	return nil
}

func (filter *Module) mustGetBucket(bucket string) *badger.DB {
	if filter.closed {
		panic(errors.New("module closed"))
	}

	if filter.cfg.SingleBucketMode {
		if filter.bucket == nil {
			panic("invalid badger module")
		}
		return filter.bucket
	}
	return filter.getOrInitBucket(bucket)
}

func (filter *Module) getOrInitBucket(bucket string) *badger.DB {
	item, ok := buckets.Load(bucket)
	if ok {
		db, ok := item.(*badger.DB)
		if ok {
			if db != nil {
				return db
			}
		}
	}

	l.Lock()
	defer l.Unlock()

	//double check after lock
	item, ok = buckets.Load(bucket)
	if ok {
		return item.(*badger.DB)
	}

	log.Debugf("init badger database [%v]", bucket)

	dir := path.Join(filter.cfg.Path, bucket)

	var err error
	option := badger.DefaultOptions(dir)
	option.InMemory = filter.cfg.InMemoryMode
	option.MemTableSize = filter.cfg.MemTableSize
	option.ValueLogMaxEntries = filter.cfg.ValueLogMaxEntries
	option.ValueThreshold = filter.cfg.ValueThreshold
	option.NumGoroutines = 1
	option.NumMemtables = filter.cfg.NumMemtables
	option.Compression = options.None
	option.MetricsEnabled = false
	option.NumLevelZeroTables = filter.cfg.NumLevelZeroTables
	option.NumLevelZeroTablesStall = filter.cfg.NumLevelZeroTablesStall
	option.SyncWrites = filter.cfg.SyncWrites
	option.CompactL0OnClose = true
	option.ValueLogFileSize = filter.cfg.ValueLogFileSize

	if !global.Env().IsDebug {
		option.Logger = nil
	}

	h, err := badger.Open(option)
	if err != nil {
		panic(err)
	}
	stats.Increment("badger", bucket+"::init")
	buckets.Store(bucket, h)
	return h
}

func (filter *Module) Close() error {

	if filter.cfg.SingleBucketMode {
		if filter.bucket != nil {
			filter.bucket.Close()
		}
	}

	buckets.Range(func(key, value any) bool {
		db, ok := value.(*badger.DB)
		if ok {
			err := db.Close()
			if err != nil {
				panic(err)
			}
		}
		return true
	})
	return nil
}

func (filter *Module) Exists(bucket string, key []byte) bool {

	stats.Increment("badger", bucket+"::exists")

	if filter.cfg.SingleBucketMode {
		key = joinKey(bucket, key)
	}

	var exists = false
	bkt := filter.getOrInitBucket(bucket)
	bkt.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if item != nil && err == nil {
			exists = true
		}
		return nil
	})
	return exists
}

var zeroVal = []byte("0")

func (filter *Module) Add(bucket string, key []byte) error {
	return filter.AddValue(bucket, key, zeroVal)
}

func (filter *Module) Delete(bucket string, key []byte) error {

	stats.Increment("badger", bucket+"::delete")

	if filter.cfg.SingleBucketMode {
		key = joinKey(bucket, key)
	}

	var err error
	bkt := filter.getOrInitBucket(bucket)
	err = bkt.Update(func(txn *badger.Txn) error {
		err = txn.Delete(key)
		return err
	})
	return err
}

func (filter *Module) CheckThenAdd(bucket string, key []byte) (b bool, err error) {
	//TODO remove this lock
	record.Lock()
	defer record.Unlock()
	b = filter.Exists(bucket, key)
	if !b {
		err = filter.Add(bucket, key)
	}
	return b, err
}

// for kv implementation
func (filter *Module) GetValue(bucket string, key []byte) ([]byte, error) {

	if filter.closed {
		return nil, errors.New("module closed")
	}

	stats.Increment("badger", bucket+"::get")

	if filter.cfg.SingleBucketMode {
		key = joinKey(bucket, key)
	}

	var valCopy []byte
	var err error
	var item *badger.Item

	bkt := filter.getOrInitBucket(bucket)
	err = bkt.View(func(txn *badger.Txn) error {
		if txn == nil {
			return errors.New("invalid txn")
		}
		item, err = txn.Get(key)
		if item != nil && err == nil {
			err = item.Value(func(val []byte) error {
				valCopy = append([]byte{}, val...)
				return nil
			})
		}
		return nil
	})
	return valCopy, err
}

func (filter *Module) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	d, err := filter.GetValue(bucket, key)
	if err != nil {
		return d, err
	}
	if len(d) == 0 {
		return nil, nil
	}

	stats.Increment("badger", bucket+"::get_compress")

	data, err := lz4.Decode(nil, d)
	if err != nil {
		log.Error("Failed to decode:", err)
		return nil, err
	}
	return data, err
}

func (filter *Module) AddValueCompress(bucket string, key []byte, value []byte) error {
	value, err := lz4.Encode(nil, value)
	if err != nil {
		log.Error("Failed to encode:", err)
		return err
	}

	stats.Increment("badger", bucket+"::add_compress")

	return filter.AddValue(bucket, key, value)
}

func joinKey(bucket string, key []byte) []byte {
	return util.UnsafeStringToBytes(bucket + "," + util.UnsafeBytesToString(key))
}

func (filter *Module) AddValue(bucket string, key []byte, value []byte) error {
	if filter.closed {
		return errors.New("module closed")
	}

	stats.Increment("badger", bucket+"::add")

	if filter.cfg.SingleBucketMode {
		key = joinKey(bucket, key)
	}
	bkt := filter.getOrInitBucket(bucket)
	err := bkt.Update(func(txn *badger.Txn) error {
		err := txn.Set(key, value)
		return err
	})
	return err
}

func (filter *Module) ExistsKey(bucket string, key []byte) (bool, error) {
	ok := filter.Exists(bucket, key)
	return ok, nil
}

func (filter *Module) DeleteKey(bucket string, key []byte) error {
	return filter.Delete(bucket, key)
}

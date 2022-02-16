package nutsdb

import (
	"github.com/xujiajun/nutsdb"
	"infini.sh/framework/core/global"
	"path"
	"github.com/bkaradzic/go-lz4"
	log "github.com/cihub/seelog"
	"sync"
)

type NutsdbKVFilter struct {
}

var v = []byte("true")
var l sync.RWMutex
var handler *nutsdb.DB

func (filter *NutsdbKVFilter) Open() error {
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

	opt.Dir = path.Join(global.Env().GetDataDir(),"kvdb")
	var err error
	h, err := nutsdb.Open(opt)
	if err != nil {
		panic(err)
	}
	handler=h
	return nil
}

func (filter *NutsdbKVFilter) Close() error {
	if handler!=nil{
		handler.Close()
	}
	return nil
}

func (filter *NutsdbKVFilter) Exists(bucket string, key []byte) bool {

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

func (filter *NutsdbKVFilter) Add(bucket string, key []byte) error {
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

func (filter *NutsdbKVFilter) Delete(bucket string, key []byte) error {
	return handler.Update(
		func(tx *nutsdb.Tx) error {
			if err := tx.Delete(bucket, key); err != nil {
				return err
			}
			return nil
		})
}

func (filter *NutsdbKVFilter) CheckThenAdd(bucket string, key []byte) (b bool, err error) {
	l.Lock()
	defer l.Unlock()
	b = filter.Exists(bucket, key)
	if !b {
		err = filter.Add(bucket, key)
	}
	return b, err
}



//for kv implementation
func (f *NutsdbKVFilter) GetValue(bucket string, key []byte) ([]byte, error) {
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
		return entry.Value,nil
	}
	return nil,nil
}

func (f *NutsdbKVFilter) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	d,err:=f.GetValue(bucket,key)
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

func (f *NutsdbKVFilter) AddValueCompress(bucket string, key []byte, value []byte) error {
	value, err := lz4.Encode(nil, value)
	if err != nil {
		log.Error("Failed to encode:", err)
		return err
	}
	return f.AddValue(bucket, key, value)
}

func (f *NutsdbKVFilter) AddValue(bucket string, key []byte, value []byte) error {
	err := handler.Update(
		func(tx *nutsdb.Tx) error {
			if err := tx.Put(bucket, key, value, 0); err != nil {
				return err
			}
			return nil
		})

	return err
}

func (f *NutsdbKVFilter) ExistsKey(bucket string, key []byte) (bool, error) {
	ok:= f.Exists(bucket,key)
	return ok,nil
}

func (f *NutsdbKVFilter) DeleteKey(bucket string, key []byte) error {
	return f.Delete(bucket,key)
}

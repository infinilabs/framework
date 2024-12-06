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

package kv

import (
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/errors"
)

type KVStore interface {
	Open() error

	Close() error

	GetValue(bucket string, key []byte) ([]byte, error)

	GetCompressedValue(bucket string, key []byte) ([]byte, error)

	AddValueCompress(bucket string, key []byte, value []byte) error

	AddValue(bucket string, key []byte, value []byte) error

	ExistsKey(bucket string, key []byte) (bool, error)

	DeleteKey(bucket string, key []byte) error

	//DeleteBucket(bucket string) error
}

var handler KVStore

func getKVHandler() KVStore {

	if handler == nil {
		panic(errors.New("kv store handler is not registered"))
	}
	return handler
}

func GetValue(bucket string, key []byte) ([]byte, error) {
	return getKVHandler().GetValue(bucket, key)
}

func GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	return getKVHandler().GetCompressedValue(bucket, key)
}

func AddValueCompress(bucket string, key []byte, value []byte) error {
	return getKVHandler().AddValueCompress(bucket, key, value)
}

func AddValue(bucket string, key []byte, value []byte) error {
	return getKVHandler().AddValue(bucket, key, value)
}

func ExistsKey(bucket string, key []byte) (bool, error) {
	return getKVHandler().ExistsKey(bucket, key)
}

func DeleteKey(bucket string, key []byte) error {
	return getKVHandler().DeleteKey(bucket, key)
}

//func DeleteBucket(bucket string) error {
//	return getKVHandler().DeleteBucket(bucket)
//}

var stores map[string]KVStore

func Register(name string, h KVStore) {
	log.Debugf("register kv store with type [%s]", name)
	if stores == nil {
		stores = map[string]KVStore{}
	}
	_, ok := stores[name]
	if ok {
		panic(errors.Errorf("KV handler with same name: %v already exists", name))
	}

	stores[name] = h

	handler = h

	log.Debug("register kv store: ", name)
}

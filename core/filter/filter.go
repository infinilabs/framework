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

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filter

import (
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/errors"
)

// Filter is used to check if the object is in the filter or not
type Filter interface {
	Exists(bucket string, key []byte) bool
	Add(bucket string, key []byte) error
	Delete(bucket string, key []byte) error

	// CheckThenAdd will check if the key was exist in the bucket or not,
	// will return the previous status, and also add the key to the bucket if not exists
	CheckThenAdd(bucket string, key []byte) (bool, error)
	Open() error
	Close() error
}

var handler Filter

func getHandler() Filter {
	if handler == nil {
		panic(errors.New("filter handler is not registered"))
	}
	return handler
}

// Exists checks if the key are already in filter bucket
func Exists(bucket string, key []byte) bool {
	return getHandler().Exists(bucket, key)
}

// Add will add key to filter bucket
func Add(bucket string, key []byte) error {
	return getHandler().Add(bucket, key)
}

// Remove will remove key from bucket
func Remove(bucket string, key []byte) error {
	return getHandler().Delete(bucket, key)
}

// CheckThenAdd will check first and if the key is not in the filter bucket, then it will add it and return false, if the key is already in the bucket, it will just return true
func CheckThenAdd(bucket string, key []byte) (bool, error) {
	return getHandler().CheckThenAdd(bucket, key)
}

var filters map[string]Filter

func Register(name string, h Filter) {
	if filters == nil {
		filters = map[string]Filter{}
	}
	_, ok := filters[name]
	if ok {
		panic(errors.Errorf("filter with same name: %v already exists", name))
	}

	filters[name] = h

	handler = h

	log.Debug("register filter: ", name)

}

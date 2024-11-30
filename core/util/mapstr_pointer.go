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

// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package util

import (
	"sync/atomic"
	"unsafe"
)

// MapStrPointer stores a pointer to atomically get/set a MapStr object
// This should give faster access for use cases with lots of reads and a few
// changes.
// It's important to note that modifying the map is not thread safe, only fully
// replacing it.
type MapStrPointer struct {
	p *unsafe.Pointer
}

// NewMapStrPointer initializes and returns a pointer to the given MapStr
func NewMapStrPointer(m MapStr) MapStrPointer {
	pointer := unsafe.Pointer(&m)
	return MapStrPointer{p: &pointer}
}

// Get returns the MapStr stored under this pointer
func (m MapStrPointer) Get() MapStr {
	if m.p == nil {
		return nil
	}
	return *(*MapStr)(atomic.LoadPointer(m.p))
}

// Set stores a pointer the given MapStr, replacing any previous one
func (m *MapStrPointer) Set(p MapStr) {
	atomic.StorePointer(m.p, unsafe.Pointer(&p))
}

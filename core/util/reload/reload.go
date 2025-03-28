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

package reload

import (
	"github.com/pkg/errors"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
	"sync"
)

// Register holds a registry of reloadable objects
var Register = NewRegistry()

// ConfigWithMeta holds a pair of common.Config and optional metadata for it
type ConfigWithMeta struct {
	// Config to store
	Config *config.Config

	// Meta data related to this config
	Meta *util.MapStrPointer
}

// ReloadableList provides a method to reload the configuration of a list of entities
type ReloadableList interface {
	Reload(configs []*ConfigWithMeta) error
}

// Reloadable provides a method to reload the configuration of an entity
type Reloadable interface {
	Reload(config *ConfigWithMeta) error
}

// ReloadableFunc wraps a custom function in order to implement the Reloadable interface.
type ReloadableFunc func(config *ConfigWithMeta) error

// Registry of reloadable objects and lists
type Registry struct {
	sync.RWMutex
	confsLists map[string]ReloadableList
	confs      map[string]Reloadable
}

// NewRegistry initializes and returns a reload registry
func NewRegistry() *Registry {
	return &Registry{
		confsLists: make(map[string]ReloadableList),
		confs:      make(map[string]Reloadable),
	}
}

// Register declares a reloadable object
func (r *Registry) Register(name string, obj Reloadable) error {
	r.Lock()
	defer r.Unlock()

	if obj == nil {
		return errors.New("got a nil object")
	}

	if r.nameTaken(name) {
		return errors.Errorf("%s configuration list is already registered", name)
	}

	r.confs[name] = obj
	return nil
}

// RegisterList declares a reloadable list of configurations
func (r *Registry) RegisterList(name string, list ReloadableList) error {
	r.Lock()
	defer r.Unlock()

	if list == nil {
		return errors.New("got a nil object")
	}

	if r.nameTaken(name) {
		return errors.Errorf("%s configuration is already registered", name)
	}

	r.confsLists[name] = list
	return nil
}

// MustRegister declares a reloadable object
func (r *Registry) MustRegister(name string, obj Reloadable) {
	if err := r.Register(name, obj); err != nil {
		panic(err)
	}
}

// MustRegisterList declares a reloadable object list
func (r *Registry) MustRegisterList(name string, list ReloadableList) {
	if err := r.RegisterList(name, list); err != nil {
		panic(err)
	}
}

// GetRegisteredNames returns the list of names registered
func (r *Registry) GetRegisteredNames() []string {
	r.RLock()
	defer r.RUnlock()
	var names []string

	for name := range r.confs {
		names = append(names, name)
	}

	for name := range r.confsLists {
		names = append(names, name)
	}

	return names
}

// GetReloadable returns the reloadable object with the given name, nil if not found
func (r *Registry) GetReloadable(name string) Reloadable {
	r.RLock()
	defer r.RUnlock()
	return r.confs[name]
}

// GetReloadableList returns the reloadable list with the given name, nil if not found
func (r *Registry) GetReloadableList(name string) ReloadableList {
	r.RLock()
	defer r.RUnlock()
	return r.confsLists[name]
}

func (r *Registry) nameTaken(name string) bool {
	if _, ok := r.confs[name]; ok {
		return true
	}

	if _, ok := r.confsLists[name]; ok {
		return true
	}

	return false
}

// Reload calls the underlying function.
func (fn ReloadableFunc) Reload(config *ConfigWithMeta) error {
	return fn(config)
}

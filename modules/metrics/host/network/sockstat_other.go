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

//go:build !linux
// +build !linux

package network

import (
	"github.com/shirou/gopsutil/v3/net"
	"infini.sh/framework/core/util"
)

// a stub function for non-linux systems
// get a list of platform-specific enhancements and apply them to our mapStr object.
func applyEnhancements(data util.MapStr) (util.MapStr, error) {
	return data, nil
}

// connections gets connection information
func connections(kind string) ([]net.ConnectionStat, error) {
	return net.Connections(kind)
}

// Resolver is an interface for HostFS resolvers. This is meant to be generic and (hopefully) future-proof way of dealing with a user-supplied root filesystem path.
// A resolver-style function serves two ends:
// 1) if we attempt to stop consumers from merely "saving off" a string, the underlying implementation can update hostfs values and pass the new paths along to consumers
// 2) This stops different bits of code from making different assumptions about what's in hostfs and otherwise treating the concept differently. It's easy to mix up "hostfs" and "procfs" and "sysfs" as concepts.
// A single resolver forces this logic to be a little more centralized.
type Resolver interface {
	// ResolveHostFS Resolves a path based on a user-set HostFS flag, in cases where a user wants to monitor an alternate filesystem root
	// If no user root has been set, it will return the input string
	ResolveHostFS(string) string
	// IsSet returns true if the user has set an alternate filesystem root
	IsSet() bool
	// Join emulates the behavior of filepath.join
	Join(...string) string
}

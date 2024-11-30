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

package safemapstr
import (
	"infini.sh/framework/core/util"
	"testing"

	"github.com/stretchr/testify/assert"
	)

func TestPut(t *testing.T) {
	m := util.MapStr{
		"subMap": util.MapStr{
			"a": 1,
		},
	}

	// Add new value to the top-level.
	err := Put(m, "a", "ok")
	assert.NoError(t, err)
	assert.Equal(t, util.MapStr{"a": "ok", "subMap": util.MapStr{"a": 1}}, m)

	// Add new value to subMap.
	err = Put(m, "subMap.b", 2)
	assert.NoError(t, err)
	assert.Equal(t, util.MapStr{"a": "ok", "subMap": util.MapStr{"a": 1, "b": 2}}, m)

	// Overwrite a value in subMap.
	err = Put(m, "subMap.a", 2)
	assert.NoError(t, err)
	assert.Equal(t, util.MapStr{"a": "ok", "subMap": util.MapStr{"a": 2, "b": 2}}, m)

	// Add value to map that does not exist.
	m = util.MapStr{}
	err = Put(m, "subMap.newMap.a", 1)
	assert.NoError(t, err)
	assert.Equal(t, util.MapStr{"subMap": util.MapStr{"newMap": util.MapStr{"a": 1}}}, m)
}

func TestPutRenames(t *testing.T) {
	assert := assert.New(t)

	a := util.MapStr{}
	Put(a, "com.docker.swarm.task", "x")
	Put(a, "com.docker.swarm.task.id", 1)
	Put(a, "com.docker.swarm.task.name", "foobar")
	assert.Equal(util.MapStr{"com": util.MapStr{"docker": util.MapStr{"swarm": util.MapStr{
		"task": util.MapStr{
			"id":    1,
			"name":  "foobar",
			"value": "x",
		}}}}}, a)

	// order is not important:
	b := util.MapStr{}
	Put(b, "com.docker.swarm.task.id", 1)
	Put(b, "com.docker.swarm.task.name", "foobar")
	Put(b, "com.docker.swarm.task", "x")
	assert.Equal(util.MapStr{"com": util.MapStr{"docker": util.MapStr{"swarm": util.MapStr{
		"task": util.MapStr{
			"id":    1,
			"name":  "foobar",
			"value": "x",
		}}}}}, b)
}

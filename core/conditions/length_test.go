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

package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLengthEqualsCreate(t *testing.T) {
	config := Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"tags": 1,
		}},
	}

	_, err := NewCondition(&config)
	assert.NoError(t, err)
}

func TestLengthEqualsSingleFieldPositiveMatch(t *testing.T) {
	testConfig(t, true, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"tags": 3,
		}},
	})

	testConfig(t, true, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"proc.keywords": 2,
		}},
	})

	testConfig(t, true, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"proc.username": 6,
		}},
	})
}

func TestLengthEqualsNegativeMatch(t *testing.T) {
	testConfig(t, false, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"proc.keywords": 4,
		}},
	})

	testConfig(t, false, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"proc.username": 4,
		}},
	})

	testConfig(t, false, secdTestEvent, &Config{
		LengthEquals: &Fields{fields: map[string]interface{}{
			"proc.pid": -1,
		}},
	})
}
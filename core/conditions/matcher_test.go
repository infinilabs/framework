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
	event2 "github.com/rubyniu105/framework/core/event"
	"github.com/rubyniu105/framework/core/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegxpCreate(t *testing.T) {
	config := Config{
		Regexp: &Fields{fields: map[string]interface{}{
			"proc.name": "58gdhsga-=kw++w00",
		}},
	}
	_, err := NewCondition(&config)
	assert.Error(t, err)
}

func TestContainsMultiFieldPositiveMatch(t *testing.T) {
	testConfig(t, true, secdTestEvent, &Config{
		Contains: &Fields{fields: map[string]interface{}{
			"proc.name":     "sec",
			"proc.username": "monica",
		}},
	})
}

func TestContainsMultiFieldNegativeMatch(t *testing.T) {
	testConfig(t, false, secdTestEvent, &Config{
		Contains: &Fields{fields: map[string]interface{}{
			"type":      "process",
			"proc.name": "secddd",
		}},
	})
}

func TestContainsSingleFieldPositiveMatch(t *testing.T) {
	testConfig(t, true, secdTestEvent, &Config{
		Contains: &Fields{fields: map[string]interface{}{
			"proc.keywords": "bar",
		}},
	})
}

func TestContainsArrayOfStringPositiveMatch(t *testing.T) {
	testConfig(t, true, secdTestEvent, &Config{
		Contains: &Fields{fields: map[string]interface{}{
			"tags": "prod",
		}},
	})
}

func TestRegexpCondition(t *testing.T) {
	configs := []Config{
		{
			Regexp: &Fields{fields: map[string]interface{}{
				"source": "apache2/error.*",
			}},
		},

		{
			Regexp: &Fields{fields: map[string]interface{}{
				"source": "apache2/access.*",
			}},
		},

		{
			Regexp: &Fields{fields: map[string]interface{}{
				"source":  "apache2/error.*",
				"message": "[client 1.2.3.4]",
			}},
		},
	}

	conds := GetConditions(t, configs)

	event := &event2.Event{
		Timestamp: time.Now(),
		Fields: util.MapStr{
			"message":    `[Fri Dec 16 01:46:23 2005] [error] [client 1.2.3.4] Directory index forbidden by rule: /home/test/`,
			"source":     "/var/log/apache2/error.log",
			"type":       "log",
			"input_type": "log",
			"offset":     30,
		},
	}

	event1 := &event2.Event{
		Timestamp: time.Now(),
		Fields: util.MapStr{
			"message":    `127.0.0.1 - - [28/Jul/2006:10:27:32 -0300] "GET /hidden/ HTTP/1.0" 404 7218`,
			"source":     "/var/log/apache2/access.log",
			"type":       "log",
			"input_type": "log",
			"offset":     30,
		},
	}

	assert.True(t, conds[0].Check(event))
	assert.False(t, conds[1].Check(event))
	assert.True(t, conds[2].Check(event))

	assert.True(t, conds[1].Check(event1))
	assert.False(t, conds[2].Check(event1))
}

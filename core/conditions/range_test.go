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
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

)

func TestRangeCreateNumeric(t *testing.T) {
	config := Config{
		Range: &Fields{fields: map[string]interface{}{
			"gtr": 0.3,
		}},
	}
	_, err := NewCondition(&config)
	assert.Error(t, err)
}

func TestRangeCreateLexicographic(t *testing.T) {
	config := Config{
		Range: &Fields{fields: map[string]interface{}{
			"gt": "fdfdd",
		}},
	}
	_, err := NewCondition(&config)
	assert.Error(t, err)
}

func TestRangeSingleFieldNegativeMatch(t *testing.T) {
	testConfig(t, false, secdTestEvent, &Config{
		Range: &Fields{fields: map[string]interface{}{
			"proc.cpu.total_p.gt": 0.5,
		}},
	})
}

func TestClosedRangeConditionPositiveMatch(t *testing.T) {
	testConfig(t, true, httpResponseTestEvent, &Config{
		Range: &Fields{fields: map[string]interface{}{
			"http.code.gte": 200,
			"http.code.lt":  300,
		}},
	})
}

func TestOpenRangeConditionPositiveMatch(t *testing.T) {
	testConfig(t, true, httpResponseTestEvent, &Config{
		Range: &Fields{fields: map[string]interface{}{
			"bytes_out.gte": 2800,
		}},
	})
}

func TestMultipleOpenRangeConditionNegativeMatch(t *testing.T) {
	testConfig(t, false, httpResponseTestEvent, &Config{
		Range: &Fields{fields: map[string]interface{}{
			"bytes_out.gte":   2800,
			"responsetime.gt": 30,
		}},
	})
}

var procCPURangeConfig = &Config{
	Range: &Fields{fields: map[string]interface{}{
		"proc.cpu.total_p.gte": 0.5,
	}},
}

func TestOpenGteRangeConditionPositiveMatch(t *testing.T) {
	mdWorkerTestEvent := &event.Event{
		Timestamp: time.Now(),
		Fields: util.MapStr{
			"proc": util.MapStr{
				"cmdline": "/System/Library/Frameworks/CoreServices.framework/Frameworks/Metadata.framework/Versions/A/Support/mdworker -s mdworker -c MDSImporterWorker -m com.apple.mdworker.single",
				"cpu": util.MapStr{
					"start_time": "09:19",
					"system":     22,
					"total":      66,
					"total_p":    0.6,
					"user":       44,
				},
				"name":     "mdworker",
				"pid":      44978,
				"ppid":     1,
				"state":    "running",
				"username": "test",
			},
			"type": "process",
		},
	}

	testConfig(t, true, mdWorkerTestEvent, procCPURangeConfig)
}

func TestOpenGteRangeConditionNegativeMatch(t *testing.T) {
	testConfig(t, false, httpResponseTestEvent, procCPURangeConfig)
}

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

package pipeline

import (
	"errors"
	"testing"

	cfg1 "github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/event"
	"github.com/rubyniu105/framework/core/util"
	"github.com/stretchr/testify/assert"
)

type countFilter struct {
	N int
}

func (c *countFilter) Run(e *event.Event) (*event.Event, error) {
	c.N++
	return e, nil
}

func (c *countFilter) String() string { return "count" }

//func TestWhenProcessor(t *testing.T) {
//	type config map[string]interface{}
//
//	tests := []struct {
//		title    string
//		filter   config
//		events   []util.MapStr
//		expected int
//	}{
//		{
//			"condition_matches",
//			config{"when.equals.i": 10},
//			[]util.MapStr{{"i": 10}},
//			1,
//		},
//		{
//			"condition_fails",
//			config{"when.equals.i": 11},
//			[]util.MapStr{{"i": 10}},
//			0,
//		},
//		{
//			"no_condition",
//			config{},
//			[]util.MapStr{{"i": 10}},
//			1,
//		},
//		{
//			"condition_matches",
//			config{"when.exists": []string{"i"}},
//			[]util.MapStr{{"i": 10}},
//			1,
//		},
//		{
//			"condition_fails",
//			config{"when.exists": []string{"j"}},
//			[]util.MapStr{{"i": 10}},
//			0,
//		},
//	}
//
//	for i, test := range tests {
//		t.Logf("run test (%v): %v", i, test.title)
//
//		cfg, err := cfg1.NewConfigFrom(test.filter)
//		if err != nil {
//			t.Error(err)
//			continue
//		}
//
//		cf := &countFilter{}
//		filter, err := NewConditional(func(_ *cfg1.Config) (p1.ProcessorBase, error) {
//			return cf, nil
//		})(cfg)
//		if err != nil {
//			t.Error(err)
//			continue
//		}
//
//		for _, fields := range test.events {
//			event := &event.Event{
//				Timestamp: time.Now(),
//				Fields:    fields,
//			}
//			_, err := filter.Run(event)
//			if err != nil {
//				t.Error(err)
//			}
//		}
//
//		assert.Equal(t, test.expected, cf.N)
//	}
//}

func TestConditionRuleInitErrorPropagates(t *testing.T) {
	testErr := errors.New("test")
	filter, err := NewConditional(func(_ *cfg1.Config) (Processor, error) {
		return nil, testErr
	})(cfg1.NewConfig())

	assert.Equal(t, testErr, err)
	assert.Nil(t, filter)
}

type testCase struct {
	event util.MapStr
	want  util.MapStr
	cfg   string
}

//func testProcessors(t *testing.T, cases map[string]testCase) {
//	for name, test := range cases {
//		test := test
//		t.Run(name, func(t *testing.T) {
//			c, err := cfg1.NewConfigWithYAML([]byte(test.cfg), "test "+name)
//			if err != nil {
//				t.Fatal(err)
//			}
//
//			var pluginConfig common.PluginConfig
//			if err = c.Unpack(&pluginConfig); err != nil {
//				t.Fatal(err)
//			}
//
//			processor, err := common.NewPipeline(pluginConfig)
//			if err != nil {
//				t.Fatal(err)
//			}
//
//			result, err := processor.Run(&event.Event{Fields: test.event.Clone()})
//			if err != nil {
//				t.Fatal(err)
//			}
//			assert.Equal(t, test.want, result.Fields)
//		})
//	}
//}
//
//func TestIfElseThenProcessor(t *testing.T) {
//	const ifThen = `
//- if:
//   range.uid.lt: 500
// then:
//   - add_fields: {target: "", fields: {uid_type: reserved}}
//`
//
//	const ifThenElse = `
//- if:
//   range.uid.lt: 500
// then:
//   - add_fields: {target: "", fields: {uid_type: reserved}}
// else:
//   - add_fields: {target: "", fields: {uid_type: user}}
//`
//
//	const ifThenElseSingleProcessor = `
//- if:
//   range.uid.lt: 500
// then:
//   add_fields: {target: "", fields: {uid_type: reserved}}
// else:
//   add_fields: {target: "", fields: {uid_type: user}}
//`
//
//	const ifThenElseIf = `
//- if:
//   range.uid.lt: 500
// then:
//   - add_fields: {target: "", fields: {uid_type: reserved}}
// else:
//   if:
//     equals.uid: 500
//   then:
//     add_fields: {target: "", fields: {uid_type: "eq_500"}}
//   else:
//     add_fields: {target: "", fields: {uid_type: "gt_500"}}
//`
//
//	testProcessors(t, map[string]testCase{
//		"if-then-true": {
//			event: util.MapStr{"uid": 411},
//			want:  util.MapStr{"uid": 411, "uid_type": "reserved"},
//			cfg:   ifThen,
//		},
//		"if-then-false": {
//			event: util.MapStr{"uid": 500},
//			want:  util.MapStr{"uid": 500},
//			cfg:   ifThen,
//		},
//		"if-then-else-true": {
//			event: util.MapStr{"uid": 411},
//			want:  util.MapStr{"uid": 411, "uid_type": "reserved"},
//			cfg:   ifThenElse,
//		},
//		"if-then-else-false": {
//			event: util.MapStr{"uid": 500},
//			want:  util.MapStr{"uid": 500, "uid_type": "user"},
//			cfg:   ifThenElse,
//		},
//		"if-then-else-false-single-processor": {
//			event: util.MapStr{"uid": 500},
//			want:  util.MapStr{"uid": 500, "uid_type": "user"},
//			cfg:   ifThenElseSingleProcessor,
//		},
//		"if-then-else-if": {
//			event: util.MapStr{"uid": 500},
//			want:  util.MapStr{"uid": 500, "uid_type": "eq_500"},
//			cfg:   ifThenElseIf,
//		},
//	})
//}

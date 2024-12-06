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
	"fmt"

	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/core/util/match"

	logger "github.com/cihub/seelog"
)

type matcherMap map[string]match.Matcher
type rawMap map[string]interface{}

// Matcher is a Condition that works with beat's internal notion of a string matcher.
type Matcher struct {
	name     string
	matchers matcherMap
	raw      rawMap
}

// NewMatcherCondition builds a new Matcher with the given human name using the provided config fields.
// The compiler function will take those fields and compile them.
func NewMatcherCondition(
	name string,
	fields map[string]interface{},
	compile func(string) (match.Matcher, error),
) (condition Matcher, err error) {
	condition.name = name
	condition.raw = fields
	condition.matchers = matcherMap{}
	condition.raw = rawMap{}

	if len(fields) == 0 {
		return condition, nil
	}

	for field, value := range fields {
		var err error

		switch v := value.(type) {
		case string:
			condition.matchers[field], err = compile(v)
			if err != nil {
				return condition, err
			}

		default:
			return condition, fmt.Errorf("unexpected type %T of %v", value, value)
		}
	}

	return condition, nil
}

// Check determines whether the given event matches this condition.
func (c Matcher) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	if c.matchers == nil {
		return true
	}

	for field, matcher := range c.matchers {
		value, err := event.GetValue(field)
		if err != nil {
			if isDebug {
				logger.Warnf("'%s' does not exist: %s", field, err)
			}
			return false
		}

		switch v := value.(type) {
		case string:
			if matcher.MatchString(v) {
				continue
			}

		case []interface{}, []string:
			if matcher.MatchAnyString(v) {
				continue
			}
		default:
			str, err := util.ExtractString(value)
			if err != nil {
				if isDebug {
					logger.Warnf("unexpected type %T in %v condition as it accepts only strings.", value, c.name)
				}
				return false
			}

			if matcher.MatchString(str) {
				continue
			}
		}

		if isDebug {
			logger.Warnf("'%s' does not match expected patterns: %v", field, value)
		}
		return false
	}

	return true
}

func (c Matcher) String() string {
	return fmt.Sprintf("%v: %v", c.name, c.raw)
}

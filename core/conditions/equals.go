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

	logger "github.com/cihub/seelog"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// Equals is a Condition for testing string equality.
type Equals map[string]equalsValue

type equalsValue func(interface{}) bool

func equalsIntValue(i int64) equalsValue {
	return func(value interface{}) bool {
		if sValue, err := util.ExtractInt(value); err == nil {
			return sValue == i
		}
		logger.Warnf("expected int but got type %T in equals condition.", value)
		return false
	}
}

func equalsFloatValue(i float64) equalsValue {
	return func(value interface{}) bool {
		if sValue, err := util.ExtractFloat(value); err == nil {
			return sValue == i
		}
		logger.Warnf("expected float but got type %T in equals condition.", value)
		return false
	}
}

func equalsStringValue(s string) equalsValue {
	return func(value interface{}) bool {
		if sValue, err := util.ExtractString(value); err == nil {
			return sValue == s
		}
		logger.Warnf("expected string but got type %T in equals condition.", value)
		return false
	}
}

func equalsBoolValue(b bool) equalsValue {
	return func(value interface{}) bool {
		if sValue, err := util.ExtractBool(value); err == nil {
			return sValue == b
		}
		logger.Warnf("expected bool but got type %T in equals condition.", value)
		return false
	}
}

// NewEqualsCondition builds a new Equals using the given configuration of string equality checks.
func NewEqualsCondition(fields map[string]interface{}) (c Equals, err error) {
	c = Equals{}

	for field, value := range fields {
		uintValue, err := util.ExtractInt(value)
		if err == nil {
			c[field] = equalsIntValue(uintValue)
			continue
		}

		ufloatValue, err := util.ExtractFloat(value)
		if err == nil {
			c[field] = equalsFloatValue(ufloatValue)
			continue
		}

		sValue, err := util.ExtractString(value)
		if err == nil {
			c[field] = equalsStringValue(sValue)
			continue
		}

		bValue, err := util.ExtractBool(value)
		if err == nil {
			c[field] = equalsBoolValue(bValue)
			continue
		}

		return nil, fmt.Errorf("condition attempted to set '%v' -> '%v' and encountered unexpected type '%T', only strings, ints, and booleans are allowed", field, value, value)
	}

	return c, nil
}

// Check determines whether the given event matches this condition.
func (c Equals) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	for field, equalValue := range c {
		value, err := event.GetValue(field)
		if err != nil {
			if isDebug {
				logger.Warnf("'%s' does not exist: %s", field, err)
			}
			return false
		}

		if !equalValue(value) {
			if isDebug {
				logger.Warnf("'%s' is not equal to expected value: %v", field, value)
			}
			return false
		}
	}

	return true
}

func (c Equals) String() string {
	return fmt.Sprintf("equals: %v", map[string]equalsValue(c))
}

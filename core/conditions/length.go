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
	logger "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"reflect"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type LengthEquals map[string]LengthEqualsValue

type LengthEqualsValue func(interface{}) bool

func ArrayLengthEqualsIntValue(i int64) LengthEqualsValue {
	return func(value interface{}) bool {
		if sValue, err := util.ExtractInt(value); err == nil {
			return sValue == i
		}
		logger.Warnf("expected int but got type %T in LengthEquals condition.", value)
		return false
	}
}

func NewArrayLengthEqualsCondition(fields map[string]interface{}) (c LengthEquals, err error) {
	c = LengthEquals{}

	for field, value := range fields {
		uintValue, err := util.ExtractInt(value)
		if err == nil {
			c[field] = ArrayLengthEqualsIntValue(uintValue)
			continue
		}

		return nil, fmt.Errorf("condition attempted to set '%v' -> '%v' and encountered unexpected type '%T', only ints are allowed", field, value, value)
	}

	return c, nil
}

func getLength(data interface{}) int {
	v := reflect.ValueOf(data)

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		// Slices and arrays have a length
		return v.Len()
	case reflect.String:
		// Strings have a length (number of characters)
		return v.Len()
	case reflect.Map:
		// Maps have a length (number of keys)
		return v.Len()
	case reflect.Chan:
		// Channels have a length (number of queued elements)
		return v.Len()
	default:
		// For other types, return -1 to indicate no meaningful length
		return -1
	}
}

// Check determines whether the given event matches this condition.
func (c LengthEquals) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	for field, equalValue := range c {
		value, err := event.GetValue(field)
		if err != nil {
			if isDebug {
				logger.Warnf("'%s' does not exist: %s", field, err)
			}
			return false
		}

		length:=getLength(value)
		if length>0{
			if !equalValue(length) {
				if isDebug {
					logger.Warnf("'%s' is not equal to expected value, actual: %v", field, value)
				}
				return false
			}
		}else{
			if isDebug {
				panic(errors.Errorf("field [%v] not supported to get length, value: %v", field, value))
			}
			return false
		}
	}

	return true
}

func (c LengthEquals) String() string {
	return fmt.Sprintf("LengthEquals: %v", map[string]LengthEqualsValue(c))
}

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
	"errors"
	"fmt"

	logger "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type InArray struct {
	Field string
	Data []interface{}
}

func NewInArrayCondition(fields map[string]interface{}) (c InArray, err error) {
	c = InArray{}

	if len(fields)>0{
		for field, value := range util.MapStr(fields).Flatten() {
			c.Field=field
			c.Data=value.([]interface{})
		}
	}else{
		return c, errors.New("invalid in parameters")
	}

	return c, nil
}

func (c InArray) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	value, err := event.GetValue(c.Field)

	if err != nil {
		if isDebug {
			logger.Warnf("'%s' does not exist: %s", c.Field, err)
		}
		return false
	}

	if util.ContainsAnyInAnyIntArray(value, c.Data) {
		return true
	} else {
		if isDebug {
			logger.Warnf("'%s' does not contain expected value: %v", c.Field, value)
		}
		return false
	}
}

func (c InArray) Name() string {
	return "in"
}

func (c InArray) String() string {
	return fmt.Sprintf("in: %v %v", c.Field,c.Data)
}

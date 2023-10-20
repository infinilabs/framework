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

// Exists is a Condition for checking field existence.
type Exists []string

// NewExistsCondition builds a new Exists checking the given list of fields.
func NewExistsCondition(fields []string) (hasFieldsCondition Exists) {
	return Exists(fields)
}

// Check determines whether the given event matches this condition
func (c Exists) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	for _, field := range c {
		v, err := event.GetValue(field)
		if err != nil {
			if isDebug {
				logger.Warnf("'%s' does not exist: %s", field, err)
			}
			return false
		}
		if util.ToString(v) != "" {
			continue
		}
		if isDebug {
			logger.Warnf("'%s' is empty", field)
		}
		return false
	}
	return true
}

func (c Exists) String() string {
	return fmt.Sprintf("exists: %v", []string(c))
}

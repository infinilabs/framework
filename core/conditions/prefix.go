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
	"infini.sh/framework/core/util"
)

// Prefix is a Condition for checking field is prefix with some specify string.
type Prefix struct {
	Field string
	Data string
}

func NewPrefixCondition(fields map[string]interface{}) (hasFieldsCondition Prefix, err error) {
	c:= Prefix{}
	if len(fields)==1{
		for field, value := range util.MapStr(fields).Flatten() {
			c.Field=field
			var ok bool
			c.Data,ok=value.(string)
			if !ok{
				return c, errors.New("invalid in parameters")
			}
			break
		}
	}else{
		return c, errors.New("invalid in parameters")
	}
	return c,nil
}

// Check determines whether the given event matches this condition
func (c Prefix) Check(event ValuesMap) bool {
	value, err := event.GetValue(c.Field)
	if err != nil {
		return false
	}
	str,ok:=value.(string)
	if ok{
		if util.PrefixStr(str,c.Data){
			return true
		}
	}
	return false
}

func (c Prefix) String() string {
	return fmt.Sprintf("field: %v prefix: %v", c.Field,c.Field)
}

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

// Equals is a Condition for testing string equality.
type InArray struct {
	Field string
	Data []interface{}
}

// NewEqualsCondition builds a new Equals using the given configuration of string equality checks.
func NewInArrayCondition(fields map[string]interface{}) (c InArray, err error) {
	c = InArray{}

	if len(fields)>0{
		for field, value := range util.MapStr(fields).Flatten() {
			//fmt.Println(field,value)
			c.Field=field
			c.Data=value.([]interface{})
			//fmt.Println(c.Data)
		}
			//for field, _ := range fields {
			//c.Field=field
			////v,ok:=value.([]interface{})
			////fmt.Println(v,ok)
			//v,ok:=p.GetArray(param.ParaKey(field))
			//fmt.Println(v,ok)
			//if ok{
			//	//c.Data=v
			//}
		//}
	}else{
		return c, errors.New("invalid in parameters")
	}

	return c, nil
}

func (c InArray) Check(event ValuesMap) bool {

	//fmt.Println("check in array,",c.Field)

	value, err := event.GetValue(c.Field)
	if err != nil {
		//fmt.Println("no field found in event")
		return false
	}

	//fmt.Println("checking data:",value," vs ",c.Data)

	if util.ContainsAnyInAnyIntArray(value,c.Data){
		//fmt.Println("event data in targets,",value,",",c.Data)
		return true
	}else{
		//fmt.Println("event data not in targets,",value,",",c.Data)
		return false
	}
}

func (c InArray) Name() string {
	return "in"
}

func (c InArray) String() string {
	return fmt.Sprintf("in: %v %v", c.Field,c.Data)
}

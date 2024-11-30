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

/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFlatten(t *testing.T) {
	o := map[string]interface{}{}
	v := map[string]interface{}{}
	o["key"] = v
	v["key1"] = map[string]interface{}{}

	v2 := map[string]interface{}{}
	v["key2"] = v2

	v2["key2"] = "val1"
	v2["key3"] = "val2"
	v2["key4"] = "val3"

	v3 := map[string]interface{}{}
	v3["key2"] = v3
	v3["key5"] = "val4"

	fmt.Println(o)
	for i, v := range o {
		fmt.Println(i, v)
	}

	fmt.Println(Flatten(o, false))
	for i, v := range Flatten(o, false) {
		fmt.Println(i, v)
	}
	assert.Equal(t, "val1", Flatten(o, false)["key.key2.key2"])

	js := struct {
		Name     string `json:"name"`
		Age      int
		Addr     string
		Location struct {
			Lat string
			Lon string
		}
	}{Name: "medcl", Addr: "Internet", Age: 8, Location: struct {
		Lat string
		Lon string
	}{Lat: "123", Lon: "123123"}}

	x := FlattenPrefixed(js, "my", false)
	for i, v := range x {
		fmt.Println(i, v)
	}

	assert.Equal(t, "medcl", x["my.Name"])
	assert.Equal(t, 8, x["my.Age"])
	assert.Equal(t, "Internet", x["my.Addr"])
	assert.Equal(t, "123", x["my.Location.Lat"])
	assert.Equal(t, "123123", x["my.Location.Lon"])

	json := `{
		  "key": {
		    "key2": [
		      "top",
		      "bottom"
		    ]
		  },
		  "outer2": 123.234,
		  "outer1": "myvalue"
		}`

	flat, _ := FlattenJSONString(json, "", false)
	fmt.Println(flat)

}

func TestFlattenMap(t *testing.T) {
	m:=MapStr{}
	m["a"]= MapStr {
		"abc":int64(153),
	}

	o:=Flatten(m,false)
	fmt.Println(o)

	result := map[string]interface{}{}
	result["key"]=123
	o=Flatten(result,false)
	fmt.Println(o)
}

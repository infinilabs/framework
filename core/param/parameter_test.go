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

package param

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

type MyConfig struct {
	Name string `config:"name"`
	Age int `config:"age"`
	Bio struct{
		Address string `config:"addr"`
	}  `config:"bio"`
	KV map[string]interface{}`config:"kv"`
	Tags []string `config:"tags"`
	Codes []int `config:"code"`
}

func TestUnpackConfig(t *testing.T) {
	para:=Parameters{}
	data:=map[string]interface{}{}
	data["name"]="medcl"
	data["age"]=123
	bio:=map[string]interface{}{}
	bio["addr"]="China"
	data["bio"]=bio


	kv:=map[string]interface{}{}
	kv["ID"]="12345"
	data["kv"]=kv

	data["tags"]=[]string{"golang","ES"}

	para.Set("config",data)

	obj:=MyConfig{}
	para.Config("config",&obj)
	assert.Equal(t,obj.Name,"medcl")
	assert.Equal(t,obj.Age,123)
	assert.Equal(t,obj.Bio.Address,"China")
	assert.Equal(t,obj.KV["ID"],"12345")
	assert.Equal(t,obj.Tags,[]string{"golang","ES"})
	fmt.Println(obj)

}


func TestGetNestedKey(t *testing.T) {
	para:=Parameters{}
	data:=map[string]interface{}{}

	province:=map[string]interface{}{}
	city:=map[string]interface{}{}
	city["gdp"]=100

	province["changsha"]=city
	data["hunan"]=province
	para.Set("config",data)

	fmt.Println(para)

	v:=para.Get("config.hunan.changsha.gdp")
	fmt.Println(v)
	assert.Equal(t,v,100)

	v1:=para.Get("config.hunan.changsha")
	fmt.Println(v1)


	v2:=para.Get("config.hunan")
	fmt.Println(v2)
}

type SimpleConfig struct {
	Tags []string `config:"tags"`
	Codes []int `config:"code"`
}
func TestGetStringArray(t *testing.T) {
	para:=Parameters{}
	data:=map[string]interface{}{}
	data["tags"]=[]string{"hello","world"}
	data["code"]=[]int{1,2,3}
	para.Set("config",data)

	obj:=SimpleConfig{}
	para.Config("config",&obj)
	fmt.Println(obj.Tags)
	fmt.Println(obj.Codes)

	v,ok:=para.GetStringArray("config.tags")
	fmt.Println(v,ok)
	v1,ok:=para.GetInt64Array("config.code")
	fmt.Println(v1,ok)

}
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

package orm

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"infini.sh/framework/core/util"
	"reflect"
	"testing"
	"time"
)

func TestGetFieldStringValue(t *testing.T) {

	obj := &ORMObjectBase{ID: "myid"}

	t1 := reflect.TypeOf(obj)
	fmt.Println(t1.Kind() == reflect.Ptr)
	assert.Equal(t, t1.Kind(), reflect.Ptr)

	v := reflect.ValueOf(obj).Elem()
	exists, value := getFieldStringValue(v, "ID")
	fmt.Println(exists, value)
	assert.Equal(t, value, "myid")

}

type Obj1 struct {
	ORMObjectBase
	T time.Time
}

func TestCheckCreated(t *testing.T) {

	t1 := time.Now()
	obj := &Obj1{}
	obj.Updated = &t1

	rValue := reflect.ValueOf(obj)
	ok := existsNonNullField(rValue, "Updated")
	assert.Equal(t, true, ok)

	ok = existsNonNullField(rValue, "Created")
	assert.Equal(t, false, ok)

	ok = existsNonNullField(rValue, "T")
	assert.Equal(t, true, ok)
}

type A struct {
	ORMObjectBase
	MyID string
}

func TestGetNestedFieldStringValue(t *testing.T) {

	obj := &A{}
	obj.ID = "myid"

	t1 := reflect.TypeOf(obj)
	fmt.Println(t1.Kind() == reflect.Ptr)

	assert.Equal(t, t1.Kind(), reflect.Ptr)

	v := reflect.ValueOf(obj).Elem()
	exists, value := getFieldStringValue(v, "ID")
	fmt.Println(exists, value)
	assert.Equal(t, value, "myid")
}

func TestSetFieldValue(t *testing.T) {

	obj := &ORMObjectBase{}

	rValue := reflect.ValueOf(obj)
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		setFieldValue(rValue, "ID", "myid1234")
	}

	fmt.Println(obj)
	assert.Equal(t, obj.ID, "myid1234")

}

func TestSetFieldTimeValue(t *testing.T) {

	obj := &ORMObjectBase{}
	rValue := reflect.ValueOf(obj)

	t1 := time.Now()
	setFieldValue(rValue, "Created", &t1)
	fmt.Println("created:", obj.Created)
	assert.Equal(t, obj.Created, &t1)

}

//func TestSetFieldTimeValue1(t *testing.T) {
//	t1:=time.Now()
//	a:=struct {
//		T time.Time
//	}{}
//	rValue := reflect.ValueOf(a)
//	setFieldValue(rValue,"T",t1)
//	fmt.Println("created:",a.T)
//	assert.Equal(t,a.T,t1)
//}

func TestFilterUpdatableFields(t *testing.T) {
	obj := struct {
		Name    string `json:"name" protected:"true"`
		Age     int    `json:"age"`
		Address *struct {
			PostCode string `json:"post_code" protected:"true"`
			Detail   string `json:"detail"`
		} `json:"address"`
		Email   string
		TestNil *struct {
			Field1 string `json:"field1"`
		} `json:"test_nil"`
	}{
		Name: "zhangsan",
		Age:  20,
		Address: &struct {
			PostCode string `json:"post_code" protected:"true"`
			Detail   string `json:"detail"`
		}{
			PostCode: "100001",
			Detail:   "北京海淀",
		},
		Email: "xxx",
	}
	fields := FilterFieldsByProtected(obj, false)
	assert.Equal(t, fields["name"], nil)
	assert.Equal(t, fields["Email"], "xxx")
	assert.Equal(t, fields["age"], float64(20))
	_, exists := util.GetMapValueByKeys([]string{"address", "post_code"}, fields)
	assert.Equal(t, exists, false)
	v, _ := util.GetMapValueByKeys([]string{"address", "detail"}, fields)
	assert.Equal(t, v, "北京海淀")
	var nilM map[string]interface{}
	assert.Equal(t, fields["test_nil"], nilM)
}

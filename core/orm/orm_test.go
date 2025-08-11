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

type TestObject struct {
	ORMObjectBase
}

func TestInjectSystemField(t *testing.T) {
	obj := &TestObject{}

	err := InjectSystemField(obj, "tenant_id", "t-001")
	if err != nil {
		t.Fatalf("InjectSystemField failed: %v", err)
	}

	if obj.System == nil {
		t.Fatalf("System map not initialized")
	}

	val, ok := obj.System["tenant_id"]
	if !ok {
		t.Fatalf("tenant_id key not found in System")
	}
	if val != "t-001" {
		t.Errorf("expected 't-001', got %v", val)
	}
}

func TestInjectSystemFields(t *testing.T) {
	obj := &TestObject{}

	fields := map[string]interface{}{
		"tenant_id": "t-002",
		"user_id":   "u-123",
		"role":      "admin",
	}
	err := InjectSystemFields(obj, fields)
	if err != nil {
		t.Fatalf("InjectSystemFields failed: %v", err)
	}

	for k, expected := range fields {
		got, ok := obj.System[k]
		if !ok {
			t.Errorf("missing key %s in System", k)
			continue
		}
		if got != expected {
			t.Errorf("key %s: expected %v, got %v", k, expected, got)
		}
	}
}

func TestInjectSystemField_NonStruct(t *testing.T) {
	notStruct := "hello"
	err := InjectSystemField(notStruct, "key", "value")
	if err == nil {
		t.Errorf("expected error for non-struct, got nil")
	}
}

func TestInjectSystemField_NilPointer(t *testing.T) {
	var nilObj *TestObject
	err := InjectSystemField(nilObj, "key", "value")
	if err == nil {
		t.Errorf("expected error for nil pointer, got nil")
	}
}

func TestGetSystemString(t *testing.T) {
	obj := &ORMObjectBase{
		System: util.MapStr{
			"tenant_id": "tenant-123",
			"user_id":   "user-abc",
		},
	}

	if got := obj.GetSystemString("tenant_id"); got != "tenant-123" {
		t.Errorf("expected tenant_id 'tenant-123', got %v", got)
	}
	if got := obj.GetSystemString("missing_key"); got != "" {
		t.Errorf("expected empty string for missing key, got %v", got)
	}
}

func TestGetSystemBool(t *testing.T) {
	obj := &ORMObjectBase{
		System: util.MapStr{
			"is_admin": true,
			"disabled": false,
		},
	}

	if !obj.GetSystemBool("is_admin") {
		t.Errorf("expected is_admin to be true")
	}
	if obj.GetSystemBool("disabled") {
		t.Errorf("expected disabled to be false")
	}
	if obj.GetSystemBool("not_exist") {
		t.Errorf("expected not_exist to be false")
	}
}

func TestGetSystemInt(t *testing.T) {
	obj := &ORMObjectBase{
		System: util.MapStr{
			"quota":     100,
			"usage":     int64(200),
			"threshold": float64(300),
		},
	}

	if got := obj.GetSystemInt("quota"); got != 100 {
		t.Errorf("expected quota 100, got %v", got)
	}
	if got := obj.GetSystemInt("usage"); got != 200 {
		t.Errorf("expected usage 200, got %v", got)
	}
	if got := obj.GetSystemInt("threshold"); got != 300 {
		t.Errorf("expected threshold 300, got %v", got)
	}
	if got := obj.GetSystemInt("missing"); got != 0 {
		t.Errorf("expected missing to return 0, got %v", got)
	}
}

type Base struct {
	ID      string
	Created *time.Time
}

type Mid struct {
	Base
	System string
}

type Top struct {
	Mid
	Updated *time.Time
}

// Helper to get *time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestFindFieldValue_Simple(t *testing.T) {
	created := timePtr(time.Now())
	obj := Base{ID: "simple123", Created: created}

	v := findFieldValue(reflect.ValueOf(obj), "ID")
	if !v.IsValid() || v.String() != "simple123" {
		t.Errorf("expected ID to be 'simple123', got %v", v)
	}

	v = findFieldValue(reflect.ValueOf(obj), "Created")
	if !v.IsValid() || v.Interface().(*time.Time) != created {
		t.Errorf("expected Created to match pointer, got %v", v)
	}
}

func TestFindFieldValue_Embedded(t *testing.T) {
	obj := Mid{
		Base:   Base{ID: "embed123"},
		System: "secure",
	}

	v := findFieldValue(reflect.ValueOf(obj), "ID")
	if !v.IsValid() || v.String() != "embed123" {
		t.Errorf("expected ID to be 'embed123', got %v", v)
	}
}

func TestFindFieldValue_DeepEmbedded(t *testing.T) {
	t1 := timePtr(time.Now())
	obj := Top{
		Mid: Mid{
			Base:   Base{ID: "deep123", Created: t1},
			System: "secure",
		},
		Updated: timePtr(time.Now()),
	}

	v := findFieldValue(reflect.ValueOf(obj), "ID")
	if !v.IsValid() || v.String() != "deep123" {
		t.Errorf("expected ID to be 'deep123', got %v", v)
	}

	v = findFieldValue(reflect.ValueOf(obj), "Created")
	if !v.IsValid() || v.Interface().(*time.Time) != t1 {
		t.Errorf("expected Created to match pointer, got %v", v)
	}
}

func TestCopySystemFields(t *testing.T) {
	t1 := timePtr(time.Now().Add(-time.Hour))
	t2 := timePtr(time.Now())

	src := Top{
		Mid: Mid{
			Base:   Base{ID: "copy123", Created: t1},
			System: "locked",
		},
		Updated: t2,
	}

	dst := Top{}
	copySystemFields(&src, &dst)

	if dst.ID != "copy123" {
		t.Errorf("expected ID to be 'copy123', got %v", dst.ID)
	}
	if dst.Created != t1 {
		t.Errorf("expected Created to match t1, got %v", dst.Created)
	}
	if dst.System != "locked" {
		t.Errorf("expected System to be 'locked', got %v", dst.System)
	}
	if dst.Updated != t2 {
		t.Errorf("expected Updated to match t2, got %v", dst.Updated)
	}
}

func TestFindFieldValue_NilPointerEmbedded(t *testing.T) {
	var nilBase *Base
	obj := struct {
		*Base
	}{
		Base: nilBase,
	}

	v := findFieldValue(reflect.ValueOf(obj), "ID")
	if v.IsValid() {
		t.Errorf("expected invalid value when embedded pointer is nil, got %v", v)
	}
}

type Inner struct {
	Name string
	Age  int
}

type Outer struct {
	ID      string
	Active  bool
	Count   int
	Created *time.Time
	Inner   Inner
	Ptr     *Inner
}

func TestMergeMapToStruct_SimpleFields(t *testing.T) {
	obj := Outer{
		ID:     "oldID",
		Active: false,
		Count:  5,
	}

	delta := map[string]interface{}{
		"ID":     "newID",
		"Active": true,
		"Count":  10,
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.ID != "newID" {
		t.Errorf("expected ID 'newID', got %s", obj.ID)
	}
	if obj.Active != true {
		t.Errorf("expected Active true, got %v", obj.Active)
	}
	if obj.Count != 10 {
		t.Errorf("expected Count 10, got %d", obj.Count)
	}
}

func TestMergeMapToStruct_NestedStruct(t *testing.T) {
	obj := Outer{
		Inner: Inner{Name: "old", Age: 20},
	}

	delta := map[string]interface{}{
		"Inner": map[string]interface{}{
			"Name": "new",
			"Age":  30,
		},
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.Inner.Name != "new" {
		t.Errorf("expected Inner.Name 'new', got %s", obj.Inner.Name)
	}
	if obj.Inner.Age != 30 {
		t.Errorf("expected Inner.Age 30, got %d", obj.Inner.Age)
	}
}

func TestMergeMapToStruct_PtrToStruct(t *testing.T) {
	obj := Outer{}

	delta := map[string]interface{}{
		"Ptr": map[string]interface{}{
			"Name": "pointer",
			"Age":  42,
		},
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.Ptr == nil {
		t.Fatal("expected Ptr to be allocated")
	}
	if obj.Ptr.Name != "pointer" {
		t.Errorf("expected Ptr.Name 'pointer', got %s", obj.Ptr.Name)
	}
	if obj.Ptr.Age != 42 {
		t.Errorf("expected Ptr.Age 42, got %d", obj.Ptr.Age)
	}
}

func TestMergeMapToStruct_NilValueClearsField(t *testing.T) {
	created := time.Now()
	obj := Outer{
		Created: &created,
	}

	delta := map[string]interface{}{
		"Created": nil,
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.Created != nil {
		t.Errorf("expected Created to be nil, got %v", obj.Created)
	}
}

func TestMergeMapToStruct_UnexportedOrUnknownFields(t *testing.T) {
	obj := Outer{}

	delta := map[string]interface{}{
		"unknownField": "should be ignored",
		"id":           "caseSensitiveIgnored",
		"ID":           "setCorrectly",
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.ID != "setCorrectly" {
		t.Errorf("expected ID 'setCorrectly', got %s", obj.ID)
	}
}

func TestMergeMapToStruct_TypeConversion(t *testing.T) {
	obj := Outer{}

	delta := map[string]interface{}{
		"Count":  int64(123), // int64 to int
		"Active": true,
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj).Elem())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.Count != 123 {
		t.Errorf("expected Count 123, got %d", obj.Count)
	}
	if obj.Active != true {
		t.Errorf("expected Active true, got %v", obj.Active)
	}
}

type TestEmbeddedStruct struct {
	ORMObjectBase

	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func TestMergeMapToStruct_WithEmbeddedORMBase(t *testing.T) {
	created := time.Now()
	obj := TestEmbeddedStruct{
		ORMObjectBase: ORMObjectBase{
			ID:      "original-id",
			Created: &created,
			System:  util.MapStr{"os": "linux"},
		},
		Name: "old-name",
	}

	delta := map[string]interface{}{
		"id":   "new-id", // matches json tag
		"name": "new-name",
		"_system": map[string]interface{}{ // matches json:"_system"
			"os": "windows",
		},
	}

	err := mergeMapToStruct(delta, reflect.ValueOf(&obj))
	assert.Equal(t, nil, err)

	assert.Equal(t, "new-id", obj.ID)            // ID updated
	assert.Equal(t, "new-name", obj.Name)        // Name updated
	assert.Equal(t, "windows", obj.System["os"]) // System updated
	assert.Equal(t, &created, obj.Created)       // Created preserved
}

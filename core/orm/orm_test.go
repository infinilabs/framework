package orm

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"reflect"
	"testing"
	"time"
)

func TestGetFieldStringValue(t *testing.T) {

	obj := &ORMObjectBase{ID: "myid"}

	t1 := reflect.TypeOf(obj)
	fmt.Println(t1.Kind() == reflect.Ptr)

	v := reflect.ValueOf(obj).Elem()
	exists, value := getFieldStringValue(v, "ID")
	fmt.Println(exists, value)

}
func TestSetFieldValue(t *testing.T) {

	obj := &ORMObjectBase{}

	rValue := reflect.ValueOf(obj)
	idExists, _ := getFieldStringValue(rValue, "ID")
	if !idExists {
		setFieldValue(rValue, "ID", "myid1234")
	}

	fmt.Println(obj)
	assert.Equal(t,obj.ID,"myid1234")

}

func TestSetFieldTimeValue(t *testing.T) {

	obj := &ORMObjectBase{}
	rValue := reflect.ValueOf(obj)

	t1:=time.Now()
	setFieldValue(rValue,"Created",&t1)
	fmt.Println("created:",obj.Created)
	assert.Equal(t,obj.Created,&t1)

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

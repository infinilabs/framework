package orm

import (
	"fmt"
	"infini.sh/framework/core/util"
	"reflect"
	"testing"
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
		setFieldValue(rValue, "ID", util.GetUUID())
	}

	fmt.Println(obj)

}

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

func TestFilterUpdatableFields(t *testing.T) {
	obj := struct {
		Name string `json:"name" protected:"true"`
		Age int `json:"age"`
		Address *struct{
			PostCode string `json:"post_code" protected:"true"`
			Detail string `json:"detail"`
		} `json:"address"`
		Email string
		TestNil *struct{
			Field1 string `json:"field1"`
		} `json:"test_nil"`
	}{
		Name: "zhangsan",
		Age: 20,
		Address: &struct {
			PostCode string `json:"post_code" protected:"true"`
			Detail string `json:"detail"`
		}{
			PostCode: "100001",
			Detail: "北京海淀",
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
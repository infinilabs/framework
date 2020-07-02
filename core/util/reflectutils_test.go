package util

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestCloneValue(t *testing.T) {
	data := map[string]interface{}{}
	v := map[string]interface{}{}
	v["key"] = "value"
	data["name"] = "name-1"
	data["data"] = v

	type T struct {
		Name string `json:"name"`
		Data struct {
			Key string `json:"key"`
		}
	}

	js := ToJson(data, false)
	o := T{}
	FromJson(js, &o)

	assert.Equal(t, "name-1", o.Name)
	assert.Equal(t, "value", o.Data.Key)
}

func TestGetTag(t *testing.T) {

	type S struct {
		F string `species:"gopher" color:"blue"`
	}

	ts := S{F: "test F"}
	st := reflect.TypeOf(ts)

	field := st.Field(0)
	fmt.Println(field.Tag.Get("color"), field.Tag.Get("species"))

	fmt.Println(field.Name)

	fmt.Println(reflect.Indirect(reflect.ValueOf(ts)).FieldByName(field.Name).String())

	v := GetFieldValueByTagName(ts, "color", "blue")
	fmt.Println(v)
	assert.Equal(t, v, "test F")

	vs := &S{"flower"}

	fmt.Println(reflect.TypeOf(vs))
	fmt.Println(reflect.ValueOf(vs))
	fmt.Println(reflect.Indirect(reflect.ValueOf(vs)).Type().Name())

	se := reflect.TypeOf(vs).Elem()
	for i := 0; i < se.NumField(); i++ {
		fmt.Println(se.Field(i).Name)
		fmt.Println(se.Field(i).Type)
		fmt.Println(se.Field(i).Tag)
	}

	v1 := GetFieldValueByTagName(vs, "color", "blue")
	fmt.Println(v1)
	assert.Equal(t, v1, "flower")

	fmt.Println(reflect.TypeOf(ts))
	fmt.Println(reflect.TypeOf(vs))
	fmt.Println(reflect.ValueOf(ts))
	fmt.Println(reflect.ValueOf(vs))

	assert.Equal(t, GetTypeName(ts, false), "S")
	assert.Equal(t, GetTypeName(vs, false), "S")

}

func TestGetTags(t *testing.T) {
	type N struct {
		S  string `index:"in"`
		S1 string `index:"in2"`
	}

	type S struct {
		F  string   `index:"out"`
		N1 N        `index:"out-1"`
		S1 []string `index:"out[]"`
		N2 []N      `index:"out-2"`
	}

	ts := S{F: "out", N1: N{}}

	v1 := GetTagsByTagName(ts, "index")
	fmt.Println(ToJson(v1, true))

	assert.Equal(t, "out", v1[0].Tag)
	assert.Equal(t, "out-1", v1[1].Tag)
	assert.Equal(t, "in", v1[1].Annotation[0].Tag)
	assert.Equal(t, "in2", v1[1].Annotation[1].Tag)
	assert.Equal(t, "out[]", v1[2].Tag)
	assert.Equal(t, "out-2", v1[3].Tag)
	assert.Equal(t, "in", v1[3].Annotation[0].Tag)
	assert.Equal(t, "in2", v1[3].Annotation[1].Tag)

}

func TestGetStructPointerTags(t *testing.T) {
	type N struct {
		S string `index:"in"`
	}

	type S struct {
		N3 *N `index:"out"`
	}

	ts := S{}

	v1 := GetTagsByTagName(ts, "index")
	fmt.Println(ToJson(v1, true))

	assert.Equal(t, "out", v1[0].Tag)
	assert.Equal(t, "in", v1[0].Annotation[0].Tag)
}

func TestCopy(t *testing.T) {
	type X struct {
		Z string
	}

	type N struct {
		S  string `index:"in"`
		S1 string `index:"in2"`
		X  X
	}

	type S struct {
		F  string   `index:"out"`
		N1 N        `index:"out-1"`
		S1 []string `index:"out[]"`
		N2 []N      `index:"out-2"`
	}

	x := S{F: "out", N1: N{"1", "2", X{"11"}}, N2: []N{{"2", "3", X{"12"}}, {"4", "5", X{"13"}}}}
	y := S{}

	Copy(x, &y)

	assert.Equal(t, "out", y.F)
	assert.Equal(t, "1", y.N1.S)
	assert.Equal(t, "2", y.N1.S1)
	assert.Equal(t, "2", y.N2[0].S)
	assert.Equal(t, "3", y.N2[0].S1)
	assert.Equal(t, "4", y.N2[1].S)
	assert.Equal(t, "5", y.N2[1].S1)
	assert.Equal(t, "13", y.N2[1].X.Z)

	fmt.Println(y)
}

package param

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

type MyConfig struct {
	Name string `config:"name"`
	Age int `config:"age"`
}

func TestUnpackConfig(t *testing.T) {
	para:=Parameters{}
	data:=map[string]interface{}{}
	data["name"]="medcl"
	data["age"]=123
	para.Set("config",data)

	fmt.Println(para)

	obj:=MyConfig{}
	para.Config("config",&obj)
	fmt.Println(obj.Name)
	fmt.Println(obj.Age)
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

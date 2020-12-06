package param

import (
	"fmt"
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

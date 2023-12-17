package elastic

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"infini.sh/framework/core/util"
	"testing"
)

type Obj struct {
	Id string `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
}

type Nest struct {
	Obj
	Name string `json:"name,omitempty"`
}

type DeepNest struct {
	Nest
	Desc string `json:"desc,omitempty"`
}

func TestGetIndexID(t *testing.T) {
	o := Obj{Id: "my_id"}
	id := getIndexID(&o)
	fmt.Println(id)
	assert.Equal(t,id,"my_id")

	o1 := Nest{}
	o1.Id = "myid2"
	//o1.Obj.Id = "myid2"
	id = getIndexID(o)
	fmt.Println(id)
	assert.Equal(t,id,"my_id")

	tag := util.GetFieldValueByTagName(&o, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t,tag,"my_id")

	id1 := getIndexID(o1)
	fmt.Println(id1)
	assert.Equal(t,id1,"myid2")

	tag1 := util.GetFieldValueByTagName(&o1, "json", "id")
	fmt.Println(tag1)
	assert.Equal(t,tag1,"myid2")

	tag = util.GetFieldValueByTagName(&o1, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t,tag,"myid2")


}

func TestGetDeepNesteIndexID(t *testing.T) {

	o2:=DeepNest{}
	o2.Id="myid3"

	tag := util.GetFieldValueByTagName(&o2, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t,tag,"myid3")
}

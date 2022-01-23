package elastic

import (
	"fmt"
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

func TestGetIndexID(t *testing.T) {
	o := Obj{Id: "my_id"}
	id := getIndexID(&o)
	fmt.Println(id)

	o1 := Nest{}
	o1.Id = "myid2"
	o1.Obj.Id = "myid2"
	id = getIndexID(o)
	fmt.Println(id)

	tag := util.GetFieldValueByTagName(&o, "elastic_meta", "_id")
	fmt.Println(tag)

	id1 := getIndexID(o1)
	fmt.Println(id1)

	tag1 := util.GetFieldValueByTagName(&o1, "json", "id")
	fmt.Println(tag1)

	tag = util.GetFieldValueByTagName(&o1, "elastic_meta", "_id")
	fmt.Println(tag)
}

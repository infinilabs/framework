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
	assert.Equal(t, id, "my_id")

	o1 := Nest{}
	o1.Id = "myid2"
	//o1.Obj.Id = "myid2"
	id = getIndexID(o)
	fmt.Println(id)
	assert.Equal(t, id, "my_id")

	tag := util.GetFieldValueByTagName(&o, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t, tag, "my_id")

	id1 := getIndexID(o1)
	fmt.Println(id1)
	assert.Equal(t, id1, "myid2")

	tag1 := util.GetFieldValueByTagName(&o1, "json", "id")
	fmt.Println(tag1)
	assert.Equal(t, tag1, "myid2")

	tag = util.GetFieldValueByTagName(&o1, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t, tag, "myid2")

}

func TestGetDeepNesteIndexID(t *testing.T) {

	o2 := DeepNest{}
	o2.Id = "myid3"

	tag := util.GetFieldValueByTagName(&o2, "elastic_meta", "_id")
	fmt.Println(tag)
	assert.Equal(t, tag, "myid3")
}

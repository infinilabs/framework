/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestRemoveDotFromIndexName(t *testing.T){
	index:=".abc"
	new:=RemoveDotFromIndexName(index,"#")
	fmt.Println(new)
	assert.Equal(t,new,"#abc")

	index="abc"
	new=RemoveDotFromIndexName(index,"#")
	assert.Equal(t,new,"abc")

	index="abc."
	new=RemoveDotFromIndexName(index,"#")
	assert.Equal(t,new,"abc#")

	index="a.b.c."
	new=RemoveDotFromIndexName(index,"#")
	assert.Equal(t,new,"a.b.c#")

	index="a.b.c"
	new=RemoveDotFromIndexName(index,"#")
	assert.Equal(t,new,"a.b.c")
}

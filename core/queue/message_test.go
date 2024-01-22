/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestParseOffset(t *testing.T) {
	offset:=NewOffset(1,2)
	offset.Version=5
	offsetStr:=offset.String()
	fmt.Println(offsetStr)
	assert.Equal(t,offsetStr,"1,2")

	offsetStr=offset.EncodeToString()
	fmt.Println(offsetStr)
	assert.Equal(t,offsetStr,"1,2,5")
	offset=DecodeFromString(offsetStr)
	fmt.Println(offset)
	assert.Equal(t,offset.Version,int64(5))
	assert.Equal(t,offset.Segment,int64(1))
	assert.Equal(t,offset.Position,int64(2))

	//check backward compatibility
	offsetStr="1,2"
	offset=DecodeFromString(offsetStr)
	fmt.Println(offset)
	assert.Equal(t,offset.Segment,int64(1))
	assert.Equal(t,offset.Position,int64(2))

}

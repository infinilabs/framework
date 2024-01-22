/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestCompareOffset(t *testing.T) {
	o1:=NewOffsetWithVersion(1,2,3)
	o2:=NewOffsetWithVersion(2,2,3)
	o3:=NewOffsetWithVersion(2,3,3)
	o4:=NewOffsetWithVersion(2,3,4)

	assert.Equal(t,o1.LatestThan(o2),false)
	assert.Equal(t,o1.LatestThan(o3),false)
	assert.Equal(t,o1.LatestThan(o4),false)

	assert.Equal(t,o2.LatestThan(o3),false)
	assert.Equal(t,o2.LatestThan(o4),false)

	assert.Equal(t,o3.LatestThan(o4),false)

	o1=NewOffsetWithVersion(0,716114,0)
	o2=NewOffsetWithVersion(0,0,1)

	assert.Equal(t,o1.LatestThan(o2),false)
}

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

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
	o1 := NewOffsetWithVersion(1, 2, 3)
	o2 := NewOffsetWithVersion(2, 2, 3)
	o3 := NewOffsetWithVersion(2, 3, 3)
	o4 := NewOffsetWithVersion(2, 3, 4)

	assert.Equal(t, o1.LatestThan(o2), false)
	assert.Equal(t, o1.LatestThan(o3), false)
	assert.Equal(t, o1.LatestThan(o4), false)

	assert.Equal(t, o2.LatestThan(o3), false)
	assert.Equal(t, o2.LatestThan(o4), false)

	assert.Equal(t, o3.LatestThan(o4), false)

	o1 = NewOffsetWithVersion(0, 716114, 0)
	o2 = NewOffsetWithVersion(0, 0, 1)

	assert.Equal(t, o1.LatestThan(o2), false)

	//32,83253693,00,0,1
	o1 = NewOffsetWithVersion(32, 83253693, 0)
	o2 = NewOffsetWithVersion(0, 0, 1)

	assert.Equal(t, o1.LatestThan(o2), false)
}

func TestParseOffset(t *testing.T) {
	offset := NewOffset(1, 2)
	offset.Version = 5
	offsetStr := offset.String()
	fmt.Println(offsetStr)
	assert.Equal(t, offsetStr, "1,2")

	offsetStr = offset.EncodeToString()
	fmt.Println(offsetStr)
	assert.Equal(t, offsetStr, "1,2,5")
	offset = DecodeFromString(offsetStr)
	fmt.Println(offset)
	assert.Equal(t, offset.Version, int64(5))
	assert.Equal(t, offset.Segment, int64(1))
	assert.Equal(t, offset.Position, int64(2))

	//check backward compatibility
	offsetStr = "1,2"
	offset = DecodeFromString(offsetStr)
	fmt.Println(offset)
	assert.Equal(t, offset.Segment, int64(1))
	assert.Equal(t, offset.Position, int64(2))

}

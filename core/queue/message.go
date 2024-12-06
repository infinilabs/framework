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
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/util"
	"strconv"
	"strings"
	"time"
)

func Itoa64(i int64) string {
	return strconv.FormatInt(i, 10)
}

func AcquireOffset(seg, pos int64) Offset {
	return Offset{Segment: seg, Position: pos}
}

func ConvertOffset(offsetStr string) (int64, int64) {
	var segment, offset int64
	segment, offset, _ = ExtendConvertOffset(offsetStr)
	return segment, offset
}

func ExtendConvertOffset(offsetStr string) (int64, int64, int64) {
	if offsetStr == "" {
		panic(errors.New("offset can't be empty"))
	}

	data := strings.Split(offsetStr, ",")

	//handle old offset format
	var segment, offset, ver int64
	if len(data) == 2 {
		segment, _ = util.ToInt64(data[0])
		offset, _ = util.ToInt64(data[1])
	} else if len(data) == 3 {
		segment, _ = util.ToInt64(data[0])
		offset, _ = util.ToInt64(data[1])
		ver, _ = util.ToInt64(data[2])
	} else {
		panic(errors.Errorf("invalid offset: %v", offsetStr))
	}
	return segment, offset, ver
}

func DecodeFromString(offsetStr string) Offset {
	return NewOffsetWithVersion(ExtendConvertOffset(offsetStr))
}

func NewOffsetWithVersion(seg, pos, ver int64) Offset {
	return Offset{Segment: seg, Position: pos, Version: ver}
}

func NewOffset(seg, pos int64) Offset {
	return Offset{Segment: seg, Position: pos}
}

type Offset struct {
	Segment  int64 `json:"segment"`
	Position int64 `json:"position"`
	Version  int64 `json:"version"`
}

func (c *Offset) Equals(v Offset) bool {
	if c.Segment != v.Segment {
		return false
	}

	if c.Position <= 0 && v.Position <= 0 {
		return true
	}

	if c.Position != v.Position {
		return false
	}
	return true
}

func (c *Offset) LatestThan(v Offset) bool {

	if c.Version > v.Version {
		return true
	}

	if c.Version == v.Version {
		if c.Segment > v.Segment {
			return true
		}

		if c.Segment == v.Segment {
			if c.Position > v.Position {
				return true
			}
		}
	}

	return false
}

func (c *Offset) String() string {
	return fmt.Sprintf("%v,%v", c.Segment, c.Position)
}

func (c *Offset) EncodeToString() string {
	return fmt.Sprintf("%v,%v,%v", c.Segment, c.Position, c.Version)
}

type Context struct {
	MessageCount int    `config:"message_count" json:"message_count"`
	NextOffset   Offset `config:"next_offset" json:"next_offset"`
	InitOffset   Offset `config:"init_offset" json:"init_offset"`
}

func (c *Context) UpdateInitOffset(seg, pos, ver int64) {
	c.InitOffset.Segment = seg
	c.InitOffset.Position = pos
	c.InitOffset.Version = ver
}

func (c *Context) UpdateNextOffset(seg, pos int64) {
	c.NextOffset.Segment = seg
	c.NextOffset.Position = pos
}

func (c *Context) String() string {
	return fmt.Sprintf("%v->%v", c.InitOffset, c.NextOffset)
}

func (c *Context) Valid() bool {
	return c.MessageCount > 0
}

type Message struct {
	Timestamp  int64  `config:"timestamp" json:"timestamp" parquet:"timestamp"`
	Offset     Offset `config:"offset" json:"offset"  parquet:"offset"`                //current offset
	NextOffset Offset `config:"next_offset" json:"next_offset"  parquet:"next_offset"` //offset for next message
	Size       int    `config:"size" json:"size"  parquet:"size"`
	Data       []byte `config:"data" json:"data"  parquet:"data,zstd"`
}

func (m *Message) String() string {
	return fmt.Sprintf("timestamp:%v, offset:%v, next_offset:%v, size:%v, data:%v", time.Unix(0, m.Timestamp), m.Offset.String(), m.NextOffset.String(), m.Size, string(m.Data))
}

type ProduceRequest struct {
	Topic string `config:"topic" json:"topic"` //queue_id
	Key   []byte `config:"key" json:"key"`
	Data  []byte `config:"data" json:"data"`
}

type ProduceResponse struct {
	Topic     string `config:"topic" json:"topic"`
	Partition int64  `config:"partition" json:"partition"`
	Offset    Offset `config:"offset" json:"offset"`
	Timestamp int64  `config:"timestamp" json:"timestamp"`
}

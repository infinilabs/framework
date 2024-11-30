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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/lib/bytebufferpool"
	"sync"
)

type Message struct {
	Payload *bytebufferpool.ByteBuffer
	Context Context
}

type Context struct {
	WriteFile           string             `json:"write_file_path"`
	WriteFileNum        int64             `json:"write_file_num"`
}

type EventType string

const WriteComplete = EventType("WriteComplete")
const ReadComplete = EventType("ReadComplete")

type Event struct {
	Queue   string
	Type    EventType
	FileNum int64
}

type EventHandler func(event Event)error

var handlers =[]EventHandler{}
var locker =sync.RWMutex{}

func RegisterEventListener(handler EventHandler){
	locker.Lock()
	defer locker.Unlock()
	handlers =append(handlers,handler)
}

func Notify(queue string, eventType EventType,fileNum int64)  {

	if global.Env().IsDebug{
		log.Tracef("notify on queue: %v, type: %v, segment: %v",queue,eventType,fileNum)
	}

	event:= Event{
		Queue: queue,
		Type: eventType,
		FileNum: fileNum,
	}

	for _,v:=range handlers {
		v(event)
	}
}


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

package event

import (
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

var meta *AgentMeta

func RegisterMeta(m *AgentMeta) {
	meta = m
}

func getMeta() *AgentMeta {
	if meta == nil {
		meta = &AgentMeta{DefaultMetricQueueName: "metrics", LoggingQueueName: "logging"}
	}
	return meta
}

func UpdateAgentID(agentID string) {
	if meta != nil {
		meta.AgentID = agentID
	}
}

func SaveWithTimestamp(event *Event, time2 time.Time) error {
	if event==nil{
		panic("event can't be nil")
	}

	if global.Env().IsDebug {
		log.Debugf("%v-%v: %v", event.Metadata.Category, event.Metadata.Name, string(util.MustToJSONBytes(event.Metadata)))
	}

	event.Timestamp = time2

	//check event specified queue name
	if  event.QueueName!= "" {
		return queue.Push(queue.GetOrInitConfig(event.QueueName), util.MustToJSONBytes(event))
	}else{
		//check default queue name
		if getMeta().DefaultMetricQueueName == "" {
			panic("queue can't be nil")
		}
		event.QueueName=getMeta().DefaultMetricQueueName;
	}

	if event.Agent==nil{
		event.Agent = getMeta()
	}

	stats.Increment("metrics.save", event.Metadata.Category, event.Metadata.Name)
	return queue.Push(queue.GetOrInitConfig(event.QueueName), util.MustToJSONBytes(event))
}

func Save(event *Event) error {
	return SaveWithTimestamp(event, time.Now())
}

func SaveLog(event *Event) error {
	if event==nil{
		panic("event can't be nil")
	}

	event.Timestamp = time.Now()
	event.Agent = getMeta()

	if getMeta().LoggingQueueName == "" {
		panic("queue can't be nil")
	}

	if global.Env().IsDebug {
		log.Debugf("%v-%v: %v, %v", event.Metadata.Category, event.Metadata.Name, util.MustToJSON(event.Metadata), util.MustToJSON(event.Fields))
	}

	stats.Increment("metrics.savelog", event.Metadata.Category, event.Metadata.Name)

	err := queue.Push(queue.GetOrInitConfig(getMeta().LoggingQueueName), util.MustToJSONBytes(event))
	if err != nil {
		panic(err)
	}

	return nil
}

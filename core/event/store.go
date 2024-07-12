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

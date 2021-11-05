/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package metrics

import (
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"time"
)

var meta *AgentMeta

func RegisterMeta(m *AgentMeta)  {
	meta =m
}

func getMeta()*AgentMeta {
	if meta ==nil{
		meta =&AgentMeta{QueueName: "metrics"}
	}
	return meta
}

func Save(event MetricEvent) error {

	event.Timestamp = time.Now()
	event.Agent= getMeta()

	if getMeta().QueueName==""{
		panic("queue can't be nil")
	}

	stats.Increment("metrics.save",event.Metadata.Category,event.Metadata.Name)

	queue.Push(getMeta().QueueName, util.MustToJSONBytes(event))

	return nil
}

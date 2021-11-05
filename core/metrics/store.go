/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package metrics

import (
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	log "src/github.com/cihub/seelog"
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

	log.Error(event.String())

	if getMeta().QueueName==""{
		panic("queue can't be nil")
	}

	queue.Push(getMeta().QueueName, util.MustToJSONBytes(event))

	return nil
}

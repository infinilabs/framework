/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package common

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/queue"
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

	log.Error(event.String())

	if getMeta().QueueName==""{
		panic("queue can't be nil")
	}

	queue.Push(getMeta().QueueName, util.MustToJSONBytes(event))

	return nil
}

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package event

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
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

func UpdateAgentID(agentID string){
	if meta != nil {
		meta.AgentID = agentID
	}
}

func UpdateHostID(hostID string){
	if meta != nil {
		meta.HostID = hostID
	}
}

func Save(event Event) error {

	event.Timestamp = time.Now()
	event.Agent= getMeta()

	if getMeta().QueueName==""{
		panic("queue can't be nil")
	}

	if global.Env().IsDebug{
		log.Debugf("%v-%v: %v",event.Metadata.Category,event.Metadata.Name,string(util.MustToJSONBytes(event.Metadata)))
	}

	stats.Increment("metrics.save",event.Metadata.Category,event.Metadata.Name)

	err:=queue.Push(queue.GetOrInitConfig(getMeta().QueueName), util.MustToJSONBytes(event))
	if err!=nil{
		panic(err)
	}

	return nil
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/agent/api"
	"time"
)

func (module *AgentModule) Name() string {
	return "agent"
}

func (module *AgentModule) Setup() {
	orm.RegisterSchemaWithIndexName(agent.Instance{}, "agent")
	orm.RegisterSchemaWithIndexName(host.HostInfo{}, "host")
	api.Init()
}
func (module *AgentModule) Start() error {
	agents, err := loadAgentsFromES("")
	if err != nil {
		log.Error(err)
	}
	taskState := map[string]agent.ShortState{}
	agentIds := map[string]struct{}{}
	for _, ag := range agents {
		if !ag.Enrolled {
			continue
		}
		agentIds[ag.ID] = struct{}{}
		for _, cluster := range ag.Clusters {
			if cluster.Task.ClusterMetric.Owner {
				taskState[cluster.ClusterID] = agent.ShortState{
					ClusterMetricTask: agent.ClusterMetricTaskState{
						AgentID:  ag.ID,
						NodeUUID: cluster.Task.ClusterMetric.TaskNodeID,
					},
				}
			}
			if cluster.Task.NodeMetric != nil && cluster.Task.NodeMetric.Owner {
				state := taskState[cluster.ClusterID]
				state.NodeMetricTask = agent.NodeMetricTaskState{
					AgentID: ag.ID,
					Nodes: cluster.Task.NodeMetric.ExtraNodes,
				}
				taskState[cluster.ClusterID] = state
			}
		}
	}

	sm := NewStateManager(time.Second*30, "agent_state", taskState, agentIds)
	agent.RegisterStateManager(sm)
	go sm.LoopState()
	//todo reassign tasks and refresh state automatically
	return nil
}

func (module *AgentModule) Stop() error {
	log.Info("start to stop agent module")
	if agent.IsEnabled(){
		agent.GetStateManager().Stop()
	}
	log.Info("agent module was stopped")
	return nil
}

type AgentModule struct {
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/agent/api"
	"infini.sh/framework/modules/agent/common"
	"time"
)

func (module *AgentModule) Name() string {
	return "agent"
}

func (module *AgentModule) Setup() {
	exists, err := env.ParseConfig("agent", &module.AgentConfig)
	if exists && err != nil {
		panic(err)
	}
	if module.AgentConfig.Enabled {
		api.Init()
	}
}
func (module *AgentModule) Start() error {
	if !module.AgentConfig.Enabled {
		return nil
	}
	orm.RegisterSchemaWithIndexName(agent.Instance{}, "agent")
	orm.RegisterSchemaWithIndexName(host.HostInfo{}, "host")
	agent.RegisterClient(&Client{})

	if module.AgentConfig.StateManager.Enabled {
		onlineAgentIDs, err := getLatestOnlineAgentIDs(nil, 60)
		if err != nil {
			log.Error(err)
		}
		agents, err := loadAgentsFromES("")
		if err != nil {
			log.Error(err)
		}
		taskState := map[string]agent.ShortState{}
		agentIds := map[string]string{}
		for _, ag := range agents {
			if _, ok := onlineAgentIDs[ag.ID]; ok {
				agentIds[ag.ID] = "online"
			}
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
	}
	return nil
}

func (module *AgentModule) Stop() error {
	if !module.AgentConfig.Enabled {
		return nil
	}
	log.Info("start to stop agent module")
	if module.AgentConfig.StateManager.Enabled {
		agent.GetStateManager().Stop()
	}
	log.Info("agent module was stopped")
	return nil
}

type AgentModule struct {
	common.AgentConfig
}

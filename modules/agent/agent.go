/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/agent/api"
	log "src/github.com/cihub/seelog"
	"time"
)

func (module *AgentModule) Name() string {
	return "agent"
}



//func loadAgentFromES() []agent.Instance {
//	configs := []agent.Instance{}
//	query := orm.Query{
//		Size: 1000,
//	}
//
//	err, result := orm.Search(agent.Instance{}, &query)
//	if err != nil {
//		log.Error(err)
//		return configs
//	}
//
//	if len(result.Result) > 0 {
//		for _, row := range result.Result {
//			cfg := agent.Instance{}
//			bytes := util.MustToJSONBytes(row)
//			util.MustFromJSONBytes(bytes, &cfg)
//			configs = append(configs, cfg)
//		}
//	}
//
//	log.Infof("loading [%v] agent configs", len(result.Result))
//	return configs
//}



func (module *AgentModule) Setup(cfg *config.Config) {
	orm.RegisterSchemaWithIndexName(agent.Instance{}, "agent")
	agents, err := loadAgentsFromES("")
	if err != nil {
		log.Error(err)
	}
	taskState := map[string]agent.ShortState{}
	agentIds := map[string]struct{}{}
	for _, ag := range agents {
		agentIds[ag.ID] = struct{}{}
		for _, cluster := range ag.Clusters {
			if cluster.Task.ClusterMetric.Owner {
				taskState[cluster.ClusterID] = agent.ShortState{
					ClusterMetricTask: agent.ClusterMetricTaskState{
						AgentID: ag.ID,
						NodeUUID: cluster.Task.ClusterMetric.TaskNodeID,
					},
				}
			}
		}
	}
	sm := NewStateManager(time.Second * 60, "agent_state", taskState, agentIds)
	agent.RegisterStateManager(sm)
	go sm.LoopState()
	//todo reassign tasks and refresh state automatically
	//go func() {
	//	elastic.WalkConfigs(func(key, value interface{}) bool {
	//
	//		return true
	//	})
	//}()
	api.Init()
}
func (module *AgentModule) Start() error {
	return nil
}

func (module *AgentModule) Stop() error {
	log.Info("start to stop agent module")
	agent.GetStateManager().Stop()
	log.Info("agent module was stopped")
	return nil
}


type AgentModule struct {
}

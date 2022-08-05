/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent


var stateManager IStateManager

func GetStateManager() IStateManager {
	if stateManager == nil {
		panic("agent state manager not init")
	}
	return stateManager
}

func RegisterStateManager(sm IStateManager)  {
	stateManager = sm
}

type IStateManager interface {
	GetAgent(ID string) (*Instance, error)
	UpdateAgent(inst *Instance, syncToES bool) (*Instance, error)
	DispatchAgent(clusterID string) (*Instance, error)
	GetTaskAgent(clusterID string) (*Instance, error)
	SetTaskAgent(clusterID, agentID string, nodeUUID string) error
	DeleteAgent(agentID string) error
	LoopState()
	Stop()
}
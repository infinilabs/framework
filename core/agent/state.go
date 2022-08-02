/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent


var stateManager IStateManager

func GetServiceInstance() IStateManager {
	if stateManager == nil {
		panic("agent service not init")
	}
	return stateManager
}

func RegisterStateManager(sm IStateManager)  {
	stateManager = sm
}

type IStateManager interface {
	GetAgent(ID string) (*Instance, error)
	UpdateAgent(inst *Instance) (*Instance, error)
	DispatchAgent(clusterID string) (*Instance, error)
	GetTaskAgent(clusterID string) (*Instance, error)
	SetTaskAgent(clusterID, agentID string) error
}
/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"runtime"
	"sync"
	"time"
)


type StateManager struct {
	TTL time.Duration // kv ttl
	KVKey string
	taskState map[string]agent.ShortState// cluster_id => {agent_id, node_uuid}
	stateMutex sync.RWMutex
	stopC chan struct{}
	stopCompleteC chan struct{}
	agentClient *Client
	agentIds map[string]struct{}
	agentMutex sync.Mutex
	workerChan chan struct{}
}



func NewStateManager(TTL time.Duration, kvKey string, taskState map[string]agent.ShortState, agentIds map[string]struct{} ) *StateManager {
	if taskState == nil {
		taskState= map[string]agent.ShortState{}
	}
	return &StateManager{
		TTL: TTL,
		KVKey: kvKey,
		taskState: taskState,
		stateMutex: sync.RWMutex{},
		stopC: make(chan struct{}),
		stopCompleteC: make(chan struct{}),
		agentClient: &Client{
		},
		agentIds: agentIds,
		workerChan: make(chan struct{}, runtime.NumCPU()),
	}
}

func (sm *StateManager) checkAgentStatus(){
	for agentID := range sm.agentIds {
		sm.workerChan <- struct{}{}
		go func(agentID string) {
			defer func() {
				<- sm.workerChan
			}()
			ag, err := sm.GetAgent(agentID)
			if err != nil {
				log.Error(err)
			}
			if ag != nil && time.Since(ag.Timestamp) > sm.TTL && ag.Status == "online" {
				ag.Status = "offline"
				log.Infof("agent [%s] is offline", ag.Host)
				_, err = sm.UpdateAgent(ag, true)
				if err != nil {
					log.Error(err)
					//continue
				}
				for _, cluster := range ag.Clusters {
					if cluster.Task.ClusterMetric.Owner {
						_, err = sm.DispatchAgent(cluster.ClusterID)
						if err != nil {
							log.Error(err)
						}
					}
				}

				//log.Infof("agent [%s] seems not work ", state.AgentID)
			}
		}(agentID)

	}
}

func (sm *StateManager) GetState(clusterID string) agent.ShortState  {
	sm.stateMutex.RLock()
	defer sm.stateMutex.RUnlock()
	return sm.taskState[clusterID]
}

func (sm *StateManager) LoopState() {
	//for console restart
	isPrepare := true
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	MAINLOOP:
	for {
		select {
			case <-sm.stopC:
				sm.stopCompleteC <- struct{}{}
				close(sm.workerChan)
				break MAINLOOP
			case <-t.C:
				if isPrepare {
					isPrepare = false
					continue
				}
				sm.checkAgentStatus()
		}
	}
}

func (sm *StateManager) Stop() {
	sm.stopC <- struct{}{}
	<-sm.stopCompleteC
}

func (sm *StateManager) GetAgent(ID string) (*agent.Instance, error){
	 buf, err := kv.GetValue(sm.KVKey, []byte(ID))
	 if err != nil {
		 return nil, err
	 }
	 inst := &agent.Instance{}
	 err = util.FromJSONBytes(buf, inst)
	 if err != nil {
		 return nil, err
	 }
	 inst.ID = ID
	 if time.Since(inst.Timestamp) > sm.TTL {
		exists, err := orm.Get(inst)
		if err != nil {
			return nil, fmt.Errorf("get agent error: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("can not found agent [%s]", ID)
		}
		//inst.Timestamp = time.Now()
		err = kv.AddValue(sm.KVKey, []byte(ID), util.MustToJSONBytes(inst))
		if err != nil {
			log.Errorf("save agent [%s] to kv error: %v", ID, err)
		}
		return inst, nil
	 }
	 return inst, nil
}

func (sm *StateManager) UpdateAgent(inst *agent.Instance, syncToES bool) (*agent.Instance, error){
	inst.Timestamp = time.Now()
	sm.agentMutex.Lock()
	sm.agentIds[inst.ID] = struct{}{}
	sm.agentMutex.Unlock()
	err := kv.AddValue(sm.KVKey, []byte(inst.ID), util.MustToJSONBytes(inst))
	if syncToES {
		err = orm.Update(inst)
		if err != nil {
			return nil, err
		}
	}
	return inst, err
}
func (sm *StateManager) DispatchAgent(clusterID string) (*agent.Instance, error) {
	state, err := sm.calcTaskAgent(clusterID)
	if err != nil {
		return nil, err
	}

	sm.setTaskAgent(clusterID, state)
	return sm.GetAgent(state.ClusterMetricTask.AgentID)
}
func (sm *StateManager) GetTaskAgent(clusterID string) (*agent.Instance, error) {
	sm.stateMutex.RLock()
	defer sm.stateMutex.RUnlock()
	if state, ok := sm.taskState[clusterID]; ok {
		return sm.GetAgent(state.ClusterMetricTask.AgentID)
	}
	return nil, nil
}

func (sm *StateManager) DeleteAgent(agentID string) error{
	sm.stateMutex.Lock()
	defer sm.stateMutex.Unlock()
	for clusterID, state := range sm.taskState {
		if agentID == state.ClusterMetricTask.AgentID {
			delete(sm.taskState, clusterID)
		}
	}
	sm.agentMutex.Lock()
	delete(sm.agentIds, agentID)
	sm.agentMutex.Unlock()
	return kv.DeleteKey(sm.KVKey, []byte(agentID))
}

func (sm *StateManager) setTaskAgent(clusterID string, state *agent.ShortState){
	//update old agent state
	if oldState, ok := sm.taskState[clusterID]; ok && oldState.ClusterMetricTask.NodeUUID != "" {
		if err := sm.syncState(clusterID, oldState.ClusterMetricTask.AgentID, oldState.ClusterMetricTask.NodeUUID, false); err != nil {
			log.Error(err)
			return
		}
		//todo request target agent to sync state
	}

	if state != nil {
		//update new agent state
		if err := sm.syncState(clusterID, state.ClusterMetricTask.AgentID, state.ClusterMetricTask.NodeUUID, true); err != nil {
			log.Error(err)
			return
		}
		//todo request target agent to sync state
	}
	sm.stateMutex.Lock()
	defer sm.stateMutex.Unlock()
	if state == nil {
		delete(sm.taskState, clusterID)
	} else {
		sm.taskState[clusterID] = *state
	}
}

func (sm *StateManager) syncState(clusterID string, agentID, nodeUUID string, taskOwner bool) error {
		oldAgent, err := sm.GetAgent(agentID)
		if err != nil {
			return err
		}
		if oldAgent == nil {
			return fmt.Errorf("can not found agent [%s]", agentID)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
		defer cancel()
		if !taskOwner {
			err = sm.agentClient.DisableTaskToNode(ctx, oldAgent.GetEndpoint(), nodeUUID)
			if err != nil {
				log.Infof("call agent disable task error: %v", err)
			}
		}else{
			err = sm.agentClient.EnableTaskToNode(ctx, oldAgent.GetEndpoint(), nodeUUID)
			if err != nil {
				return fmt.Errorf("call agent enable task error: %w", err)
			}
		}

		for i, cluster := range oldAgent.Clusters {
			if cluster.ClusterID == clusterID {
				oldAgent.Clusters[i].Task.ClusterMetric.Owner = taskOwner
				if taskOwner {
					oldAgent.Clusters[i].Task.ClusterMetric.TaskNodeID = nodeUUID
					break
				}
				oldAgent.Clusters[i].Task.ClusterMetric.TaskNodeID = ""
			}
		}
		_, err = sm.UpdateAgent(oldAgent, true)
		return err
}
func (sm *StateManager) SetAgentTask(clusterID, agentID string, nodeUUID string) error {
	var (
		ag *agent.Instance
		err error
	)
	if agentID != "" {
		ag, err = sm.GetAgent(agentID)
		if err != nil {
			return err
		}
		if ag.Status != "online" {
			return fmt.Errorf("agent [%s] is offline", agentID)
		}
	}
	state := &agent.ShortState{
		ClusterMetricTask: agent.ClusterMetricTaskState{
			AgentID: agentID,
			NodeUUID: nodeUUID,
		},
	}
	sm.setTaskAgent(clusterID, state)
	return nil
}
func (sm *StateManager) StopAgentTask(clusterID, agentID string, nodeUUID string) error {
	return nil
}
func (sm *StateManager) calcTaskAgent(clusterID string) (*agent.ShortState, error){
	agents, err := loadAgentsFromES(clusterID)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	var targetNodeID string
	nodeToAgent := map[string]int{}
	for i, ag := range agents {
		for _, cluster := range ag.Clusters {
			if cluster.ClusterID == clusterID {
				for _, n := range cluster.Nodes {
					nodeToAgent[n.UUID] = i
				}
			}
		}
	}
	esClient := elastic.GetClient(clusterID)
	nodes, err := esClient.CatNodes("id,name,ip,port,master")
	if err != nil {
		return nil, fmt.Errorf("cat nodes error: %w", err)
	}
	var (
		masterIdx = -1
		agentIdx = -1
		ok bool
	)
	for i, node := range nodes {
		if node.Master == "*" {
			masterIdx = i
			continue
		}
		if agentIdx, ok = nodeToAgent[node.Id]; ok {
			targetNodeID = node.Id
			break
		}
	}
	if agentIdx > -1 && agents[agentIdx].Status == "online" {
		var targetAgent = agents[agentIdx]
		return &agent.ShortState{
			ClusterMetricTask: agent.ClusterMetricTaskState{
				AgentID: targetAgent.ID,
				NodeUUID: targetNodeID,
			},
		}, nil
	}
	if masterIdx > -1 {
		if agentIdx, ok = nodeToAgent[nodes[masterIdx].Id]; ok && agents[agentIdx].Status == "online" {
			var targetAgent = agents[agentIdx]
			return  &agent.ShortState{
				ClusterMetricTask: agent.ClusterMetricTaskState{
					AgentID:  targetAgent.ID,
					NodeUUID: nodes[masterIdx].Id,
				},
			}, nil
		}
	}
	return nil, nil
}

func loadAgentsFromES(clusterID string) ([]agent.Instance, error){
	q := orm.Query{
		Size: 1000,
	}
	if clusterID != "" {
		q.Conds = orm.And(orm.Eq("clusters.cluster_id", clusterID))
	}
	err, result := orm.Search(agent.Instance{}, &q)
	if err != nil {
		return nil, fmt.Errorf("query agent error: %w", err)
	}

	if len(result.Result) > 0 {
		var agents = make([]agent.Instance, 0, len(result.Result))
		for _, row := range result.Result {
			ag := agent.Instance{}
			bytes := util.MustToJSONBytes(row)
			util.MustFromJSONBytes(bytes, &ag)
			agents = append(agents, ag)
		}
		return agents, nil
	}
	return nil, nil
}
/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"fmt"
	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type StateManager struct {
	TTL           time.Duration // kv ttl
	KVKey         string
	taskState     map[string]agent.ShortState // cluster_id => {agent_id, node_uuid}
	stateMutex    sync.RWMutex
	stopC         chan struct{}
	stopCompleteC chan struct{}
	agentClient   *Client
	agentIds      map[string]struct{}
	agentMutex    sync.Mutex
	workerChan    chan struct{}
}

func NewStateManager(TTL time.Duration, kvKey string, taskState map[string]agent.ShortState, agentIds map[string]struct{}) *StateManager {
	if taskState == nil {
		taskState = map[string]agent.ShortState{}
	}
	return &StateManager{
		TTL:           TTL,
		KVKey:         kvKey,
		taskState:     taskState,
		stateMutex:    sync.RWMutex{},
		stopC:         make(chan struct{}),
		stopCompleteC: make(chan struct{}),
		agentClient:   &Client{},
		agentIds:      agentIds,
		workerChan:    make(chan struct{}, runtime.NumCPU()),
	}
}

func (sm *StateManager) DispatchNodeMetricTask(clusterID string) error {
	oldState := sm.GetState(clusterID)
	//not set task to agent
	if oldState.ClusterMetricTask.AgentID == "" {
		return nil
	}
	var (
		targetAg *agent.Instance
		err error
	)
	if oldState.NodeMetricTask.AgentID != "" {
		targetAg, _ = sm.GetAgent(oldState.NodeMetricTask.AgentID)
	}
	extraNodes, err := sm.getNodesWithoutAgent(clusterID)
	if err != nil {
		return err
	}
	if oldState.NodeMetricTask.AgentID == "" || (targetAg != nil && targetAg.Status != "online") {
		if targetAg != nil && targetAg.Status != "online" {
			for i, cluster := range targetAg.Clusters {
				if cluster.ClusterID == clusterID {
					targetAg.Clusters[i].Task.NodeMetric = nil
					break
				}
			}
			_, err = sm.UpdateAgent(targetAg, true)
			if err != nil {
				return err
			}
		}
		if len(extraNodes) > 0 {
			targetAg, err = sm.getAvailableAgent(clusterID)
			if err != nil {
				return err
			}
		}
	}
	if targetAg == nil {
		return nil
	}
	oldState.NodeMetricTask = agent.NodeMetricTaskState{
		AgentID: targetAg.ID,
		Nodes: extraNodes,
	}
	err = sm.agentClient.SetNodesMetricTask(nil, targetAg.GetEndpoint(), map[string]interface{}{
		clusterID: extraNodes,
	})
	if err != nil {
		return err
	}
	for i, cluster := range targetAg.Clusters {
		if cluster.ClusterID == clusterID {
			targetAg.Clusters[i].Task.NodeMetric = &agent.NodeMetricTask{
				Owner: true,
				ExtraNodes: extraNodes,
			}
			break
		}
	}
	_, err = sm.UpdateAgent(targetAg, true)
	if err != nil {
		return err
	}
	sm.stateMutex.Lock()
	sm.taskState[clusterID] = oldState
	sm.stateMutex.Unlock()
	return nil
}

func (sm *StateManager) checkAgentStatus() {
	for agentID := range sm.agentIds {
		sm.workerChan <- struct{}{}
		go func(agentID string) {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("check agent [%s] status recover form panic error: %v", agentID, err)
					debug.PrintStack()
				}
				<-sm.workerChan
			}()
			ag, err := sm.GetAgent(agentID)
			if err != nil {
				log.Error(err)
			}
			if ag != nil && time.Since(ag.Timestamp) > sm.TTL && ag.Status == "online" {
				ag.Status = "offline"
				log.Infof("agent [%s] is offline", ag.RemoteIP)
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
					err = sm.DispatchNodeMetricTask(cluster.ClusterID)
					if err != nil {
						log.Errorf("dispatch node metric task error: %v", err)
					}
				}

				//log.Infof("agent [%s] seems not work ", state.AgentID)
			}
		}(agentID)

	}
}

func (sm *StateManager) getAvailableAgent(clusterID string) (*agent.Instance, error) {
	agents, err := loadAgentsFromES(clusterID)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	for _, ag := range agents {
		if ag.Status == "offline" {
			continue
		}
		for _, cluster := range ag.Clusters {
			if cluster.ClusterID == clusterID {
				return &ag, nil
			}
		}
	}
	return nil, nil
}

func (sm *StateManager) GetState(clusterID string) agent.ShortState {
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

func (sm *StateManager) GetAgent(ID string) (*agent.Instance, error) {
	buf, err := kv.GetValue(sm.KVKey, []byte(ID))
	if err != nil {
		return nil, err
	}
	strTime, _ := jsonparser.GetString(buf, "timestamp")
	timestamp, _ := time.Parse(time.RFC3339, strTime)
	inst := &agent.Instance{}
	inst.ID = ID
	if time.Since(timestamp) > sm.TTL {
		exists, err := orm.Get(inst)
		if err != nil {
			return nil, fmt.Errorf("get agent [%s] error: %w", ID, err)
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
	err = util.FromJSONBytes(buf, inst)
	return inst, err
}

func (sm *StateManager) UpdateAgent(inst *agent.Instance, syncToES bool) (*agent.Instance, error) {
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
		//for es search latency
		time.Sleep(time.Second)
	}
	return inst, err
}
func (sm *StateManager) DispatchAgent(clusterID string) (*agent.Instance, error) {
	state, err := sm.calcTaskAgent(clusterID)
	if err != nil {
		return nil, err
	}

	if state != nil {
		log.Infof("dispatch cluster metric task of cluster [%s] to agent [%s], node [%s]", clusterID, state.ClusterMetricTask.AgentID, state.ClusterMetricTask.NodeUUID)
	} else {
		log.Infof("dispatch cluster metric task of cluster [%s] to console", clusterID)
	}
	sm.setTaskAgent(clusterID, state)
	if state == nil {
		return nil, nil
	}

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

func (sm *StateManager) DeleteAgent(agentID string) error {
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
	log.Infof("delete agent [%s] from state", agentID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	inst, err := sm.GetAgent(agentID)
	if err != nil {
		return err
	}

	err = sm.agentClient.DeleteInstance(ctx, inst.GetEndpoint(), agentID)
	if err != nil {
		log.Errorf("delete agent instance error: %v", err)
	}
	return kv.DeleteKey(sm.KVKey, []byte(agentID))
}

func (sm *StateManager) setTaskAgent(clusterID string, state *agent.ShortState) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if !taskOwner {
		err = sm.agentClient.DisableTaskToNode(ctx, oldAgent.GetEndpoint(), nodeUUID)
		if err != nil {
			log.Infof("call agent disable task error: %v", err)
		}
	} else {
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
		ag  *agent.Instance
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
			AgentID:  agentID,
			NodeUUID: nodeUUID,
		},
	}
	oldState := sm.GetState(clusterID)
	state.NodeMetricTask = oldState.NodeMetricTask

	sm.setTaskAgent(clusterID, state)

	err = sm.DispatchNodeMetricTask(clusterID)
	if err != nil {
		return fmt.Errorf("dispatch node metric task error: %w", err)
	}

	log.Infof("set task state for cluster [%s]: %s", clusterID, util.MustToJSON(sm.GetState(clusterID)))
	return nil
}

func (sm *StateManager) getNodesWithoutAgent(clusterID string) ([]string, error){
	agents, err := loadAgentsFromES(clusterID)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	nodeToAgent := map[string]int{}
	for i, ag := range agents {
		if ag.Status == "offline" {
			continue
		}
		for _, cluster := range ag.Clusters {
			if cluster.ClusterID == clusterID {
				for _, n := range cluster.Nodes {
					nodeToAgent[n.UUID] = i
				}
			}
		}
	}
	//no available agent
	if len(nodeToAgent) == 0 {
		return nil, nil
	}
	esClient := elastic.GetClient(clusterID)
	nodes, err := esClient.CatNodes("id,name,ip,port,master")
	if err != nil {
		return nil, fmt.Errorf("cat nodes error: %w", err)
	}
	var (
		tempIdx = -1
		ok        bool
		nodesWithoutAgent []string
	)
	for _, node := range nodes {
		if tempIdx, ok = nodeToAgent[node.Id]; ok {
			if tempIdx > -1 && agents[tempIdx].Status == "online" {
				continue
			}
		}
		nodesWithoutAgent = append(nodesWithoutAgent, node.Id)
	}
	return nodesWithoutAgent, nil
}

func (sm *StateManager) StopAgentTask(clusterID, agentID string, nodeUUID string) error {
	return nil
}

func (sm *StateManager) EnrollAgent(inst *agent.Instance, confirmInfo interface{}) error {
	inst.Enrolled = true
	_, err := sm.UpdateAgent(inst, true)
	if err != nil {
		return err
	}
	log.Tracef("callback to agent client [%s]: %s", inst.GetEndpoint(), util.MustToJSON(confirmInfo))
	return sm.agentClient.EnrollInstance(nil, inst.GetEndpoint(), inst.ID, confirmInfo)
}
func (sm *StateManager) calcTaskAgent(clusterID string) (*agent.ShortState, error) {
	agents, err := loadAgentsFromES(clusterID)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	var targetNodeID string
	nodeToAgent := map[string]int{}
	oldState := sm.GetState(clusterID)
	for i, ag := range agents {
		//return whether agent is offline or agent id is equal old (es search latency )
		if ag.Status == "offline" || oldState.ClusterMetricTask.AgentID == ag.ID {
			continue
		}
		for _, cluster := range ag.Clusters {
			if cluster.ClusterID == clusterID {
				for _, n := range cluster.Nodes {
					nodeToAgent[n.UUID] = i
				}
			}
		}
	}
	//no available agent
	if len(nodeToAgent) == 0 {
		return nil, nil
	}
	esClient := elastic.GetClient(clusterID)
	nodes, err := esClient.CatNodes("id,name,ip,port,master")
	if err != nil {
		return nil, fmt.Errorf("cat nodes error: %w", err)
	}
	var (
		masterIdx = -1
		agentIdx  = -1
		tempIdx = -1
		ok        bool
		nodesWithoutAgent []string
	)
	for i, node := range nodes {
		if node.Master == "*" {
			masterIdx = i
		}
		if tempIdx, ok = nodeToAgent[node.Id]; ok {
			if tempIdx > -1 && agents[tempIdx].Status == "online" {
				if targetNodeID == "" {
					agentIdx = tempIdx
					targetNodeID = node.Id
				}
				continue
			}
		}
		nodesWithoutAgent = append(nodesWithoutAgent, node.Id)
	}
	if agentIdx > -1 {
		var targetAgent = agents[agentIdx]
		return &agent.ShortState{
			ClusterMetricTask: agent.ClusterMetricTaskState{
				AgentID:  targetAgent.ID,
				NodeUUID: targetNodeID,
			},
			NodeMetricTask: oldState.NodeMetricTask,
		}, nil
	}
	if masterIdx > -1 {
		if agentIdx, ok = nodeToAgent[nodes[masterIdx].Id]; ok && agents[agentIdx].Status == "online" {
			var targetAgent = agents[agentIdx]
			return &agent.ShortState{
				ClusterMetricTask: agent.ClusterMetricTaskState{
					AgentID:  targetAgent.ID,
					NodeUUID: nodes[masterIdx].Id,
				},
			}, nil
		}
	}
	return nil, nil
}

func loadAgentsFromES(clusterID string) ([]agent.Instance, error) {
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
			err = util.FromJSONBytes(bytes, &ag)
			if err != nil {
				log.Errorf("got unexpected agent: %s, error: %v", string(bytes), err)
				continue
			}
			agents = append(agents, ag)
		}
		return agents, nil
	}
	return nil, nil
}

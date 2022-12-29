/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	elastic2 "infini.sh/framework/modules/elastic"
	"net/http"
	"strconv"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) createInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &agent.Instance{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if obj.Port == 0 {
		h.WriteError(w, fmt.Sprintf("invalid port [%d] of agent", obj.Port), http.StatusInternalServerError)
		return
	}
	if obj.Schema == "" {
		obj.Schema = "http"
	}
	remoteIP := util.ClientIP(req)
	obj.RemoteIP = remoteIP
	res, err := agent.GetClient().GetInstanceBasicInfo(context.Background(), obj.GetEndpoint())
	if err != nil {
		errStr := fmt.Sprintf("get agent instance basic info error: %s", err.Error())
		h.WriteError(w,errStr , http.StatusInternalServerError)
		log.Error(errStr)
		return
	}
	if id, ok := res["id"].(string); !ok {
		errStr :=fmt.Sprintf("got unexpected response of agent instance basic info: %s", util.MustToJSON(res))
		h.WriteError(w, errStr , http.StatusInternalServerError)
		log.Error(errStr)
		return
	}else{
		obj.ID = id
	}
	if v, ok := res["name"].(string); ok {
		obj.Name = v
	}

	oldInst := &agent.Instance{}
	oldInst.ID = obj.ID
	exists, err := orm.Get(oldInst)

	if err != nil && err != elastic2.ErrNotFound {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if exists {
		errMsg := fmt.Sprintf("agent [%s] already exists", remoteIP)
		h.WriteError(w, errMsg, http.StatusInternalServerError)
		log.Error(errMsg)
		return
	}
	obj.Enrolled = true

	err = orm.Create(nil, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	log.Infof("receive agent register from host [%s]: %s", obj.RemoteIP, util.MustToJSON(obj))

	h.WriteCreatedOKJSON(w, obj.ID)

}

func bindAgentToHostByIP(ag *agent.Instance) error{
	err, result := orm.GetBy("ip", ag.MajorIP, host.HostInfo{})
	if err != nil {
		return err
	}
	if len(result.Result) > 0 {
		buf := util.MustToJSONBytes(result.Result[0])
		hostInfo := &host.HostInfo{}
		err = util.FromJSONBytes(buf, hostInfo)
		if err != nil {
			return err
		}
		sm := agent.GetStateManager()
		if ag.Status == "" {
			_, err1 := sm.GetAgentClient().GetHostInfo(nil, ag.GetEndpoint(), ag.ID)
			if err1 == nil {
				ag.Status = "online"
			}else{
				ag.Status = "offline"
			}
		}

		hostInfo.AgentStatus = ag.Status
		hostInfo.AgentID = ag.ID
		err = orm.Update(nil, hostInfo)
		if err != nil {
			return  err
		}

		err = sm.GetAgentClient().DiscoveredHost(nil, ag.GetEndpoint(), util.MapStr{
			"host_id": hostInfo.ID,
		})
		if err != nil {
			return  err
		}
	}
	return nil
}

func (h *APIHandler) getInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")

	obj := agent.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (h *APIHandler) updateInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	obj := agent.Instance{}

	obj.ID = id
	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	newObj := agent.Instance{}
	err = h.DecodeJSON(req, &newObj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if newObj.Port != obj.Port {
		obj.Port = newObj.Port
	}
	if newObj.Schema != obj.Schema {
		obj.Schema = newObj.Schema
	}
	if len(newObj.Version) > 0 {
		obj.Version = newObj.Version
	}
	if len(newObj.IPS) > 0 {
		obj.IPS = newObj.IPS
	}
	remoteIP := util.ClientIP(req)
	if remoteIP != obj.RemoteIP {
		log.Infof("agent [%s] remote ip changed from [%s] to [%s]", obj.ID, obj.RemoteIP, remoteIP)
		obj.RemoteIP = remoteIP
	}

	if global.Env().IsDebug {
		log.Debug("update agent [%s]: %v", obj.ID, util.MustToJSON(obj))
	}

	clusterM := map[string]int{}
	for i, cluster := range obj.Clusters {
		clusterM[cluster.ClusterUUID] = i
	}
	for i, cluster := range newObj.Clusters {
		if j, ok := clusterM[cluster.ClusterUUID]; ok {
			newObj.Clusters[i].Task = obj.Clusters[j].Task
		}
	}
	obj.Clusters = newObj.Clusters
	err = orm.Update(nil, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":      obj.ID,
		"result":   "updated",
	}, 200)
}

func (h *APIHandler) setTaskToInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	reqBody := []struct {
		ClusterID string `json:"cluster_id"`
		NodeUUID  string `json:"node_uuid"`
	}{}

	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	sm := agent.GetStateManager()
	for _, node := range reqBody {
		err = sm.SetAgentTask(node.ClusterID, id, node.NodeUUID)
		if err != nil {
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			log.Error(err)
			return
		}
	}

	h.WriteJSON(w, util.MapStr{
		"result": "success",
	}, 200)
}

func (h *APIHandler) deleteInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")

	obj := agent.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	err = orm.Delete(nil, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if sm := agent.GetStateManager(); sm != nil {
		sm.DeleteAgent(obj.ID)
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "deleted",
	}, 200)
}

func (h *APIHandler) getInstanceStats(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var instanceIDs = []string{}
	err := h.DecodeJSON(req, &instanceIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(instanceIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	q := orm.Query{
		WildcardIndex: true,
	}
	queryDSL := util.MapStr{
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
		"collapse": util.MapStr{
			"field": "agent.id",
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"filter": []util.MapStr{
					{
						"range": util.MapStr{
							"timestamp": util.MapStr{
								"gte": "now-1m",
							},
						},
					},
				},
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.name": util.MapStr{
								"value": "agent",
							},
						},
					}, {
						"terms": util.MapStr{
							"agent.id": instanceIDs,
						},
					},
				},
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(event.Event{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result := util.MapStr{}
	for _, item := range res.Result {
		if itemV, ok := item.(map[string]interface{}); ok {
			if agentID, ok := util.GetMapValueByKeys([]string{"agent", "id"}, itemV); ok {
				if v, ok := agentID.(string); ok {
					if ab, ok := util.GetMapValueByKeys([]string{"payload","instance", "system"}, itemV); ok{
						if abV, ok := ab.(map[string]interface{}); ok {
							result[v] = util.MapStr{
								"system": util.MapStr{
									"cpu": abV["cpu"],
									"mem": abV["mem"],
									"uptime_in_ms": abV["uptime_in_ms"],
									"status": "online",
								},
							}
						}
					}
				}
			}
		}

	}
	h.WriteJSON(w, result, http.StatusOK)
}


func (h *APIHandler) searchInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var (
		keyword = h.GetParameterOrDefault(req, "keyword", "")
		//queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		strSize      = h.GetParameterOrDefault(req, "size", "20")
		strFrom      = h.GetParameterOrDefault(req, "from", "0")
	)

	var (
		mustQ       []interface{}
	)

	if keyword != "" {
		mustQ = append(mustQ, util.MapStr{
			"query_string": util.MapStr{
				"default_field": "*",
				"query":         keyword,
			},
		})
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	queryDSL := util.MapStr{
		"size": size,
		"from": from,
	}
	if len(mustQ) > 0 {
		queryDSL["query"] = util.MapStr{
			"bool": util.MapStr{
				"must": mustQ,
			},
		}
	}

	q := orm.Query{}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&agent.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.Write(w, res.Raw)
}

func (h *APIHandler) getClusterInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := h.GetParameterOrDefault(req, "cluster_id", "")
	if clusterID == "" {
		h.WriteError(w, "parameter cluster_id should not be empty", http.StatusInternalServerError)
		return
	}
	esClient := elastic.GetClient(clusterID)
	nodes, err := esClient.CatNodes("id,ip,name")
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nodesM := make(map[string]*struct {
		NodeID    string
		IP        string
		Name      string
		AgentHost string
		Owner     bool
	}, len(nodes))
	for _, node := range nodes {
		nodesM[node.Id] = &struct {
			NodeID    string
			IP        string
			Name      string
			AgentHost string
			Owner     bool
		}{NodeID: node.Id, IP: node.Ip, Name: node.Name}
	}
	query := util.MapStr{
		"query": util.MapStr{
			"term": util.MapStr{
				"clusters.cluster_id": util.MapStr{
					"value": clusterID,
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}
	err, result := orm.Search(agent.Instance{}, q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, row := range result.Result {
		buf := util.MustToJSONBytes(row)
		inst := &agent.Instance{}
		util.MustFromJSONBytes(buf, inst)
		for _, cluster := range inst.Clusters {
			for _, n := range cluster.Nodes {
				if _, ok := nodesM[n.UUID]; ok {
					nodesM[n.UUID].AgentHost = inst.RemoteIP
					nodesM[n.UUID].Owner = cluster.Task.ClusterMetric.TaskNodeID == n.UUID
				}
			}
		}
	}

	h.WriteJSON(w, nodesM, 200)
}
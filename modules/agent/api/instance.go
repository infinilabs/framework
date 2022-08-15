/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) heartbeat(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	sm := agent.GetStateManager()
	inst, err := sm.GetAgent(id)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	syncToES := inst.Status != "online"
	inst.Status = "online"
	host := util.ClientIP(req)
	log.Tracef("heartbeat from [%s]", host)
	ag, err := sm.UpdateAgent(inst, syncToES)
	if err != nil {
		log.Error(err)
	}
	taskState := map[string]map[string]string{}
	for _, cluster := range ag.Clusters {
		taskState[cluster.ClusterID] = map[string]string{
			"cluster_metric": sm.GetState(cluster.ClusterID).ClusterMetricTask.NodeUUID,
		}
	}

	h.WriteJSON(w, util.MapStr{
		"agent_id":   id,
		"result": "ok",
		"task_state": taskState,
		"timestamp": time.Now().Unix(),
	}, 200)
}

func (h *APIHandler) getIP(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	remoteHost := util.ClientIP(req)
	h.WriteJSON(w, util.MapStr{
		"ip": remoteHost,
	}, http.StatusOK)
}

func (h *APIHandler) createInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &agent.Instance{
		Confirmed: true,
	}
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
	q := &orm.Query{
		Size: 2,
	}
	remoteIP := util.ClientIP(req)
	q.Conds = orm.And(orm.Eq("host", remoteIP))
	err, result := orm.Search(obj, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if len(result.Result) > 0 {
		errMsg := fmt.Sprintf("agent [%s] already exists", remoteIP)
		h.WriteError(w, errMsg, http.StatusInternalServerError)
		log.Error(errMsg)
		return
	}

	//match clusters
	obj.Host = remoteIP
	clusters, err := getMatchedClusters(obj.Host, obj.Clusters)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	var filterClusters []agent.ESCluster
	//remove clusters of not matched
	for i, cluster := range obj.Clusters {
		if vmap, ok := clusters[cluster.ClusterName].(map[string]interface{}); ok {
			obj.Clusters[i].ClusterID = vmap["cluster_id"].(string)
			filterClusters = append(filterClusters, obj.Clusters[i])
		}
	}
	obj.Clusters = filterClusters
	obj.Status = "online"

	log.Trace("register agent [%s]: %v", obj.Host, obj)
	err = orm.Create(obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	sm := agent.GetStateManager()
	_, err = sm.UpdateAgent(obj, false)
	if err != nil {
		log.Error(err)
	}
	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"clusters": clusters,
		"result": "created",
	}, 200)

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
	newMatchedClusters, err :=  h.updateInstanceNodes(&obj, newObj.Clusters)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	log.Infof("update agent [%s]: %v", obj.Host, obj)
	err = orm.Update(&obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	sm := agent.GetStateManager()
	_, err = sm.UpdateAgent(&obj, false)
	if err != nil {
		log.Error(err)
	}
	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "updated",
		"clusters": newMatchedClusters,
	}, 200)
}
func (h *APIHandler) updateInstanceNodes(obj *agent.Instance, esClusters []agent.ESCluster) (map[string]interface{}, error){
	if len(esClusters) == 0 {
		return nil, fmt.Errorf("request body should not be empty")
	}

	clusters := map[string]agent.ESCluster{}
	var newClusters []agent.ESCluster
	for _, nc := range esClusters {
		if strings.TrimSpace(nc.ClusterID) == "" {
			newClusters = append(newClusters, nc)
			continue
		}
		clusters[nc.ClusterID] = nc
	}
	var toUpClusters []agent.ESCluster
	for _, cluster := range obj.Clusters {
		if upCluster, ok := clusters[cluster.ClusterID]; ok {
			newUpCluster := agent.ESCluster{
				ClusterUUID: cluster.ClusterUUID,
				ClusterName: upCluster.ClusterName,
				ClusterID: cluster.ClusterID,
				Nodes: upCluster.Nodes,
				Task: cluster.Task,
			}
			toUpClusters = append(toUpClusters, newUpCluster)
			continue
		}
		//todo log delete nodes
	}
	var (
		matchedClusters map[string]interface{}
		err error
	)
	if len(newClusters) > 0 {
		matchedClusters, err = getMatchedClusters(obj.Host, newClusters)
		if err != nil {
			return nil, err
		}
		//filter already
		//for _, cluster := range toUpClusters {
		//	if _, ok := matchedClusters[cluster.ClusterName]; ok {
		//		delete(matchedClusters, cluster.ClusterName)
		//	}
		//}
	}
	//attach old cluster
	oldMatchedClusters, err := getMatchedClusters(obj.Host, toUpClusters)
	if err != nil {
		return nil, err
	}

	for clusterName, matchedCluster := range matchedClusters {
		if vm, ok := matchedCluster.(map[string]interface{}); ok {
			toUpClusters = append(toUpClusters, agent.ESCluster{
				ClusterUUID: vm["cluster_uuid"].(string),
				ClusterName: clusterName,
				ClusterID: vm["cluster_id"].(string),
			})
		}
	}
	obj.Clusters = toUpClusters
	if matchedClusters == nil {
		matchedClusters = map[string]interface{}{}
	}
	err = util.MergeFields(matchedClusters, oldMatchedClusters, true)
	return matchedClusters, err


}
func (h *APIHandler) setTaskToInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	reqBody := []struct{
		ClusterID string `json:"cluster_id"`
		NodeUUID string `json:"node_uuid"`
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

	err = agent.GetStateManager().DeleteAgent(obj.ID)
	if err != nil {
		log.Error(err)
	}
	err = orm.Delete(&obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "deleted",
	}, 200)
}

func (h *APIHandler) searchInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var (
		keyword        = h.GetParameterOrDefault(req, "keyword", "")
		queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		strSize     = h.GetParameterOrDefault(req, "size", "20")
		strFrom     = h.GetParameterOrDefault(req, "from", "0")
		mustBuilder = &strings.Builder{}
	)
	if keyword != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"query_string":{"default_field":"*","query": "%s"}}`, keyword))
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	q := orm.Query{}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	q.RawQuery = []byte(queryDSL)

	err, res := orm.Search(&agent.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//searchRes := elastic.SearchResponse{}
	//util.MustFromJSONBytes(res.Raw, &searchRes)
	//for _, hit := range searchRes.Hits.Hits {
	//	hit.Source["task_count"] =
	//}


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
	nodesM := make(map[string]*struct{
		NodeID string
		IP string
		Name string
		AgentHost string
		Owner bool
	}, len(nodes))
	for _, node := range nodes {
		nodesM[node.Id] = &struct {
			NodeID  string
			IP      string
			Name    string
			AgentHost string
			Owner   bool
		}{NodeID: node.Id, IP: node.Ip, Name: node.Name }
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
					nodesM[n.UUID].AgentHost = inst.Host
					nodesM[n.UUID].Owner = cluster.Task.ClusterMetric.TaskNodeID == n.UUID
				}
			}
		}
	}

	h.WriteJSON(w, nodesM, 200)
}


func getMatchedClusters(host string, clusters []agent.ESCluster) (map[string]interface{}, error){
	resultClusters := map[string] interface{}{}
	for _, cluster := range clusters {
		queryDsl := util.MapStr{
			"query": util.MapStr{
				"bool": util.MapStr{
					"should": []util.MapStr{
						{
							"term": util.MapStr{
								"cluster_uuid": util.MapStr{
									"value": cluster.ClusterUUID,
								},
							},
						},
						{
							"bool": util.MapStr{
								"minimum_should_match": 1,
								"must": []util.MapStr{
									{
										"prefix": util.MapStr{
											"host": util.MapStr{
												"value": host,
											},
										},
									},
								},
								"should": []util.MapStr{
									{
										"term": util.MapStr{
											"raw_name": util.MapStr{
												"value": cluster.ClusterName,
											},
										},
									},
									{
										"term": util.MapStr{
											"name": util.MapStr{
												"value": cluster.ClusterName,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		q := &orm.Query{
			RawQuery: util.MustToJSONBytes(queryDsl),
		}
		err, result := orm.Search(elastic.ElasticsearchConfig{}, q)
		if err != nil {
			return nil, err
		}
		if len(result.Result) == 1 {
			buf := util.MustToJSONBytes(result.Result[0])
			esConfig := elastic.ElasticsearchConfig{}
			util.MustFromJSONBytes(buf, &esConfig)
			resultClusters[cluster.ClusterName] = map[string]interface{}{
				"cluster_id": esConfig.ID,
				"cluster_uuid": esConfig.ClusterUUID,
				"basic_auth": esConfig.BasicAuth,
			}
		}
	}
	return resultClusters, nil
}


func (h *APIHandler) getClusterAuth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := h.GetParameterOrDefault(req, "cluster_id", "")
	if clusterID == "" {
		h.WriteError(w, "parameter cluster_id should not be empty", http.StatusInternalServerError)
		return
	}
	//esClient := elastic.GetClient(clusterID)
}

func getAgentByHost(host string) (*agent.Instance, error){
	q := &orm.Query{
		Size: 1,
	}
	q.Conds = orm.And(orm.Eq("host", host))
	inst := agent.Instance{}
	err, result := orm.Search(inst, q)
	if err != nil {
		return nil, err
	}
	if len(result.Result) == 0 {
		return nil, nil
	}
	buf, err := util.ToJSONBytes(result.Result[0])
	if err != nil {
		return nil, err
	}
	err = util.FromJSONBytes(buf, &inst)
	return &inst, err
}
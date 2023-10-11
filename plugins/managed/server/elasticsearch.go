/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
)

func (h *APIHandler) refreshESNodesInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	obj := model.Instance{}
	obj.ID = id
	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	_, err = refreshNodesInfo(&obj)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteAckOKJSON(w)
}

func refreshNodesInfo(inst *model.Instance) ([]model.ESNodeInfo, error) {
	oldNodesInfo, err := getNodesInfoFromES(inst)
	if err != nil {
		return nil, fmt.Errorf("get elasticsearch nodes info from es error: %w", err)
	}
	nodesInfo, err := GetElasticsearchNodes(context.Background(), inst)
	if err != nil {
		log.Errorf("get elasticsearch nodes error: %v", err)
		//return nodes info from es after failed to get nodes info from agent
		var nodes = []model.ESNodeInfo{}
		for _, nodeInfo := range oldNodesInfo {
			nodes = append(nodes, *nodeInfo)
		}
		return nodes, nil
	}

	oldPids := map[int]struct{}{}
	var resultNodes []model.ESNodeInfo
	for _, node := range nodesInfo {
		oldNode := getNodeByPidOrUUID(oldNodesInfo, node.ProcessInfo.PID, node.NodeUUID, node.HttpPort)
		node.AgentID = inst.ID
		if oldNode != nil {
			node.ID = oldNode.ID
			//keep old validate info
			if node.ClusterUuid == "" && oldNode.ClusterUuid != "" {
				node = *oldNode
			}
			oldPids[oldNode.ProcessInfo.PID] = struct{}{}
		} else {
			node.ID = util.GetUUID()
		}
		if node.ClusterUuid != "" {
			if oldNode != nil && oldNode.ClusterID != "" {
				node.ClusterID = oldNode.ClusterID
			}
		}

		node.Status = "online"
		err = orm.Save(nil, node)
		if err != nil {
			log.Error("save node info error: ", err)
		}
		resultNodes = append(resultNodes, node)
	}
	for k, node := range oldNodesInfo {
		if _, ok := oldPids[k]; !ok {
			//auto delete not associated cluster
			if node.ClusterID == "" {
				log.Info("delete node with pid: ", node.ProcessInfo.PID)
				err = orm.Delete(nil, node)
				if err != nil {
					log.Error("delete node info error: ", err)
				}
				continue
			}
			node.Status = "offline"
			err = orm.Save(nil, node)
			if err != nil {
				log.Error("save node info error: ", err)
			}
			resultNodes = append(resultNodes, *node)
		}
	}
	return resultNodes, nil
}

func GetElasticsearchNodes(ctx context.Context, instance *model.Instance) ([]model.ESNodeInfo, error) {
	req := &util.Request{
		Method:  http.MethodGet,
		Path:    "/elasticsearch/nodes/_discovery",
		Context: ctx,
	}
	resBody := []model.ESNodeInfo{}
	err := doRequest(instance, req, &resBody)
	if err != nil {
		return nil, err
	}

	return resBody, nil
}

func AuthESNode(ctx context.Context, agentBaseURL string, cfg elastic.ElasticsearchConfig) (*model.ESNodeInfo, error) {
	reqBody, err := util.ToJSONBytes(cfg)
	if err != nil {
		return nil, err
	}
	req := &util.Request{
		Method:  http.MethodPost,
		Path:    "/elasticsearch/_auth",
		Context: ctx,
		Body:    reqBody,
	}
	resBody := &model.ESNodeInfo{}
	err = DoRequest(req, resBody)
	if err != nil {
		return nil, err
	}
	return resBody, nil
}


func getNodeByPidOrUUID(nodes map[int]*model.ESNodeInfo, pid int, uuid string, port string) *model.ESNodeInfo {
	if nodes[pid] != nil {
		return nodes[pid]
	}
	for _, node := range nodes {
		if node.NodeUUID != "" && node.NodeUUID == uuid {
			return node
		}
	}
	return nil
}

func getNodesInfoFromES(instance *model.Instance) (map[int]*model.ESNodeInfo, error) {
	query := util.MapStr{
		"size": 1000,
		"query": util.MapStr{
			"term": util.MapStr{
				"agent_id": util.MapStr{
					"value": instance.ID,
				},
			},
		},
	}
	q := orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}

	err, result := orm.Search(model.ESNodeInfo{}, &q)
	if err != nil {
		return nil, err
	}
	nodesInfo := map[int]*model.ESNodeInfo{}
	for _, row := range result.Result {
		node := model.ESNodeInfo{}
		buf := util.MustToJSONBytes(row)
		util.MustFromJSONBytes(buf, &node)
		nodesInfo[node.ProcessInfo.PID] = &node
	}
	return nodesInfo, nil
}

func getUnAssociateNodes() (map[string][]model.ESNodeInfo, error) {
	query := util.MapStr{
		"size": 3000,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must_not": []util.MapStr{
					{
						"exists": util.MapStr{
							"field": "cluster_id",
						},
					},
				},
			},
		},
	}
	q := orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}

	err, result := orm.Search(model.ESNodeInfo{}, &q)
	if err != nil {
		return nil, err
	}
	nodesInfo := map[string][]model.ESNodeInfo{}
	for _, row := range result.Result {
		node := model.ESNodeInfo{}
		buf := util.MustToJSONBytes(row)
		util.MustFromJSONBytes(buf, &node)
		nodesInfo[node.AgentID] = append(nodesInfo[node.AgentID], node)
	}
	return nodesInfo, nil
}


func GetElasticLogFiles(ctx context.Context, instance *model.Instance, logsPath string) (interface{}, error) {

	reqBody := util.MustToJSONBytes(util.MapStr{
		"logs_path": logsPath,
	})
	req := &util.Request{
		Method:  http.MethodPost,
		Path:    "/elasticsearch/logs/_list",
		Context: ctx,
		Body:    reqBody,
	}
	resBody := map[string]interface{}{}
	err := doRequest(instance,req, &resBody)
	if err != nil {
		return nil, err
	}
	if resBody["success"] != true {
		return nil, fmt.Errorf("get elasticsearch log files error: %v", resBody)
	}
	return resBody["result"], nil
}

func GetElasticLogFileContent(ctx context.Context, instance *model.Instance, body interface{}) (interface{}, error) {
	req := &util.Request{
		Method:  http.MethodPost,
		Path:    "/elasticsearch/logs/_read",
		Context: ctx,
		Body:    util.MustToJSONBytes(body),
	}
	resBody := map[string]interface{}{}
	err := doRequest(instance,req, &resBody)
	if err != nil {
		return nil, err
	}
	if resBody["success"] != true {
		return nil, fmt.Errorf("get elasticsearch log files error: %v", resBody["error"])
	}
	var hasMore bool
	if v, ok := resBody["EOF"].(bool); ok && !v {
		hasMore = true
	}
	return map[string]interface{}{
		"lines":    resBody["result"],
		"has_more": hasMore,
	}, nil
}


func (h *APIHandler) getLogFilesByNode(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	nodeID := ps.MustGetParameter("node_id")
	inst, node, err := getAgentByNodeID(nodeID)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if inst == nil {
		log.Error(fmt.Sprintf("can not find agent by node [%s]", nodeID))
		h.WriteJSON(w, util.MapStr{
			"success": false,
			"reason": "AGENT_NOT_FOUND",
		}, http.StatusOK)
		return
	}
	logFiles, err := GetElasticLogFiles(nil, inst, node.Path.Logs)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"success": true,
		"log_files": logFiles,
	}, http.StatusOK)
}

func (h *APIHandler) getLogFileContent(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	nodeID := ps.MustGetParameter("node_id")
	inst, node, err := getAgentByNodeID(nodeID)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if inst == nil {
		h.WriteError(w, fmt.Sprintf("can not find agent by node [%s]", nodeID), http.StatusInternalServerError)
		return
	}
	reqBody := struct {
		FileName string `json:"file_name"`
		LogsPath string `json:"logs_path"`
		Offset int `json:"offset"`
		Lines int `json:"lines"`
		StartLineNumber int64 `json:"start_line_number"`
	}{}
	err = h.DecodeJSON(req, &reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	reqBody.LogsPath = node.Path.Logs
	res, err := GetElasticLogFileContent(nil, inst, reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	h.WriteJSON(w, res, http.StatusOK)
}

func getAgentByNodeID(nodeID string) (*model.Instance, *model.ESNodeInfo, error){
	queryDsl := util.MapStr{
		"size":1,
		"query": util.MapStr{
			"term": util.MapStr{
				"node_uuid": util.MapStr{
					"value": nodeID,
				},
			},
		},
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(queryDsl),
	}
	err, result := orm.Search(model.ESNodeInfo{}, q)
	if err != nil {
		return nil,nil, err
	}
	if len(result.Result) > 0 {
		buf := util.MustToJSONBytes(result.Result[0])
		v := &model.ESNodeInfo{}
		err = util.FromJSONBytes(buf, v)
		inst := &model.Instance{}
		inst.ID = v.AgentID
		_, err = orm.Get(inst)
		if err != nil {
			return nil, v, err
		}
		if inst.Name == "" {
			return nil, v, nil
		}
		return inst, v, nil
	}
	return nil, nil, nil
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
)

func (h *APIHandler) getLogFilesByNode(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	nodeID := ps.MustGetParameter("node_id")
	inst, err := getAgentByNodeID(nodeID)
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
	sm := agent.GetStateManager()
	logFiles, err := sm.GetAgentClient().GetElasticLogFiles(nil, inst.GetEndpoint(), inst.ID, nodeID)
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
	inst, err := getAgentByNodeID(nodeID)
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
		Offset int `json:"offset"`
		Lines int `json:"lines"`
		NodeID string `json:"node_id"`
	}{}
	err = h.DecodeJSON(req, &reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	reqBody.NodeID = nodeID
	sm := agent.GetStateManager()
	res, err := sm.GetAgentClient().GetElasticLogFileContent(nil, inst.GetEndpoint(), inst.ID, reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	h.WriteJSON(w, res, http.StatusOK)
}

func getAgentByNodeID(nodeID string) (*agent.Instance, error){
	queryDsl := util.MapStr{
		"size":1,
		"query": util.MapStr{
			"term": util.MapStr{
				"clusters.nodes.uuid": util.MapStr{
					"value": nodeID,
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(queryDsl),
	}
	err, result := orm.Search(agent.Instance{}, q)
	if err != nil {
		return nil, err
	}
	if len(result.Result) > 0 {
		buf := util.MustToJSONBytes(result.Result[0])
		inst := &agent.Instance{}
		err = util.FromJSONBytes(buf, inst)
		return inst, err
	}
	return nil, nil
}

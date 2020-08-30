/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net/http"
)

type ClusterAPI struct {
	api.Handler
}

func InitAPI() {

	cluster := ClusterAPI{}

	api.HandleAPIFunc("/_cluster/health", cluster.clusterInfo)
	api.HandleAPIFunc("/_cluster/node/_leave", cluster.handleLeave)
	//apihandler.HandleAPIFunc("/_cluster/node/_leave", s.handleLeave)
	//
	//apihandler.HandleAPIFunc("/cache", s.handleKeyRequest)
	//apihandler.HandleAPIFunc("/cache/", s.handleKeyRequest)
}

func (handler ClusterAPI) clusterInfo(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{}
	stats["cluster_name"] = global.Env().SystemConfig.ClusterConfig.Name
	stats["status"] = GetRaftStatus()
	stats["number_of_minimum_nodes"] = global.Env().SystemConfig.ClusterConfig.MinimumNodes
	nodes := GetNodes()
	stats["number_of_nodes"] = len(nodes)
	stats["timed_out"] = false
	stats["leader_node"] = GetLeader()
	stats["nodes"] = nodes

	stats["local"] = util.MapStr{
		"local_peers":        GetLocalPeers(),
		"local_active_peers": GetLocalActivePeersCount(),
		"stats":              GetStats(),
	}

	//TODO local node stats
	//stats["cluster_meta"] = GetClusterReadonlyMetadata()
	//stats["number_of_nodes"] = GetLocalActivePeersCount()
	//stats["stats"] = GetStats()
	//stats["allow_self_promote"] = (global.Env().SystemConfig.ClusterConfig.MinimumNodes<=1&&len(global.Env().SystemConfig.ClusterConfig.GetSeeds())==0)

	b, _ := json.MarshalIndent(stats, "", "  ")
	w.Write(b)
}

func (handler ClusterAPI) handleLeave(w http.ResponseWriter, r *http.Request) {
	node := Node{}
	err := handler.DecodeJSON(r, &node)
	if err != nil {
		handler.Error(w, err)
	}

	log.Error("receive node leave request, ", node)

	getRaft().Leave(node.RPCEndpoint, node.RPCEndpoint)

}

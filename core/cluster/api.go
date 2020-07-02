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
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"

	"encoding/json"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
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
	stats["number_of_nodes"] = GetLocalActivePeersCount()
	stats["local_meta"] = GetLocalPeers()
	stats["cluster_meta"] = GetClusterReadonlyMetadata()
	stats["number_of_minimum_nodes"] = global.Env().SystemConfig.ClusterConfig.MinimumNodes
	stats["timed_out"] = false
	stats["raft"] = util.MapStr{
		"leader": GetLeader(),
		"seeds":  GetPeers(),
		"stats":  GetStats(),
	}

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

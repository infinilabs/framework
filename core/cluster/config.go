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
	"infini.sh/framework/core/util"
	"net"
)

type Request struct {
	NodeType string `json:"type,omitempty"`
	Node     Node   `json:"node,omitempty"`
	FromNode Node   `json:"from,omitempty"`
}

type Node struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	ClusterName string `json:"cluster,omitempty"`
	Token       string `json:"token,omitempty"`

	//RaftEndpoint string `json:"raft_endpoint,omitempty"`
	APIEndpoint string `json:"api_endpoint,omitempty"`
	RPCEndpoint string `json:"rpc_endpoint,omitempty"`

	Active bool `json:"active"`

	StartTime int64 `json:"start_time,omitempty"`

	rpcAddr *net.TCPAddr
	apiAddr *net.TCPAddr
}

func (v *Node) GetRPCAddr() *net.TCPAddr {
	if v.rpcAddr == nil {
		v.rpcAddr = util.GetAddress(v.RPCEndpoint)
	}
	return v.rpcAddr
}

type Command struct {
	Op    string `json:"op,omitempty,omitempty"`
	Key   string `json:"key,omitempty,omitempty"`
	Value string `json:"value,omitempty,omitempty"`
}

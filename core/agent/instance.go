/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"fmt"
	"time"
)

type Instance struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Created time.Time `json:"created,omitempty" elastic_mapping:"created: { type: date }"`
	Updated time.Time `json:"updated,omitempty" elastic_mapping:"updated: { type: date }"`
	Schema      string `json:"schema,omitempty" elastic_mapping:"Schema: { type: keyword }"`
	Port uint `json:"port,omitempty" elastic_mapping:"port: { type: keyword }"`
	IPS []string `json:"ips,omitempty" elastic_mapping:"ips: { type: keyword }"`
	RemoteHost string `json:"remote_host" elastic_mapping:"remote_host: { type: keyword }"`
	Version map[string]interface{} `json:"version,omitempty" elastic_mapping:"version: { type: object }"`
	Clusters []ESCluster `json:"clusters,omitempty" elastic_mapping:"clusters: { type: object }"`
	Tags [] string `json:"tags,omitempty" elastic_mapping:"tags: { type: keyword }"`
	Status string `json:"status,omitempty" elastic_mapping:"status: { type: keyword }"`
	Timestamp time.Time `json:"-"`
}

func (inst *Instance) GetEndpoint() string{
	return fmt.Sprintf("%s://%s:%d", inst.Schema, inst.RemoteHost, inst.Port)
}

type ESCluster struct {
	ClusterUUID string `json:"cluster_uuid,omitempty" elastic_mapping:"cluster_uuid: { type: keyword }"`
	ClusterID string   `json:"cluster_id,omitempty" elastic_mapping:"cluster_id: { type: keyword }"`
	ClusterName string `json:"cluster_name,omitempty" elastic_mapping:"cluster_name: { type: keyword }"`
	Nodes  []string `json:"nodes,omitempty" elastic_mapping:"cluster_name: { type: keyword }"`
	TaskOwner bool `json:"task_owner" elastic_mapping:"task_owner: { type: keyword }"`
	TaskNodeID string `json:"task_node_id" elastic_mapping:"task_node_id: { type: keyword }"`
	BasicAuth *BasicAuth `json:"basic_auth,omitempty" elastic_mapping:"basic_auth: { type: keyword }"`
}

type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
}
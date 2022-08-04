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
	Tags [] string `json:"tags,omitempty" elastic_mapping:"clusters: { type: keyword }"`
	Status string `json:"status,omitempty" elastic_mapping:"status: { type: keyword }"`
	Timestamp time.Time `json:"-"`
}

func (inst *Instance) GetEndpoint() string{
	return fmt.Sprintf("%s://%s:%d", inst.Schema, inst.RemoteHost, inst.Port)
}

type ESCluster struct {
	ClusterUUID string `json:"cluster_uuid,omitempty"`
	ClusterID string   `json:"cluster_id,omitempty"`
	ClusterName string `json:"cluster_name,omitempty"`
	Nodes  []string `json:"nodes,omitempty"`
	TaskOwner bool `json:"task_owner"`
	BasicAuth *BasicAuth `json:"basic_auth,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
}
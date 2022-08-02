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
	Schema      string `json:"schema"`
	Host string `json:"host"`
	Version map[string]interface{} `json:"version,omitempty" elastic_mapping:"version: { type: object }"`
	Clusters []ESCluster `json:"clusters"`
	Tags [] string `json:"tags,omitempty"`
	Status string `json:"status"`
	Timestamp time.Time `json:"-"`
}

func (inst *Instance) GetEndpoint() string{
	return fmt.Sprintf("%s://%s", inst.Schema, inst.Host)
}

type ESCluster struct {
	ClusterUUID string `json:"cluster_uuid"`
	ClusterID string   `json:"cluster_id"`
	Nodes  []string `json:"nodes"`
	TaskOwner bool `json:"task_owner"`
	BasicAuth BasicAuth `json:"basic_auth"`
}

type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
}
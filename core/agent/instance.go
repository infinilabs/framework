/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"fmt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
	"time"
)

type Instance struct {
	orm.ORMObjectBase
	Name        string                 `json:"name,omitempty" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Endpoint    string                 `json:"endpoint,omitempty" elastic_mapping:"endpoint: { type: keyword }"`
	Version     map[string]interface{} `json:"version,omitempty" elastic_mapping:"version: { type: object }"`
	BasicAuth   BasicAuth        `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`
	Owner       string                 `json:"owner,omitempty" config:"owner" elastic_mapping:"owner:{type:keyword}"`
	Tags        []string               `json:"tags,omitempty"`
	Description string                 `json:"description,omitempty" config:"description" elastic_mapping:"description:{type:keyword}"`

	IPS        []string               `json:"ips,omitempty" elastic_mapping:"ips: { type: keyword,copy_to:search_text }"`
	MajorIP    string                 `json:"major_ip,omitempty" elastic_mapping:"major_ip: { type: keyword }"`
	Status     string                 `json:"status,omitempty" elastic_mapping:"status: { type: keyword, copy_to:search_text }"`
	SearchText string                 `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
	Host       HostInfo               `json:"host" elastic_mapping:"host: { type: object }"`
}

type HostInfo struct {
	Name     string        `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	OS       OSInfo        `json:"os" elastic_mapping:"os: { type: object }"`
	Hardware *HardwareInfo `json:"hardware,omitempty" elastic_mapping:"hardware: { type: object }"`
}

type HardwareInfo struct {
	Memory    interface{} `json:"memory,omitempty" elastic_mapping:"name: { type: object }"`
	Processor interface{} `json:"processor,omitempty" elastic_mapping:"processor: { type: object }"`
	Disk      interface{} `json:"disk,omitempty" elastic_mapping:"disk: { type: object }"`
}
type OSInfo struct {
	Name    string `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	Arch    string `json:"arch,omitempty" elastic_mapping:"arch: { type: keyword }"`
	Version string `json:"version,omitempty" elastic_mapping:"version: { type: keyword }"`
}

func (inst *Instance) GetEndpoint() string {
	return inst.Endpoint
}

func (inst *Instance) GetVersion() (map[string]interface{}, error) {
	req := &util.Request{
		Method:  http.MethodGet,
		Url:     fmt.Sprintf("%s/_framework/api/_info", inst.GetEndpoint()),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 3)
	defer cancel()
	req.Context = ctx
	result, err := util.ExecuteRequest(req)
	if err != nil {
		return nil, err
	}
	if result.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("unknow agent version")
	}
	res := map[string]interface{}{}
	err = util.FromJSONBytes(result.Body, &res)
	if err != nil {
		return nil, err
	}
	if v, ok := res["version"].(map[string]interface{}); ok {
		return v, nil
	}
	return nil, fmt.Errorf("unknow agent version")
}

type ShortState struct {
	ClusterMetricTask ClusterMetricTaskState
	NodeMetricTask    NodeMetricTaskState
}

type ClusterMetricTaskState struct {
	AgentID  string
	NodeUUID string
}

type NodeMetricTaskState struct {
	AgentID string
	Nodes   []string
}


type BasicAuth struct {
	Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
	Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
}

const (
	KVInstanceInfo   string = "agent_instance_info"
	KVInstanceBucket        = "agent_bucket"
)

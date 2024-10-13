/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package model

import (
	"context"
	"fmt"
	"net/http"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

type NetworkInfo struct {
	IP      []string `json:"ip,omitempty" elastic_mapping:"ip: { type: keyword,copy_to:search_text }"`
	MajorIP string   `json:"major_ip,omitempty" elastic_mapping:"major_ip: { type: keyword }"`
}

type Instance struct {
	orm.ORMObjectBase

	Name string `json:"name,omitempty" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`

	//application information
	Application env.Application `json:"application,omitempty" elastic_mapping:"application: { type: object }"`

	BasicAuth *BasicAuth `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`

	Labels map[string]string `json:"labels,omitempty" elastic_mapping:"labels:{type:object}"`
	Tags   []string          `json:"tags,omitempty"`

	//user can pass
	Description string `json:"description,omitempty" config:"description" elastic_mapping:"description:{type:keyword}"`

	Endpoint string `json:"endpoint,omitempty" elastic_mapping:"endpoint: { type: keyword }"` //API endpoint

	Host *HostInfo `json:"host,omitempty" elastic_mapping:"host: { type: object }"`

	Network  NetworkInfo   `json:"network,omitempty" elastic_mapping:"network: { type: object }"`
	Services []ServiceInfo `json:"services,omitempty" elastic_mapping:"services: { type: object }"`
	Status   string        `json:"status,omitempty" elastic_mapping:"status: { type: keyword, copy_to:search_text }"`

	//SearchText string   `json:"search_text,omitempty" elastic_mapping:"search_text:{type:text,index_prefixes:{},index_phrases:true, analyzer:suggest_text_search }"`
}

type ServiceInfo struct {
	Name     string `json:"name,omitempty" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Endpoint string `json:"endpoint,omitempty" elastic_mapping:"endpoint: { type: keyword }"`
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
	Name         string `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	Architecture string `json:"architecture,omitempty" elastic_mapping:"architecture: { type: keyword }"`
	Version      string `json:"version,omitempty" elastic_mapping:"version: { type: keyword }"`
}

func (inst *Instance) GetEndpoint() string {
	return inst.Endpoint
}

func (inst *Instance) GetVersion() (map[string]interface{}, error) {
	req := &util.Request{
		Method: http.MethodGet,
		Url:    fmt.Sprintf("%s/_info", inst.GetEndpoint()),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
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

func GetInstanceInfo() Instance {
	instance := Instance{}
	instance.ID = global.Env().SystemConfig.NodeConfig.ID
	instance.Name = global.Env().SystemConfig.NodeConfig.Name
	instance.Application = global.Env().GetApplicationInfo()

	instance.Labels = global.Env().SystemConfig.NodeConfig.Labels
	instance.Tags = global.Env().SystemConfig.NodeConfig.Tags

	_, publicIP, _, _ := util.GetPublishNetworkDeviceInfo(global.Env().SystemConfig.NodeConfig.MajorIpPattern)

	instance.Endpoint = global.Env().SystemConfig.APIConfig.GetEndpoint()

	ips := util.GetLocalIPs()
	if len(ips) > 0 {
		log.Infof("majorIP: %s,local ips: %v", publicIP, util.JoinArray(ips, ", "))
	}

	instance.Network = NetworkInfo{
		IP:      ips,
		MajorIP: publicIP,
	}
	hostInfo := &HostInfo{}
	hostInfo.Name, _, hostInfo.OS.Name, _, hostInfo.OS.Version, hostInfo.OS.Architecture, _ = host.GetOSInfo()
	instance.Host = hostInfo

	return instance
}

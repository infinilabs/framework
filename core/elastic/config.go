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

package elastic

import (
	"fmt"
	url2 "net/url"
	"strings"
	"time"
)

var apis = map[string]API{}
var cfgs = map[string]ElasticsearchConfig{}
var metas = map[string]*ElasticsearchMetadata{}

func RegisterInstance(elastic string, cfg ElasticsearchConfig, handler API) {
	if apis == nil {
		apis = map[string]API{}
	}
	if cfgs == nil {
		cfgs = map[string]ElasticsearchConfig{}
	}
	if metas == nil {
		metas = map[string]*ElasticsearchMetadata{}
	}
	apis[elastic] = handler
	cfgs[elastic] = cfg

}

type ElasticsearchMetadata struct {
	NodesTopologyVersion int
	IndicesChanged       bool
	Nodes                map[string]NodesInfo
	Indices              map[string]IndexInfo
	PrimaryShards        map[string]ShardInfo
	Aliases              map[string]AliasInfo
}

func (meta *ElasticsearchMetadata) GetPrimaryShardInfo(index string, shardID int) *ShardInfo {
	info, ok := meta.PrimaryShards[fmt.Sprintf("%v:%v", index, shardID)]
	if ok {
		return &info
	}
	return nil
}

func (meta *ElasticsearchMetadata) GetNodeInfo(nodeID string) *NodesInfo {
	info, ok := meta.Nodes[nodeID]
	if ok {
		return &info
	}
	return nil
}

// ElasticsearchConfig contains common settings for elasticsearch
type ElasticsearchConfig struct {
	ID        string   `json:"-" index:"id"`
	Name      string   `json:"name,omitempty" config:"name" elastic_mapping:"endpoint:{type:keyword}"`
	Description string    `json:"description,omitempty" elastic_mapping:"description:{type:text}"`
	Enabled   bool     `json:"enabled,omitempty" config:"enabled" elastic_mapping:"enabled:{type:boolean}"`
	HttpProxy string   `json:"http_proxy,omitempty" config:"http_proxy"`
	Endpoint  string   `json:"endpoint,omitempty" config:"endpoint" elastic_mapping:"endpoint:{type:keyword}"`
	//Endpoints []string `config:"endpoints"`
	Version   string   `json:"version,omitempty" config:"version"`

	BasicAuth *struct {
		Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
		Password string `json:"password,omitempty" config:"password" elastic_mapping:"username:{type:keyword}"`
	} `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`

	Discovery struct {
		Enabled bool     `json:"enabled,omitempty" config:"enabled"`
		Modules []string `json:"module,omitempty" config:"module"`
		Refresh struct {
			Enabled  bool   `json:"enabled,omitempty" config:"enabled"`
			Interval string `json:"interval,omitempty" config:"interval"`
		} `json:"refresh,omitempty" config:"refresh"`
	} `json:"discovery,omitempty" config:"discovery"`

	Order       int       `json:"order,omitempty" elastic_mapping:"order:{type:integer}"`
	Created     time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
	Updated     time.Time `json:"updated,omitempty" elastic_mapping:"updated:{type:date}"`
}

//format: host:port
func (config *ElasticsearchConfig) GetHost() string {
	u, err := url2.Parse(config.Endpoint)
	if err != nil {
		panic(err)
	}
	return u.Host
}

func (config *ElasticsearchConfig) IsTLS() bool {
	if strings.Contains(config.Endpoint, "https") {
		return true
	} else {
		return false
	}
}

func GetConfig(k string) ElasticsearchConfig {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}
	v, ok := cfgs[k]
	if !ok {
		panic(fmt.Sprintf("elasticsearch config [%v] was not found", k))
	}
	return v
}

func GetMetadata(k string) *ElasticsearchMetadata {
	if k == "" {
		panic(fmt.Errorf("elasticsearch metata undefined"))
	}
	v, _ := metas[k]
	//if !ok {
	//	panic(fmt.Sprintf("elasticsearch metata [%v] was not found", k))
	//}
	return v
}

func GetClient(k string) API {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}

	v, ok := apis[k]
	if ok {
		return v
	}

	////try to load config,and init client
	//cfg:=GetConfig(k)

	panic(fmt.Sprintf("elasticsearch client [%v] was not found", k))
}

func GetAllConfigs() map[string]ElasticsearchConfig {
	return cfgs
}
func SetMetadata(k string, v *ElasticsearchMetadata) {
	metas[k] = v
}

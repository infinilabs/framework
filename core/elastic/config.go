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
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
)

var apis = map[string]API{}
var cfgs = map[string]*ElasticsearchConfig{}
var metas = map[string]*ElasticsearchMetadata{}
var lock = sync.RWMutex{}

func RegisterInstance(elastic string, cfg ElasticsearchConfig, handler API) {

	lock.Lock()
	defer lock.Unlock()

	if apis == nil {
		apis = map[string]API{}
	}
	if cfgs == nil {
		cfgs = map[string]*ElasticsearchConfig{}
	}
	if metas == nil {
		metas = map[string]*ElasticsearchMetadata{}
	}
	apis[elastic] = handler
	cfgs[elastic] = &cfg

}

type ElasticsearchMetadata struct {
	NodesTopologyVersion int
	IndicesChanged       bool
	Nodes                map[string]NodesInfo
	Indices              map[string]IndexInfo
	PrimaryShards        map[string]map[string]ShardInfo
	Aliases              map[string]AliasInfo
}

func (meta *ElasticsearchMetadata) GetPrimaryShardInfo(index string, shardID int) *ShardInfo {
	indexMap, ok := meta.PrimaryShards[index]
	if ok {
		shardInfo, ok := indexMap[util.IntToString(shardID)]
		if ok {
			return &shardInfo
		}
	}
	return nil
}

func (meta *ElasticsearchMetadata) GetActiveNodeInfo() *NodesInfo {
	for _, v := range meta.Nodes {
		return &v
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
	Source      string `json:"-"`
	ID          string `json:"-" index:"id"`
	Name        string `json:"name,omitempty" config:"name" elastic_mapping:"endpoint:{type:keyword}"`
	Description string `json:"description,omitempty" elastic_mapping:"description:{type:text}"`
	Enabled     bool   `json:"enabled,omitempty" config:"enabled" elastic_mapping:"enabled:{type:boolean}"`
	Monitored   bool   `json:"monitored,omitempty" config:"monitored" elastic_mapping:"monitored:{type:boolean}"`
	HttpProxy   string `json:"http_proxy,omitempty" config:"http_proxy"`
	Endpoint    string `json:"endpoint,omitempty" config:"endpoint" elastic_mapping:"endpoint:{type:keyword}"`
	Version string `json:"version,omitempty" config:"version"`
	ClientMode string `json:"client_mode,omitempty" config:"client_mode"`

	BasicAuth *struct {
		Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
		Password string `json:"password,omitempty" config:"password" elastic_mapping:"username:{type:keyword}"`
	} `config:"basic_auth" json:"basic_auth,omitempty" elastic_mapping:"basic_auth:{type:object}"`

	TrafficControl *struct {
		MaxBytesPerNode int `json:"max_bytes_per_node,omitempty" config:"max_bytes_per_node" elastic_mapping:"max_bytes_per_node:{type:keyword}"`
		MaxQpsPerNode   int `json:"max_qps_per_node,omitempty" config:"max_qps_per_node" elastic_mapping:"max_qps_per_node:{type:keyword}"`
	} `config:"traffic_control" json:"traffic_control,omitempty" elastic_mapping:"traffic_control:{type:object}"`

	Discovery struct {
		Enabled bool     `json:"enabled,omitempty" config:"enabled"`
		Modules []string `json:"module,omitempty" config:"module"`
		Refresh struct {
			Enabled  bool   `json:"enabled,omitempty" config:"enabled"`
			Interval string `json:"interval,omitempty" config:"interval"`
		} `json:"refresh,omitempty" config:"refresh"`
	} `json:"discovery,omitempty" config:"discovery"`

	Order   int       `json:"order,omitempty" elastic_mapping:"order:{type:integer}"`
	Created time.Time `json:"created,omitempty" elastic_mapping:"created:{type:date}"`
	Updated time.Time `json:"updated,omitempty" elastic_mapping:"updated:{type:date}"`

	clusterFailureTicket int
	clusterOnFailure     bool
	clusterAvailable     bool
	schema string

	configLock sync.RWMutex
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

func (config *ElasticsearchConfig) Schema() string {
	if config.schema!=""{
		return config.schema
	}
	if strings.Contains(config.Endpoint, "https") {
		config.schema= "https"
	} else {
		config.schema= "http"
	}
	return config.schema
}

func GetConfig(k string) *ElasticsearchConfig {
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

func GetAllConfigs() map[string]*ElasticsearchConfig {
	return cfgs
}
func SetMetadata(k string, v *ElasticsearchMetadata) {
	metas[k] = v
}

func (config *ElasticsearchConfig) ReportFailure() bool {
	config.configLock.Lock()
	defer config.configLock.Unlock()

	if !config.clusterAvailable {
		return true
	}

	config.clusterOnFailure = true
	if rate.GetRateLimiter("cluster_failure", config.Name, 1, 1, time.Second*1).Allow() {
		log.Debug("vote failure ticket++")
		config.clusterFailureTicket++
		if config.clusterFailureTicket >= 10 {
			log.Debug("enough failure ticket, mark it down")
			config.clusterFailureTicket = 10
			config.clusterAvailable = false
			config.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is not available", config.Name)
			return true
		}
	}
	return false
}

func (config *ElasticsearchConfig) IsAvailable() bool {
	if !config.Enabled {
		return false
	}

	config.configLock.RLock()
	defer config.configLock.RUnlock()

	return config.clusterAvailable
}

func (config *ElasticsearchConfig) Init() {
	config.clusterAvailable = true
	config.clusterOnFailure = false
	config.clusterFailureTicket = 0
}

func (config *ElasticsearchConfig) ReportSuccess() {
	if !config.Enabled {
		return
	}

	if config.clusterAvailable {
		return
	}

	config.configLock.Lock()
	defer config.configLock.Unlock()

	if config.clusterOnFailure && !config.clusterAvailable {
		if rate.GetRateLimiter("cluster_recovery_health", config.Name, 1, 1, time.Second*1).Allow() {
			log.Debug("vote success ticket++")
			config.clusterOnFailure = false
			config.clusterAvailable = true
			config.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is coming back", config.Name)
		}
	}
}

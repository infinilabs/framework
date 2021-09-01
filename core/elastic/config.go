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

func RemoveInstance(elastic string){
	lock.Lock()
	defer lock.Unlock()
	if cfgs != nil {
		delete(cfgs, elastic)
	}
	if apis != nil {
		delete(apis, elastic)
	}
	if metas != nil {
		delete(metas, elastic)
	}

}

type ElasticsearchMetadata struct {
	Config *ElasticsearchConfig

	NodesTopologyVersion int
	IndicesChanged       bool
	Nodes                map[string]NodesInfo
	Indices              map[string]IndexInfo
	PrimaryShards        map[string]map[int]ShardInfo
	Aliases              map[string]AliasInfo
	HealthStatus string

	clusterFailureTicket int
	clusterOnFailure     bool
	clusterAvailable     bool
	lastSuccess time.Time
	configLock sync.RWMutex
}

func (meta *ElasticsearchMetadata) GetPrimaryShardInfo(index string, shardID int) *ShardInfo {
	indexMap, ok := meta.PrimaryShards[index]
	if ok {
		shardInfo, ok := indexMap[shardID]
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
	Name        string `json:"name,omitempty" config:"name" elastic_mapping:"name:{type:keyword,fields:{text: {type: text}}}"`
	Description string `json:"description,omitempty" elastic_mapping:"description:{type:text}"`
	Enabled     bool   `json:"enabled,omitempty" config:"enabled" elastic_mapping:"enabled:{type:boolean}"`
	Monitored   bool   `json:"monitored,omitempty" config:"monitored" elastic_mapping:"monitored:{type:boolean}"`
	HttpProxy   string `json:"http_proxy,omitempty" config:"http_proxy"`
	Endpoint    string `json:"endpoint,omitempty" config:"endpoint" elastic_mapping:"endpoint:{type:keyword}"`
	Version string `json:"version,omitempty" config:"version"`
	ClientMode string `json:"client_mode,omitempty" config:"client_mode"`

	BasicAuth *struct {
		Username string `json:"username,omitempty" config:"username" elastic_mapping:"username:{type:keyword}"`
		Password string `json:"password,omitempty" config:"password" elastic_mapping:"password:{type:keyword}"`
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

	Schema string `json:"schema,omitempty" elastic_mapping:"schema:{type:keyword}"`

	Host string `json:"host,omitempty" elastic_mapping:"host:{type:keyword}"`
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

func (config *ElasticsearchConfig) GetSchema() string {
	lock.RLock()
	defer lock.RUnlock()

	if config.Schema!=""{
		return config.Schema
	}
	if strings.Contains(config.Endpoint, "https") {
		config.Schema= "https"
	} else {
		config.Schema= "http"
	}
	return config.Schema
}

func GetConfig(k string) *ElasticsearchConfig {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}
	lock.RLock()
	defer lock.RUnlock()
	v, ok := cfgs[k]
	if !ok {
		panic(fmt.Sprintf("elasticsearch config [%v] was not found", k))
	}
	return v
}

func GetOrInitMetadata(cfg *ElasticsearchConfig) *ElasticsearchMetadata {
	v:=GetMetadata(cfg.ID)
	if v==nil{
		v=&ElasticsearchMetadata{Config: cfg}
		v.Init(false)
		SetMetadata(cfg.ID,v)
	}
	return v
}

func GetMetadata(k string) *ElasticsearchMetadata {
	if k == "" {
		panic(fmt.Errorf("elasticsearch metata undefined"))
	}

	lock.Lock()
	defer lock.Unlock()

	v, ok := metas[k]
	if !ok {
		log.Debug(fmt.Sprintf("elasticsearch metadata [%v] was not found", k))
		//panic(fmt.Sprintf("elasticsearch metadata [%v] was not found", k))
	}
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

func GetAllMetadata() map[string]*ElasticsearchMetadata {
	lock.Lock()
	defer lock.Unlock()

	return metas
}

func GetAllConfigs() map[string]*ElasticsearchConfig {
	return cfgs
}

func SetMetadata(k string, v *ElasticsearchMetadata) {

	lock.Lock()
	defer lock.Unlock()

	metas[k] = v
}

func (meta *ElasticsearchMetadata) ReportFailure() bool {
	meta.configLock.Lock()
	defer meta.configLock.Unlock()

	if !meta.clusterAvailable {
		return true
	}

	meta.clusterOnFailure = true
	if rate.GetRateLimiter("cluster_failure", meta.Config.ID, 1, 1, time.Second*1).Allow() {
		log.Debugf("vote failure ticket++ for elasticsearch [%v]",meta.Config.Name)
		meta.clusterFailureTicket++
		if (meta.clusterFailureTicket >= 10 && time.Since(meta.lastSuccess)>5*time.Second) ||time.Since(meta.lastSuccess)>10*time.Second{
			log.Debug("enough failure ticket for elasticsearch [%v], mark it down",meta.Config.Name)
			meta.clusterAvailable = false
			meta.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is not available", meta.Config.Name)
			return true
		}
	}
	return false
}

func (meta *ElasticsearchMetadata) IsAvailable() bool {
	if !meta.Config.Enabled {
		return false
	}

	meta.configLock.RLock()
	defer meta.configLock.RUnlock()

	return meta.clusterAvailable
}

func (meta *ElasticsearchMetadata) Init(health bool){
	meta.clusterAvailable = health
	meta.clusterOnFailure = !health
	meta.lastSuccess=time.Now()
	meta.clusterFailureTicket = 0
}

func (meta *ElasticsearchMetadata) ReportSuccess() {

	meta.lastSuccess=time.Now()

	if !meta.Config.Enabled {
		return
	}

	if meta.clusterAvailable {
		return
	}

	meta.configLock.Lock()
	defer meta.configLock.Unlock()

	if meta.clusterOnFailure && !meta.clusterAvailable {
		if rate.GetRateLimiter("cluster_recovery_health", meta.Config.ID, 1, 1, time.Second*1).Allow() {
			log.Debugf("vote success ticket++ for elasticsearch [%v]",meta.Config.Name)
			meta.clusterOnFailure = false
			meta.clusterAvailable = true
			meta.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is coming back", meta.Config.Name)
		}
	}
}

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

var apis = sync.Map{}
var cfgs = sync.Map{}

var metas = sync.Map{}

func RegisterInstance(elastic string, cfg ElasticsearchConfig, handler API) {
	apis.Store(elastic,handler)
	cfgs.Store(elastic,&cfg)
}

func RemoveInstance(elastic string){
	cfgs.Delete(elastic)
	apis.Delete(elastic)
	metas.Delete(elastic)
}

func (action *BulkActionMetadata)GetItem() *BulkIndexMetadata  {
	if action.Index!=nil{
		return action.Index
	}else if action.Delete!=nil{
		return action.Delete
	}else if action.Create!=nil{
		return action.Create
	}else{
		return action.Update
	}
}

func (meta *ElasticsearchMetadata) GetAvailableHost(endpoint string)string  {
	for x,v:=range meta.HostAvailableInfo {
		if v.Available && x!=endpoint{
			return x
		}
	}
	return meta.Config.GetHost()
}

func (meta *ElasticsearchMetadata) IsHostAvailable(endpoint string)bool  {
	info,ok:=meta.HostAvailableInfo[endpoint]
	if ok{
		return info.Available
	}
	log.Warnf("node info for [%v]  not found in meta",endpoint)
	return false
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
	v, ok := cfgs.Load(k)
	if !ok {
		panic(fmt.Sprintf("elasticsearch config [%v] was not found", k))
	}
	return v.(*ElasticsearchConfig)
}

func GetOrInitMetadata(cfg *ElasticsearchConfig) *ElasticsearchMetadata {
	v:=GetMetadata(cfg.ID)
	if v==nil{
		v=&ElasticsearchMetadata{Config: cfg}
		v.Init(true)
		SetMetadata(cfg.ID,v)
	}
	return v
}

func GetMetadata(k string) *ElasticsearchMetadata {
	if k == "" {
		panic(fmt.Errorf("elasticsearch metata undefined"))
	}

	v, ok := metas.Load(k)
	if !ok {
		log.Debug(fmt.Sprintf("elasticsearch metadata [%v] was not found", k))
		return nil
	}
	 x,ok:=v.(*ElasticsearchMetadata)
	 return x
}

func GetClient(k string) API {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}

	v, ok := apis.Load(k)
	if ok {
		f,ok:=v.(API)
		if ok{
			return f
		}
	}

	panic(fmt.Sprintf("elasticsearch client [%v] was not found", k))
}

func WalkMetadata(walkFunc func(key, value interface{}) bool){
	metas.Range(walkFunc)
}

func WalkConfigs(walkFunc func(key, value interface{})bool) {
	 cfgs.Range(walkFunc)
}

func SetMetadata(k string, v *ElasticsearchMetadata) {
	metas.Store(k,v)
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
			log.Debugf("enough failure ticket for elasticsearch [%v], mark it down",meta.Config.Name)
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
	meta.HostAvailableInfo = map[string]HostAvailableInfo{}
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
			log.Infof("elasticsearch [%v] is available now", meta.Config.Name)
		}
	}
}

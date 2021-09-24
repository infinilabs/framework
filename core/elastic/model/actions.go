/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package model

import (
	"infini.sh/framework/core/rate"
	url2 "net/url"
	log "github.com/cihub/seelog"
	"strings"
	"time"
)

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

func (meta *BulkActionMetadata)GetItem() *BulkIndexMetadata {
	if meta.Index!=nil{
		return meta.Index
	}else if meta.Delete!=nil{
		return meta.Delete
	}else if meta.Create!=nil{
		return meta.Create
	}else{
		return meta.Update
	}
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


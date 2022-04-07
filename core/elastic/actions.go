/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/rate"
	"strings"
	"time"
)

func (node *NodeAvailable) ReportFailure() {
	node.lastCheck = time.Now()

	node.configLock.Lock()
	defer node.configLock.Unlock()

	if !node.available {
		if !node.isDead{
			if time.Since(node.lastSuccess)>1*time.Hour{
				node.isDead=true
				log.Infof("node [%v] is dead, lost >1 hour: %v",node.Host,node.lastSuccess)
			}
		}
		return
	}

	if len(node.Host) == 0 {
		return
	}

	if rate.GetRateLimiter("node_failure", node.Host, 1, 1, time.Second*1).Allow() {
		log.Debugf("vote failure ticket++ for elasticsearch [%v]", node.Host)

		node.ticket++
		//if the target host is not available for 10s, mark it down
		if (node.ticket >= 10 && time.Since(node.lastSuccess) > 5*time.Second) || time.Since(node.lastSuccess) > 10*time.Second {
			log.Debugf("enough failure ticket for elasticsearch [%v], mark it down", node.Host)
			node.available = false
			node.ticket = 0
			log.Infof("node [%v] is not available", node.Host)
			return
		}
	}
	return
}

func (node *NodeAvailable) ReportSuccess() {

	node.lastSuccess = time.Now()
	node.lastCheck = time.Now()

	if node.available {
		return
	}

	node.configLock.Lock()
	defer node.configLock.Unlock()

	if  !node.available {
		if rate.GetRateLimiter("node_available", node.Host, 1, 1, time.Second*1).Allow() {
			log.Debugf("vote success ticket++ for elasticsearch [%v]", node.Host)
			node.isDead = false
			node.available = true
			node.ticket = 0
			log.Infof("node [%v] is available", node.Host)
		}
	}
}

func (node *NodeAvailable) LastSuccess() time.Time {
	return node.lastSuccess
}

func (node *NodeAvailable) LastCheck() time.Time {
	return node.lastCheck
}

func (node *NodeAvailable) FailureTickets() int {
	return node.ticket
}

func (node *NodeAvailable) IsAvailable() bool {
	return node.available
}

func (node *NodeAvailable) IsDead() bool {
	return node.isDead
}

func (meta *ElasticsearchMetadata) IsAvailable() bool {
	if meta.Config==nil||!meta.Config.Enabled {
		return false
	}

	meta.configLock.RLock()
	defer meta.configLock.RUnlock()

	return meta.clusterAvailable
}

func (meta *ElasticsearchMetadata) Init(health bool) {
	meta.clusterAvailable = health
	meta.clusterFailureTicket = 0
}

func (meta *BulkActionMetadata) GetItem() *BulkIndexMetadata {
	if meta.Index != nil {
		return meta.Index
	} else if meta.Delete != nil {
		return meta.Delete
	} else if meta.Create != nil {
		return meta.Create
	} else {
		return meta.Update
	}
}

func (meta *ElasticsearchMetadata) GetPrimaryShardInfo(index string, shardID string) (*IndexShardRouting,error) {


	table,err:=meta.GetIndexRoutingTable(index)
	if err!=nil{
		return nil,err
	}

	shards,ok:=table[shardID]
	if ok{
		for _,v:=range shards{
			if v.Primary{
				return &v,nil
			}
		}
	}


	return nil, errors.Errorf("primary shard info for shard [%v][%v] was not found",index,shardID)

}

func (meta *ElasticsearchMetadata) GetActiveNodeInfo() *NodesInfo {
	if meta.Nodes != nil {
		for _, v := range *meta.Nodes {
			return &v
		}
	}
	return nil
}

func (meta *ElasticsearchMetadata) GetNodeInfo(nodeID string) *NodesInfo {
	if meta.Nodes != nil {
		info, ok := (*meta.Nodes)[nodeID]
		if ok {
			return &info
		}
	}

	info,_:=GetClient(meta.Config.ID).GetNodeInfo(nodeID)

	return info
}

func (meta *ElasticsearchMetadata) GetActiveEndpoint() string {
	return fmt.Sprintf("%s://%s", meta.GetSchema(), meta.GetActiveHost())
}


func (meta *ElasticsearchMetadata) GetActivePreferredEndpoint(host string) string {
	available := IsHostAvailable(host)
	if !available {
		if meta.IsAvailable() {
			host = meta.GetActiveHost()
		} else {
			time.Sleep(1 * time.Second)
		}
	}
	return fmt.Sprintf("%s://%s", meta.GetSchema(), host)
}

func (meta *ElasticsearchMetadata) GetActiveHost() string {

	hosts := meta.GetSeedHosts()
	for _, v := range hosts {
		if IsHostAvailable(v) {
			return v
		}
	}
	if rate.GetRateLimiter("cluster_available", meta.Config.Name, 1, 1, time.Second*10).Allow() {
		log.Debug("no hosts available, choose: ", hosts[0])
	}
	meta.ReportFailure()
	return hosts[0]
}

func (meta *ElasticsearchMetadata) IsTLS() bool {
	return meta.GetSchema() == "https"
}

func (meta *ElasticsearchMetadata) GetSchema() string {
	if meta.Config.Schema != "" {
		return meta.Config.Schema
	}
	if meta.Config.Endpoint != "" {
		if strings.Contains(meta.Config.Endpoint, "https") {
			meta.Config.Schema = "https"
		} else {
			meta.Config.Schema = "http"
		}
		return meta.Config.Schema
	}
	if len(meta.Config.Endpoints) > 0 {
		for _, v := range meta.Config.Endpoints {
			if strings.Contains(v, "https") {
				meta.Config.Schema = "https"
			} else {
				meta.Config.Schema = "http"
			}
			return meta.Config.Schema
		}
	}

	if meta.Config.Schema == "" {
		meta.Config.Schema = "http"
	}

	return meta.Config.Schema
}

func (meta *ElasticsearchMetadata) ReportFailure() bool {
	log.Tracef("report failure for elasticsearch [%v]", meta.Config.Name)

	meta.configLock.Lock()
	defer meta.configLock.Unlock()

	if !meta.clusterAvailable {
		log.Tracef("elasticsearch [%v] is already in [%v] now, skip", meta.Config.Name, meta.clusterAvailable)
		return true
	}

	if rate.GetRateLimiter("cluster_failure", meta.Config.Name, 1, 1, time.Second*1).Allow() {
		log.Tracef("vote failure ticket++ for elasticsearch [%v] ticks:[%v], time:%v", meta.Config.Name, meta.clusterFailureTicket, time.Since(meta.lastSuccess) > 5*time.Second)

		meta.clusterFailureTicket++
		//if the target host is not available for 10s, mark it down
		if (meta.clusterFailureTicket >= 10 && time.Since(meta.lastSuccess) > 5*time.Second) || time.Since(meta.lastSuccess) > 10*time.Second {
			log.Debugf("enough failure ticket for elasticsearch [%v], mark it down", meta.Config.Name)
			meta.clusterAvailable = false
			meta.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is not available", meta.Config.Name)
			return true
		}
	}
	return false
}

func (meta *ElasticsearchMetadata) ReportSuccess() {

	meta.lastSuccess = time.Now()

	if meta.clusterAvailable {
		return
	}

	meta.configLock.Lock()
	defer meta.configLock.Unlock()

	if !meta.clusterAvailable {
		if rate.GetRateLimiter("cluster_available", meta.Config.Name, 1, 1, time.Second*1).Allow() {
			log.Tracef("vote success ticket++ for elasticsearch [%v]", meta.Config.Name)
			meta.clusterAvailable = true
			meta.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is available", meta.Config.Name)
		}
	}
}

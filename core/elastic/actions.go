// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"strings"
	"time"
)

func (node *NodeAvailable) ReportFailure() {
	node.lastCheck = time.Now()

	node.configLock.Lock()
	defer node.configLock.Unlock()

	if !node.available {
		if !node.isDead {
			if time.Since(node.lastSuccess) > 1*time.Hour {
				node.isDead = true
				log.Debugf("node [%v] is dead, lost >1 hour: %v", node.Host, node.lastSuccess)
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
		if (node.ticket >= 30 && time.Since(node.lastSuccess) > 10*time.Second) || time.Since(node.lastSuccess) > 60*time.Second {
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

	if !node.available {
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
	if meta.Config == nil || !meta.Config.Enabled {
		return false
	}
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

func (meta *ElasticsearchMetadata) GetPrimaryShardInfo(index string, shardID string) (*IndexShardRouting, error) {

	table, err := meta.GetIndexRoutingTable(index)
	if err != nil {
		return nil, err
	}

	shards, ok := table[shardID]
	if ok {
		for _, v := range shards {
			if v.Primary {
				return &v, nil
			}
		}
	}

	return nil, errors.Errorf("primary shard info for shard [%v][%v] was not found", index, shardID)

}

func (meta *ElasticsearchMetadata) GetNodeInfo(nodeID string) *NodesInfo {
	if meta.Nodes != nil {
		info, ok := (*meta.Nodes)[nodeID]
		if ok {
			return &info
		}
	}
	info, err := GetClient(meta.Config.ID).GetNodeInfo(nodeID)
	if err==nil{
		return info
	}
	if global.Env().IsDebug{
		log.Error(err)
	}
	return nil
}

func (meta *ElasticsearchMetadata) GetActiveEndpoint() string {
	return fmt.Sprintf("%s://%s", meta.GetSchema(), meta.GetActiveHost())
}

func (meta *ElasticsearchMetadata) GetActivePreferredSeedHost() string {
	hosts := meta.GetSeedHosts()
	if len(hosts) > 0 {
		for _, v := range hosts {
			if v != "" && IsHostAvailable(v) {
				return v
			}
		}
	}
	return meta.Config.Host
}

func (meta *ElasticsearchMetadata) GetActivePreferredSeedEndpoint() string {
	var endpoint string
	hosts := meta.GetSeedHosts()
	if len(hosts) > 0 {
		endpoint = meta.GetActivePreferredEndpoints(hosts)
	} else {
		endpoint = meta.GetActiveEndpoint()
	}
	if strings.TrimSpace(endpoint) == "" {
		return meta.Config.Endpoint
	}
	return endpoint
}

func (meta *ElasticsearchMetadata) GetActivePreferredEndpoint(host string) string {
	if strings.TrimSpace(host) == "" {
		return meta.Config.Endpoint
	}
	return meta.GetActivePreferredEndpoints([]string{host})
}

func (meta *ElasticsearchMetadata) GetActivePreferredEndpoints(hosts []string) string {
	if len(hosts) == 0 {
		panic(errors.New("hosts is empty"))
	}

	for _, v := range hosts {
		if v != "" && IsHostAvailable(v) {
			return fmt.Sprintf("%s://%s", meta.GetSchema(), v)
		}
	}

	return fmt.Sprintf("%s://%s", meta.GetSchema(), meta.GetActiveHost())
}

func (meta *ElasticsearchMetadata) PrepareEndpoint(host string) string {
	return fmt.Sprintf("%s://%s", meta.GetSchema(), host)
}

func (meta *ElasticsearchMetadata) GetActiveHosts() int {
	hash := hashset.New()
	hosts := meta.GetSeedHosts()
	for _, v := range hosts {
		if v != "" && IsHostAvailable(v) {
			hash.Add(v)
		}
	}
	if meta.Config.Discovery.Enabled {
		if meta.Nodes != nil {
			for _, v1 := range *meta.Nodes {
				v := v1.GetHttpPublishHost()
				if v != "" && IsHostAvailable(v) {
					//add to cache
					info, ok := GetHostAvailableInfo(v)
					if ok && info != nil {
						hash.Add(v)
					}
				}
			}
		}
	}

	return hash.Size()
}

func (meta *ElasticsearchMetadata) GetActiveHost() string {

	if meta.activeHost != nil {
		if meta.activeHost.IsAvailable() {
			return meta.activeHost.Host
		}
	}

	hosts := meta.GetSeedHosts()
	if hosts != nil && len(hosts) > 0 {
		for _, v := range hosts {
			if v != "" {
				if IsHostAvailable(v) {
					//add to cache
					info, ok := GetHostAvailableInfo(v)
					if ok && info != nil {
						if info.IsAvailable() {
							meta.activeHost = info
						}
					}
					return v

				}
			}
		}
	}

	if meta.Config.Discovery.Enabled {
		if meta.Nodes != nil {
			for _, v1 := range *meta.Nodes {
				v := v1.GetHttpPublishHost()
				if v != "" {
					if IsHostAvailable(v) {
						//add to cache
						info, ok := GetHostAvailableInfo(v)
						if ok && info != nil {
							if info.IsAvailable() {
								meta.activeHost = info
							}
						}
						return v
					}
				}
			}
		}
	}

	if len(hosts) == 0 {
		panic(errors.New("hosts is empty"))
	}

	if rate.GetRateLimiter("cluster_available", meta.Config.Name, 1, 1, time.Second*10).Allow() {
		log.Debug("no hosts available, choose: ", hosts[0])
	}
	meta.ReportFailure(nil)
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

var masterNotFoundErrors = []string{"master_not_discovered_exception"}

func (meta *ElasticsearchMetadata) ReportFailure(errorMessage error) bool {

	//Handle master_not_discovered_exception
	//{"error":{"root_cause":[{"type":"master_not_discovered_exception","reason":null}],"type":"master_not_discovered_exception","reason":null},"status":503}json: cannot unmarshal "503}" into Go struct field map[string]adapter.AliasesResponse{}.status of type adapter.AliasesResponse

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

			if errorMessage != nil && util.ContainsAnyInArray(errorMessage.Error(), masterNotFoundErrors) && !meta.Config.AllowAccessWhenMasterNotFound {
				log.Warnf("master_not_discovered_exception found for [%v], mark it down", meta.Config.Name)
				meta.clusterAvailable = false
				meta.clusterFailureTicket = 0
				return true
			}

			num := meta.GetActiveHosts()
			log.Infof("%v has active hosts: %v", meta.Config.Name, num)
			if num > 0 {
				log.Debugf("enough failure ticket for elasticsearch [%v], but still have [%v] alive nodes", meta.Config.Name, num)
				return false
			}

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
		allowedUpdate := rate.GetRateLimiter("cluster_available", meta.Config.Name, 10, 1, time.Second*1).Allow()
		log.Infof("update elasticsearch [%v] to available, allowedUpdate:[%v]", meta.Config.Name, allowedUpdate)
		if allowedUpdate {
			log.Tracef("vote success ticket++ for elasticsearch [%v]", meta.Config.Name)
			meta.clusterAvailable = true
			meta.clusterFailureTicket = 0
			log.Infof("elasticsearch [%v] is available", meta.Config.Name)
		}
	}
}

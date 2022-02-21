package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"strings"
	"sync"
	"time"
)

func clusterHealthCheck(force bool) {
	elastic.WalkConfigs(func(key, value interface{}) bool {
		cfg1, ok := value.(*elastic.ElasticsearchConfig)
		if ok && cfg1 != nil {
			log.Tracef("init task walk configs: %v",cfg1.Name)

			go func(clusterID string) {
				log.Tracef("execute task walk configs: %v",clusterID)
				cfg:=elastic.GetConfig(clusterID)
				metadata := elastic.GetOrInitMetadata(cfg)
				if cfg.Enabled || force {
					//check seeds' availability
					if force {
						//add seeds to host for health check
						hosts := metadata.GetSeedHosts()
						for _, host := range hosts {
							elastic.GetOrInitHost(host)
						}
					}

					client := elastic.GetClient(cfg.ID)

					//check cluster health status
					health,err := client.ClusterHealth()
					if err!=nil||health==nil||health.StatusCode!=200{
						metadata.ReportFailure()
					}else{
						metadata.ReportSuccess()
						if metadata.Health==nil|| metadata.Health.Status!=health.Status{
							metadata.Health=health
							log.Tracef("cluster [%v] health [%v] updated", clusterID,metadata.Health)
						}
					}
				}
			}(cfg1.ID)
		}
		return true
	})
}

//update cluster state, on state version change
func (module *ElasticModule)updateClusterState(clusterId string) {

	log.Trace("update cluster state:",clusterId)

	meta := elastic.GetMetadata(clusterId)
	if meta==nil{
		return
	}

	if !meta.IsAvailable(){
		return
	}

	client := elastic.GetClient(clusterId)
	state,err := client.GetClusterState()
	if err!=nil{
		log.Errorf("failed to get [%v] state: %v",clusterId,err)
		return
	}

	if state != nil {
		stateChanged := false
		if meta.ClusterState == nil {
			stateChanged = true
			log.Tracef("cluster state updated from nothing to [%v]", state.Version)
		} else if state.Version > meta.ClusterState.Version {
			stateChanged = true
			log.Tracef("cluster state updated from version [%v] to [%v]", meta.ClusterState.Version, state.Version)
		}

		if stateChanged {
			//TODO locker
			if moduleConfig.ORMConfig.Enabled{

				module.saveIndexMetadata(state, clusterId)

				state.Metadata = nil
				event:=util.MapStr{
					"cluster_id":clusterId,
					"state":state,
				}
				queue.Push(queue.GetOrInitConfig("cluster_state_change"),util.MustToJSONBytes(event))
			}
			meta.ClusterState = state
		}
	}
}

var saveIndexMetadataMutex = sync.Mutex{}
func (module *ElasticModule)saveIndexMetadata(state *elastic.ClusterState, clusterID string){
	saveIndexMetadataMutex.Lock()
	defer saveIndexMetadataMutex.Unlock()
	//indexNames := make([]string, 0, len(state.Metadata.Indices))
	//for indexName, _ := range state.Metadata.Indices {
	//	indexNames = append(indexNames, indexName)
	//}
	queryDslTpl := `{
  "size": 5000, 
  "query": {
    "bool": {
      "must": [
        {"term": {
          "cluster_id": {
            "value": "%s"
          }
        }}
      ]
    }
  },
 "collapse": {
    "field": "index_name"
  },
  "sort": [
    {
      "timestamp": {
        "order": "desc"
      }
    }
  ]
}`
	queryDsl := fmt.Sprintf(queryDslTpl, clusterID)
	q := &orm.Query{}
	q.RawQuery = []byte(queryDsl)
	err, result := orm.Search(&elastic.IndexMetadata{}, q)
	if err != nil {
		if rate.GetRateLimiterPerSecond(clusterID, "save_index_metadata_failure_on_error", 1).Allow() {
			log.Errorf("elasticsearch [%v] failed to save index metadata: %v",clusterID, err)
		}
		return
	}
	notChanges := util.MapStr{}
	var indexName string
	for _, item := range result.Result {
		if info, ok := item.(map[string]interface{}); ok {
			infoMap := util.MapStr(info)
			indexName = info["index_name"].(string)
			if v, err := infoMap.GetValue("metadata.version"); err == nil {
				if newInfo, ok := state.Metadata.Indices[indexName].(map[string]interface{}); ok {
					if v != nil && newInfo["version"] != nil && v.(float64) >= newInfo["version"].(float64) {
						notChanges[indexName] = true
					}
				}
			}else{
				//compare metadata for lower elasticsearch version
				oldMetadata := util.MapStr(info["metadata"].(map[string]interface{}))
				if newData, ok :=  state.Metadata.Indices[indexName]; ok {
					newMetadata := util.MapStr(newData.(map[string]interface{}))
					if oldMetadata.Equals(newMetadata) {
						notChanges[indexName] = true
					}
				}
			}
		}
	}

	for indexName, indexMetadata := range state.Metadata.Indices {
		data := indexMetadata.(map[string]interface{})
		var indexID interface{} = nil
		indexID, _ = util.GetMapValueByKeys([]string{"settings","index", "uuid"}, data)
		if indexID == nil {
			indexID = ""
		}
		newIndexMetadata := &elastic.IndexMetadata{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			ClusterID: clusterID,
			IndexID: indexID.(string),
			IndexName: indexName,
			Metadata: data,
		}
		if _, ok := notChanges[indexName]; ok {
			continue
		}
		err = orm.Save(newIndexMetadata)
		if err != nil {
			if rate.GetRateLimiterPerSecond(clusterID, "save_index_metadata_failure_on_error", 1).Allow() {
				log.Errorf("elasticsearch [%v] failed to save index metadata: %v",clusterID, err)
			}
		}
	}
}

//on demand, on state version change
func (module *ElasticModule)updateNodeInfo(meta *elastic.ElasticsearchMetadata) {
	if !meta.IsAvailable(){
		log.Debugf("elasticsearch [%v] is not available, skip update node info",meta.Config.Name)
		return
	}

	client := elastic.GetClient(meta.Config.ID)
	nodes, err := client.GetNodes()
	if err != nil || nodes == nil || len(*nodes) <= 0 {
		if rate.GetRateLimiterPerSecond(meta.Config.ID, "get_nodes_failure_on_error", 1).Allow() {
			log.Errorf("elasticsearch [%v] failed to get nodes info", meta.Config.Name)
		}
		return
	}

	var nodesChanged = false

	if meta.Nodes == nil {
		nodesChanged = true
	} else {
		if len(*meta.Nodes) != len(*nodes) {
			nodesChanged = true
		} else {
			for k, v := range *nodes {
				v1, ok := (*meta.Nodes)[k]
				if ok {
					if v.GetHttpPublishHost() != v1.GetHttpPublishHost() {
						nodesChanged = true
						break
					}
				} else {
					nodesChanged = true
					break
				}
			}
		}
	}

	if nodesChanged {
		//TODO locker
		if moduleConfig.ORMConfig.Enabled{
			for k, v := range *nodes {
				err = saveNodeMetadata(k, &v, meta.Config.ID)
				if err != nil {
					if rate.GetRateLimiterPerSecond(meta.Config.ID, "save_nodes_metadata_on_error", 1).Allow() {
						log.Errorf("elasticsearch [%v] failed to save nodes info: %v", meta.Config.Name, err)
					}
				}
			}
		}

		meta.Nodes = nodes
		meta.NodesTopologyVersion++
		log.Tracef("cluster nodes [%v] updated", meta.Config.Name)

		//register host to do availability monitoring
		for _, v := range *nodes {
			elastic.GetOrInitHost(v.GetHttpPublishHost())
		}
		//TODO　save to es metadata
	}
	log.Trace(meta.Config.Name,"nodes changed:",nodesChanged)
}

var saveNodeMetadataMutex = sync.Mutex{}
func saveNodeMetadata(nodeID string, nodeInfo *elastic.NodesInfo, clusterID string) error {
	saveNodeMetadataMutex.Lock()
	defer saveNodeMetadataMutex.Unlock()
	queryDslTpl := `{
  "size": 1, 
  "query": {
    "bool": {
      "must": [
        {"term": {
          "cluster_id": {
            "value": "%s"
          }
        }},
        {"term": {
          "node_id": {
            "value": "%s"
          }
        }}
      ]
    }
  },
  "sort": [
    {
      "timestamp": {
        "order": "desc"
      }
    }
  ]
}`
	queryDsl := fmt.Sprintf(queryDslTpl, clusterID, nodeID)
	q := &orm.Query{}
	q.RawQuery = []byte(queryDsl)
	err, result := orm.Search(&elastic.NodeMetadata{}, q)
	if err != nil {
		return err
	}
	isStateChange := true
	if len(result.Result) > 0 {
		if info, ok := result.Result[0].(map[string]interface{}); ok {
			if v, err := util.MapStr(info).GetValue("metadata.http.publish_address"); err == nil {
				if nodeInfo.Http.PublishAddress == v.(string) {
					isStateChange = false
				}
			}
		}

	}
	nodeMetadata := &elastic.NodeMetadata{
		Metadata: *nodeInfo,
		ClusterID: clusterID,
		ID:  util.GetUUID(),
		NodeID: nodeID,
		Timestamp: time.Now(),
	}

	if isStateChange {
		transportIP := strings.Split(nodeInfo.TransportAddress, ":")[0]
		tempIps := util.MapStr{
			nodeInfo.Ip : struct{}{},
			nodeInfo.Host: struct{}{},
			transportIP: struct {}{},
		}
		ips := make([]string, 0, len(tempIps))
		for k, _ := range tempIps {
			ips = append(ips, k)
		}
		hostMetadata := &elastic.HostMetadata{
			ClusterID: clusterID,
			NodeID: nodeID,
			ID:  util.GetUUID(),
			Timestamp: time.Now(),
		}
		hostMetadata.Metadata.Host = nodeInfo.Host
		hostMetadata.Metadata.OS = nodeInfo.Os
		hostMetadata.Metadata.IPs = ips
		err := orm.Save(hostMetadata)
		if err != nil {
			return err
		}
		return orm.Save(nodeMetadata)
	}
	return nil
}


//func updateIndices(meta *elastic.ElasticsearchMetadata) {
//
//	if !meta.IsAvailable(){
//		return
//	}
//
//	client := elastic.GetClient(meta.Config.ID)
//
//	//Indices
//	var indicesChanged bool
//	indices, err := client.GetIndices("")
//	if err != nil {
//		log.Errorf("[%v], %v", meta.Config.Name, err)
//		return
//	}
//
//	if indices != nil {
//		if meta.Indices == nil {
//			indicesChanged = true
//		} else {
//			for k, v := range *indices {
//				v1, ok := (*meta.Indices)[k]
//				if ok {
//					if v.ID != v1.ID {
//						indicesChanged = true
//						break
//					}
//				} else {
//					indicesChanged = true
//					break
//				}
//			}
//		}
//	}
//
//	if indicesChanged {
//		//TOD locker
//		meta.Indices = indices
//
//		log.Tracef("cluster indices [%v] updated", meta.Config.Name)
//	}
//}

//on demand, on state version change
func updateAliases(meta *elastic.ElasticsearchMetadata) {

	if !meta.IsAvailable(){
		return
	}

	client := elastic.GetClient(meta.Config.ID)

	//Aliases
	var aliasesChanged bool
	aliases, err := client.GetAliases()
	if err != nil {
		log.Errorf("[%v], %v", meta.Config.Name, err)
		return
	}

	if aliases != nil {
		if meta.Aliases == nil {
			aliasesChanged = true
		} else {
			for k, v := range *aliases {
				v1, ok := (*meta.Aliases)[k]
				if ok {
					if len(v.Index) != len(v1.Index) || v.WriteIndex != v1.WriteIndex || util.JoinArray(v.Index, ",") != util.JoinArray(v1.Index, ",") {
						aliasesChanged = true
						break
					}
				} else {
					aliasesChanged = true
					break
				}
			}
		}
	}

	if aliasesChanged {
		//TOD locker
		meta.Aliases = aliases
		log.Tracef("cluster aliases [%v] updated", meta.Config.Name)
	}
}

//func updateShards(meta *elastic.ElasticsearchMetadata) {
//	if !meta.IsAvailable(){
//		return
//	}
//
//	client := elastic.GetClient(meta.Config.ID)
//
//	//Shards
//	var shardsChanged bool
//	shards, err := client.GetPrimaryShards()
//	if err != nil {
//		log.Errorf("[%v], %v", meta.Config.Name, err)
//		return
//	}
//
//	if meta.PrimaryShards == nil {
//		shardsChanged = true
//	} else {
//		if shards != nil {
//			for k, v := range *shards {
//				v1, ok := (*meta.PrimaryShards)[k]
//				if ok {
//					if len(v) != len(v1) {
//						shardsChanged = true
//						break
//					} else {
//						for x,y:=range v{
//							z1, ok := v1[x]
//							if ok{
//								if y.NodeID!=z1.NodeID{
//									shardsChanged = true
//									break
//								}
//							}else{
//								shardsChanged = true
//								break
//							}
//
//						}
//					}
//				} else {
//					shardsChanged = true
//					break
//				}
//			}
//		}
//	}
//
//	if shardsChanged {
//		//TOD locker
//		meta.PrimaryShards = shards
//		log.Tracef("cluster shards [%v] updated", meta.Config.Name)
//	}
//}

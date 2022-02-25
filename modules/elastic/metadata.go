package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
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
				if meta.Config.Source != "file"{
					module.saveIndexMetadata(state, clusterId)
				}
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
	defer func() {
		saveIndexMetadataMutex.Unlock()
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()
	var indexIDToName = map[string] interface{}{}
	for indexName, indexMetadata := range state.Metadata.Indices {
		if data, ok := indexMetadata.(map[string]interface{}); ok {
			indexID, _ := util.GetMapValueByKeys([]string{"settings", "index", "uuid"}, data)
			if indexID == nil {
				continue
			}
			indexIDToName[indexID.(string)] = indexName
		}

	}
	queryDslTpl := `{
  "size": 5000, 
  "query": {
    "bool": {
      "must": [
        {"term": {
          "metadata.cluster_id": {
            "value": "%s"
          }
        }},
		 {"term": {
          "metadata.category": {
            "value": "elasticsearch"
          }
        }}
      ],
		"must_not": [
        {"term": {
          "metadata.labels.index_status": {
            "value": "deleted"
          }
        }}
      ]
    }
  }
}`
	queryDsl := fmt.Sprintf(queryDslTpl, clusterID)
	q := &orm.Query{}
	q.RawQuery = []byte(queryDsl)
	err, result := orm.Search(&elastic.IndexConfig{}, q)
	if err != nil {
		if rate.GetRateLimiterPerSecond(clusterID, "save_index_metadata_failure_on_error", 1).Allow() {
			log.Errorf("elasticsearch [%v] failed to save index metadata: %v",clusterID, err)
		}
		return
	}
	notChanges := util.MapStr{}
	var (
		indexName string
		indexIDMap = map[string]string{}
	)

	deletedConfigMap := map[string]util.MapStr{}
	oldMetadataMap := map[string]util.MapStr{}
	for _, item := range result.Result {
		if info, ok := item.(map[string]interface{}); ok {
			infoMap := util.MapStr(info)
			tempIndexID, _ := infoMap.GetValue("metadata.index_id")
			if tempIndexID == nil {
				continue
			}
			indexID := tempIndexID.(string)
			indexIDMap[indexID] = info["id"].(string)
			tempIndexName, _ := infoMap.GetValue("metadata.index_name")
			indexName = tempIndexName.(string)
			oldMetadataMap[indexIDMap[indexID]] = info
			if indexIDToName[indexID] == nil {
				//deleted
				deletedConfigMap[info["id"].(string)] = infoMap
				continue
			}
			if v, err := infoMap.GetValue("payload.index_state.version"); err == nil {
					if newInfo, ok := state.Metadata.Indices[indexName].(map[string]interface{}); ok {
						if v != nil && newInfo["version"] != nil && v.(float64) >= newInfo["version"].(float64) {
							notChanges[indexName] = true
						}
					}
			}else{
				//compare metadata for lower elasticsearch version
				tempOldMetadata, _ := infoMap.GetValue("payload.index_state")
				if oldMetadata, ok := tempOldMetadata.(map[string]interface{}); ok {
					if newData, ok :=  state.Metadata.Indices[indexName]; ok {
						newMetadata := util.MapStr(newData.(map[string]interface{}))
						if newMetadata.Equals(oldMetadata) {
							notChanges[indexName] = true
						}
					}
				}
			}
		}
	}

	for indexName, indexMetadata := range state.Metadata.Indices {
		if _, ok := notChanges[indexName]; ok {
			continue
		}
		var indexID interface{} = nil
		data := indexMetadata.(map[string]interface{})
		indexID, _ = util.GetMapValueByKeys([]string{"settings","index", "uuid"}, data)
		if indexID == nil {
			indexID = ""
		}
		var typ = ""
		var newIndexMetadata *elastic.IndexConfig
		var innerIndexID = ""
		if innerID, ok := indexIDMap[indexID.(string)]; ok {
			//update
			typ = "update"
			innerIndexID = innerID
			//only overwrite follow labels
			newLabels := util.MapStr{
				"version": data["version"],
				"aliases": data["aliases"],
				"state": data["state"],
			}
			if labels, err := oldMetadataMap[innerID].GetValue("metadata.labels"); err == nil {
				if labelsM, ok := labels.(map[string]interface{}); ok {
					for k, v := range labelsM {
						if _, ok := newLabels[k]; !ok {
							newLabels[k] = v
						}
					}
				}
			}
			newIndexMetadata = &elastic.IndexConfig{
				ID:        innerID,
				Timestamp: time.Now(),
				Metadata:  elastic.IndexMetadata{
					IndexID: indexID.(string),
					IndexName: indexName,
					ClusterID: clusterID,
					Labels: newLabels,
					Category: "elasticsearch",
				},
				Fields: util.MapStr{
					"index_state": indexMetadata,
				},
			}
		}else{
			//new
			typ = "create"
			innerIndexID = util.GetUUID()
			newIndexMetadata = &elastic.IndexConfig{
				ID: innerIndexID,
				Timestamp: time.Now(),
				Metadata:  elastic.IndexMetadata{
					IndexID: indexID.(string),
					IndexName: indexName,
					ClusterID: clusterID,
					Category: "elasticsearch",
					Labels: util.MapStr{
						"version": data["version"],
						"aliases": data["aliases"],
					},
				},
				Fields: util.MapStr{
					"index_state": indexMetadata,
				},
			}
		}

		err = orm.Save(newIndexMetadata)
		if err != nil {
			if rate.GetRateLimiterPerSecond(clusterID, "save_index_metadata_failure_on_error", 1).Allow() {
				log.Errorf("elasticsearch [%v] failed to save index metadata: %v",clusterID, err)
			}
		}

		activityInfo := &event.Activity{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			Metadata: event.ActivityMetadata{
				Category: "elasticsearch",
				Group: "metadata",
				Name: "metadata_index",
				Type: typ,
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"index_id": innerIndexID,
				},
			},
			Fields: util.MapStr{
				"metadata": indexMetadata,
			},
		}
		err = orm.Save(activityInfo)
		if err != nil {
			log.Error(err)
		}
	}
	//update deleted index
	for innerIndexID, configInfo := range deletedConfigMap {
		if indexStatus, err := configInfo.GetValue("metadata.labels.index_status"); err == nil {
			if indexStatus == "deleted" {
				continue
			}
		}
		configInfo.Put("metadata.labels.index_status", "deleted")
		buf := util.MustToJSONBytes(configInfo)
		configObj := &elastic.IndexConfig{}
		util.MustFromJSONBytes(buf, configObj)
		err = orm.Save(configObj)
		if err != nil {
			log.Error(err)
		}

		activityInfo := &event.Activity{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			Metadata: event.ActivityMetadata{
				Category: "elasticsearch",
				Group: "metadata",
				Name: "metadata_index",
				Type: "deleted",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"index_id": innerIndexID,
				},
			},
			Fields: util.MapStr{},
		}
		err = orm.Save(activityInfo)
		if err != nil {
			log.Error(err)
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
		if moduleConfig.ORMConfig.Enabled {
			if meta.Config.Source != "file"{
				err = saveNodeMetadata(*nodes, meta.Config.ID)
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
func saveNodeMetadata(nodes map[string]elastic.NodesInfo, clusterID string) error {
	saveNodeMetadataMutex.Lock()
	defer func() {
		saveNodeMetadataMutex.Unlock()
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()

	queryDslTpl := `{
	"size": 1000,
  "query": {
    "bool": {
      "must": [
        {"term": {
          "metadata.cluster_id": {
            "value": "%s"
          }
        }},
		 {"term": {
          "metadata.category": {
            "value": "elasticsearch"
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
	queryDsl := fmt.Sprintf(queryDslTpl, clusterID)
	q := &orm.Query{}
	q.RawQuery = []byte(queryDsl)
	err, result := orm.Search(&elastic.NodeConfig{}, q)
	if err != nil {
		return err
	}
	//nodeMetadatas := map[string] util.MapStr{}
	nodeIDMap := map[string]interface{}{}
	historyNodeMetadata := map[string] util.MapStr{}
	for _, nodeItem := range result.Result {
		if nodeInfo, ok := nodeItem.(map[string]interface{}); ok {
			if nodeID, ok := util.GetMapValueByKeys([]string{"metadata", "node_id"}, nodeInfo); ok {
				//nodeMetadatas[nodeID] = nodeInfo
				if nid, ok := nodeID.(string); ok {
					if id, ok := nodeInfo["id"]; ok {
						nodeIDMap[nid] = id
					}
					historyNodeMetadata[nid] = nodeInfo
				}
			}
		}
	}

	for rawNodeID, nodeInfo := range nodes {
		rawBytes := util.MustToJSONBytes(nodeInfo)
		currentNodeInfo := util.MapStr{}
		util.MustFromJSONBytes(rawBytes, &currentNodeInfo)
		var innerID interface{}
		var typ string
		if rowID, ok := nodeIDMap[rawNodeID]; !ok {
			//new
			newID := util.GetUUID()
			typ = "create"
			innerID = newID
			nodeMetadata := &elastic.NodeConfig{
				Metadata: elastic.NodeMetadata{
					ClusterID: clusterID,
					NodeID:    rawNodeID,
					Category: "elasticsearch",
					Labels: util.MapStr{
						"node_name": nodeInfo.Name,
						"transport_address": nodeInfo.TransportAddress,
						"host": nodeInfo.Host,
						"ip": nodeInfo.Ip,
						"version": nodeInfo.Version,
						"roles": nodeInfo.Roles,
					},
				},
				ID:  newID,
				Timestamp: time.Now(),
				Fields: util.MapStr{
					"node_state": nodeInfo,
				},
			}
			err = orm.Save(nodeMetadata)
			if err != nil {
				log.Error(err)
			}
		}else {
			innerID = rowID
			typ = "update"
			if rid, ok := rowID.(string); ok {
				if historyM, ok := historyNodeMetadata[rawNodeID]; ok {
					if oldMetadata, err := historyM.GetValue("payload.node_state"); err == nil  {
						if oldMetadataM, ok := oldMetadata.(map[string]interface{}); ok && currentNodeInfo.Equals(oldMetadataM) {
							continue
						}
					}
					//only overwrite follow labels
					newLabels := util.MapStr{
						"node_name": nodeInfo.Name,
						"transport_address": nodeInfo.TransportAddress,
						"host": nodeInfo.Host,
						"ip": nodeInfo.Ip,
						"version": nodeInfo.Version,
						"roles": nodeInfo.Roles,
					}
					if labels, err := historyM.GetValue("metadata.labels"); err == nil {
						if labelsM, ok := labels.(map[string]interface{}); ok {
							for k, v := range labelsM {
								if _, ok := newLabels[k]; !ok {
									newLabels[k] = v
 								}
							}

						}
					}
					nodeMetadata := &elastic.NodeConfig{
						Metadata: elastic.NodeMetadata{
							ClusterID: clusterID,
							NodeID: rawNodeID,
							Labels: newLabels,
							Category: "elasticsearch",
						},
						ID:  rid,
						Timestamp: time.Now(),
						Fields: util.MapStr{
							"node_state": nodeInfo,
						},
					}
					err = orm.Save(nodeMetadata)
					if err != nil {
						log.Error(err)
					}
				}
			}


		}
		activityInfo := &event.Activity{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			Metadata: event.ActivityMetadata{
				Category: "elasticsearch",
				Group: "metadata",
				Name: "metadata_node",
				Type: typ,
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"node_id": innerID,
				},
			},
			Fields: util.MapStr{
				"metadata": nodeInfo,
			},
		}
		err = orm.Save(activityInfo)
		if err != nil {
			log.Error(err)
		}
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

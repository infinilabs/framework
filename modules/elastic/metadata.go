package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"reflect"
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
						if metadata.Config.Source != "file" {
							updateClusterHealthStatus(clusterID, "unavailable")
						}
					}else{
						metadata.ReportSuccess()
						if metadata.Health==nil|| metadata.Health.Status!=health.Status{
							metadata.Health=health
							if metadata.Config.Source != "file" {
								updateClusterHealthStatus(clusterID, health.Status)
							}
							log.Tracef("cluster [%v] health [%v] updated", clusterID,metadata.Health)
						}
					}
				}
			}(cfg1.ID)
		}
		return true
	})
}

func updateClusterHealthStatus(clusterID string, healthStatus string){
	client := elastic.GetClient(moduleConfig.Elasticsearch)
	if client == nil {
		log.Errorf("cluster %s not found", moduleConfig.Elasticsearch)
	}
	var indexName = orm.GetIndexName(elastic.ElasticsearchConfig{})
	getRes, err := client.Get(indexName,"", clusterID)
	if err != nil {
		return
	}
	if !getRes.Found {
		log.Errorf("cluster %s not found", clusterID)
		return
	}
	var (
		labels map[string]interface{}
		ok bool
		oldHealthStatus interface{}
	)
	if labels, ok =  getRes.Source["labels"].(map[string]interface{}); ok {
		if !reflect.DeepEqual(labels["health_status"], healthStatus) {
			oldHealthStatus = labels["health_status"]
			labels["health_status"] = healthStatus
		}else{
			return
		}
	}else{
		oldHealthStatus = "unknown"
		labels = util.MapStr{
			"health_status": healthStatus,
		}
	}
	getRes.Source["labels"] = labels
	getRes.Source["updated"] = time.Now()

	_, err = client.Index(indexName, "", getRes.ID, getRes.Source)
	if err != nil {
		log.Errorf("save cluster health status error: %v", err)
	}

	activityInfo := &event.Activity{
		ID: util.GetUUID(),
		Timestamp: time.Now(),
		Metadata: event.ActivityMetadata{
			Category: "elasticsearch",
			Group: "health",
			Name: "cluster_health",
			Type: "update",
			Labels: util.MapStr{
				"cluster_id": clusterID,
				"from_status": oldHealthStatus,
				"to_status": healthStatus,
			},
		},
	}
	_, err = client.Index(orm.GetIndexName(activityInfo), "", activityInfo.ID, activityInfo)
	if err != nil {
		log.Error(err)
	}

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
			if meta.Config.Source != "file"{
				module.saveRoutingTable(state, clusterId)
			}
			meta.ClusterState = state
		}
	}
}

func (module *ElasticModule)saveRoutingTable(state *elastic.ClusterState, clusterID string) {
	nodesRouting := map[string][]elastic.IndexShardRouting{}
	for indexName, routing := range state.RoutingTable.Indices {
		err := event.Save(event.Event{
			Timestamp: time.Now(),
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name:     "index_routing_table",
				Datatype: "snapshot",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"index_name": indexName,
				},
			},
			Fields: util.MapStr{
				"elasticsearch": util.MapStr{
					"index_routing_table": routing,
				},
			},
		})
		if err != nil {
			log.Error(err)
		}
		for _, routeData := range routing.Shards {
			for _, rd := range routeData {
				if _, ok := nodesRouting[rd.Node]; !ok {
					if rd.Node == ""{
						continue
					}
					nodesRouting[rd.Node] = []elastic.IndexShardRouting{}
				}
				nodesRouting[rd.Node] = append(nodesRouting[rd.Node], rd)
			}
		}

	}
	for nodeID, routing := range nodesRouting {
		event.Save(event.Event{
			Timestamp: time.Now(),
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name: "node_routing_table",
				Datatype: "snapshot",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"node_id": nodeID,
				},
			},
			Fields: util.MapStr{
				"elasticsearch": util.MapStr{
					"node_routing_table": routing,
				},
			},
		})

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
	esConfig := elastic.GetConfig(clusterID)
	esClient := elastic.GetClient(clusterID)
	indexInfos, err := esClient.GetIndices("")
	if err != nil {
		log.Error(err)
		return
	}
	indexHealths := map[string]string{}
	for iname, info := range *indexInfos {
		indexHealths[iname] = info.Health
	}

	oldIndexBytes, err := kv.GetValue(elastic.KVElasticIndexMetadata, []byte(clusterID))
	if err != nil {
		log.Error(err)
		return
	}


	oldIndexMetadata := util.MapStr{}
	util.MustFromJSONBytes(oldIndexBytes, &oldIndexMetadata)

	notChanges := util.MapStr{}
	isIndicesStateChange := false

	for indexName, item := range oldIndexMetadata {
		if info, ok := item.(map[string]interface{}); ok {
			infoMap := util.MapStr(info)
			//tempIndexID, _ := infoMap.GetValue("settings.index.uuid")

			if state.Metadata.Indices[indexName] == nil {
				isIndicesStateChange = true
				metadataEvent := event.Event{
					Timestamp: time.Now(),
					Metadata: event.EventMetadata{
						Name: "index_state_change",
						Category: "elasticsearch",
						Datatype: "snapshot",
						Labels: util.MapStr{
							"cluster_id": clusterID,
							"index_name": indexName,
							"cluster_name": esConfig.Name,
							"type": "delete",
						},
					},
				}
				queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexState)
				if queueConfig.Labels == nil {
					queueConfig.Labels = map[string]interface{}{
						"type":     "metadata",
						"name":     "index_state_change",
						"category": "elasticsearch",
					}
				}
				err = queue.Push(queueConfig, util.MustToJSONBytes(metadataEvent))
				if err != nil {
					panic(err)
				}
				continue
			}
			if v, err := infoMap.GetValue("version"); err == nil {
					if newInfo, ok := state.Metadata.Indices[indexName].(map[string]interface{}); ok {
						if v != nil && newInfo["version"] != nil && v.(float64) >= newInfo["version"].(float64) {
							notChanges[indexName] = true
						}
					}
			}else{
				//compare metadata for lower elasticsearch version
					if newData, ok :=  state.Metadata.Indices[indexName]; ok {
						newMetadata := util.MapStr(newData.(map[string]interface{}))
						if newMetadata.Equals(info) {
							notChanges[indexName] = true
						}
					}
			}
		}
	}

	for indexName, indexMetadata := range state.Metadata.Indices {
		if _, ok := notChanges[indexName]; ok {
			continue
		}
		//var indexID interface{} = nil
		//data := indexMetadata.(map[string]interface{})
		//indexID, _ = util.GetMapValueByKeys([]string{"settings","index", "uuid"}, data)
		//if indexID == nil {
		//	indexID = ""
		//}
		isIndicesStateChange = true
		metadataEvent := event.Event{
			Timestamp: time.Now(),
			Metadata: event.EventMetadata{
				Name: "index_state_change",
				Category: "elasticsearch",
				Datatype: "snapshot",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"index_name": indexName,
					"cluster_name": esConfig.Name,
					"health": indexHealths[indexName],
				},
			},
			Fields: util.MapStr{
				"index_state": indexMetadata,
			},
		}
		if oldIndexMetadata[indexName] != nil {
			metadataEvent.Metadata.Labels["type"] = "update"
			//typ = "update"
			//only overwrite follow labels
			//newLabels := util.MapStr{
			//	"version": data["version"],
			//	"state": data["state"],
			//	"index_uuid": indexID,
			//	"health_status": indexInfo.Health,
			//}
			//if labels, err := oldMetadataMap[innerID].GetValue("metadata.labels"); err == nil {
			//	if labelsM, ok := labels.(map[string]interface{}); ok {
			//		for k, v := range labelsM {
			//			if _, ok := newLabels[k]; !ok {
			//				newLabels[k] = v
			//			}
			//		}
			//	}
			//}

			//newIndexMetadata = &elastic.IndexConfig{
			//	ID:        innerID,
			//	Timestamp: time.Now(),
			//	Metadata:  elastic.IndexMetadata{
			//		IndexID: fmt.Sprintf("%s:%s", clusterID, indexName),
			//		IndexName: indexName,
			//		ClusterName: esConfig.Name,
			//		Aliases:  data["aliases"],
			//		ClusterID: clusterID,
			//		Labels: newLabels,
			//		Category: "elasticsearch",
			//	},
			//	Fields: util.MapStr{
			//		"index_state": indexMetadata,
			//	},
			//}
			//if tags, err := oldMetadataMap[innerID].GetValue("metadata.tags"); err == nil {
			//	if vtags, ok := tags.([]interface{}); ok {
			//		newIndexMetadata.Metadata.Tags = vtags
			//	}
			//}
		}else{
			//new
			metadataEvent.Metadata.Labels["type"] = "create"
		}
		queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexState)
		if queueConfig.Labels == nil {
			queueConfig.Labels = map[string]interface{}{
				"type":     "metadata",
				"name":     "index_state_change",
				"category": "elasticsearch",
			}
		}
		err = queue.Push(queueConfig, util.MustToJSONBytes(metadataEvent))
		if err != nil {
			panic(err)
		}
	}
	if isIndicesStateChange {
		kv.AddValue(elastic.KVElasticIndexMetadata, []byte(clusterID), util.MustToJSONBytes(state.Metadata.Indices))
	}
	//update deleted index
	//for innerIndexID, configInfo := range deletedConfigMap {
	//	if indexStatus, err := configInfo.GetValue("metadata.labels.index_status"); err == nil {
	//		if indexStatus == "deleted" {
	//			continue
	//		}
	//	}
	//	configInfo.Put("metadata.labels.index_status", "deleted")
	//	buf := util.MustToJSONBytes(configInfo)
	//	configObj := &elastic.IndexConfig{}
	//	util.MustFromJSONBytes(buf, configObj)
	//	err = orm.Save(configObj)
	//	if err != nil {
	//		log.Error(err)
	//	}
	//	indexUUID, _ :=  configInfo.GetValue("metadata.labels.index_uuid")
	//	activityInfo := &event.Activity{
	//		ID: util.GetUUID(),
	//		Timestamp: time.Now(),
	//		Metadata: event.ActivityMetadata{
	//			Category: "elasticsearch",
	//			Group: "metadata",
	//			Name: "index_state",
	//			Type: "deleted",
	//			Labels: util.MapStr{
	//				"cluster_id": clusterID,
	//				"index_id": innerIndexID,
	//				"index_uuid": indexUUID,
	//				"cluster_name": esConfig.Name,
	//				"index_name": configObj.Metadata.IndexName,
	//			},
	//		},
	//		Fields: util.MapStr{},
	//	}
	//	err = orm.Save(activityInfo)
	//	if err != nil {
	//		log.Error(err)
	//	}
	//}
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
	esConfig := elastic.GetConfig(clusterID)
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
  }
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
	unavailableNodeIDs := map[string]bool {}
	for _, nodeItem := range result.Result {
		if nodeInfo, ok := nodeItem.(map[string]interface{}); ok {
			if nodeID, ok := util.GetMapValueByKeys([]string{"metadata", "node_id"}, nodeInfo); ok {
				//nodeMetadatas[nodeID] = nodeInfo
				if nid, ok := nodeID.(string); ok {
					if id, ok := nodeInfo["id"]; ok {
						nodeIDMap[nid] = id
					}
					historyNodeMetadata[nid] = nodeInfo
					if _, ok = nodes[nid]; !ok {
						unavailableNodeIDs[nid] = true
					}
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
					ClusterName: esConfig.Name,
					NodeName: nodeInfo.Name,
					Host: nodeInfo.Host,
					Labels: util.MapStr{
						"transport_address": nodeInfo.TransportAddress,
						"ip": nodeInfo.Ip,
						"version": nodeInfo.Version,
						"roles": nodeInfo.Roles,
						"status": "available",
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
						"transport_address": nodeInfo.TransportAddress,
						"ip": nodeInfo.Ip,
						"version": nodeInfo.Version,
						"roles": nodeInfo.Roles,
						"status": "available",
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
							ClusterName: esConfig.Name,
							NodeName: nodeInfo.Name,
							Host: nodeInfo.Host,
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
				Name: "node_state",
				Type: typ,
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"node_id": innerID,
					"cluster_name": esConfig.Name,
				},
			},
			Fields: util.MapStr{
				"node_state": nodeInfo,
			},
		}
		err = orm.Save(activityInfo)
		if err != nil {
			log.Error(err)
		}
	}

	//update unavailable node
	for nodeID, _ := range unavailableNodeIDs {
		oldMetadata := historyNodeMetadata[nodeID]
		oldBytes := util.MustToJSONBytes(oldMetadata)
		oldConfig := elastic.NodeConfig{}
		util.MustFromJSONBytes(oldBytes, &oldConfig)
		oldConfig.Metadata.Labels["status"] = "unavailable"

		err = orm.Save(oldConfig)
		if err != nil {
			log.Error(err)
		}
		activityInfo := &event.Activity{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			Metadata: event.ActivityMetadata{
				Category: "elasticsearch",
				Group: "health",
				Name: "node_health",
				Type: "update",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"status": "unavailable",
					"cluster_name": esConfig.Name,
				},
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

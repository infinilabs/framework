package elastic

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/r3labs/diff/v2"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"reflect"
	"strings"
	"sync"
	"time"
)

func (module *ElasticModule) clusterHealthCheck(clusterID string, force bool) {

	log.Tracef("execute health check for: %v", clusterID)

	cfg := elastic.GetConfig(clusterID)
	metadata := elastic.GetOrInitMetadata(cfg)
	if cfg.Enabled || force {
		//check seeds' availability
		if force {
			//add seeds to host for health check
			hosts := metadata.GetSeedHosts()
			for _, host := range hosts {
				elastic.GetOrInitHost(host, clusterID)
			}
		}
		//metadata.GetHttpClient(metadata.GetActivePreferredSeedEndpoint())
		client := elastic.GetClient(cfg.ID)
		//check cluster health status
		health, err := client.ClusterHealth()
		if err != nil || health == nil || health.StatusCode != 200 {
			if health!=nil&&util.ContainStr(util.UnsafeBytesToString(health.RawResult.Body), "master_not_discovered_exception") {
				metadata.ReportFailure(errors.New("master_not_discovered_exception"))
			} else {
				metadata.ReportFailure(err)
			}

			if metadata.Config.Source != "file" && !metadata.IsAvailable() {
				updateClusterHealthStatus(clusterID, "unavailable")
			}
		} else {
			if metadata.Health == nil || metadata.Health.Status != health.Status || !metadata.IsAvailable(){
				metadata.Health = health
				if metadata.Config.Source != "file" {
					updateClusterHealthStatus(clusterID, health.Status)
				}
				log.Tracef("cluster [%v] health [%v] updated", clusterID, metadata.Health)
			}
			metadata.ReportSuccess()

		}
	}
}

func updateClusterHealthStatus(clusterID string, healthStatus string) {
	client := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
	if client == nil {
		log.Errorf("cluster %s not found", global.MustLookupString(elastic.GlobalSystemElasticsearchID))
		return
	}
	var indexName = orm.GetIndexName(elastic.ElasticsearchConfig{})
	getRes, err := client.Get(indexName, "", clusterID)
	if err != nil {
		log.Errorf("get cluster %s error: %v", clusterID, err)
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

	_, err = client.Index(indexName, "", getRes.ID, getRes.Source, "")
	if err != nil {
		log.Errorf("save cluster health status error: %v", err)
	}

	activityInfo := &event.Activity{
		ID: util.GetUUID(),
		Timestamp: time.Now(),
		Metadata: event.ActivityMetadata{
			Category: "elasticsearch",
			Group: "health",
			Name: "cluster_health_change",
			Type: "update",
			Labels: util.MapStr{
				"cluster_id": clusterID,
				"cluster_name": getRes.Source["name"],
				"from": oldHealthStatus,
				"to": healthStatus,
			},
		},
	}
	_, err = client.Index(orm.GetIndexName(activityInfo), "", activityInfo.ID, activityInfo, "")
	if err != nil {
		log.Error(err)
	}

}

//update cluster state, on state version change
func (module *ElasticModule)updateClusterState(clusterId string) {

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
		var res string
		if state!=nil{
			res=util.UnsafeBytesToString(state.RawResult.Body)
		}
		log.Errorf("failed to get [%v] state: %v, got response: %s",clusterId,err,res )
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

		oldIndexState, err := kv.GetValue(elastic.KVElasticIndexMetadata, []byte(clusterId))

		//TODO locker
		if stateChanged || (err == nil && oldIndexState == nil){
			if meta.Config.Source != "file"{
				if meta.ClusterState == nil || oldIndexState == nil{
					//load init state from es when console start
					oldIndexState, err = module.loadIndexMetadataFromES(clusterId)
					kv.AddValue(elastic.KVElasticIndexMetadata, []byte(clusterId), oldIndexState)
				}
				module.saveIndexMetadata(state, clusterId)
			}
		}
		if stateChanged {
			state.Metadata = nil
			if meta.Config.Source != "file"{
				module.saveRoutingTable(state, clusterId)
			}
			meta.ClusterState = state
		}
	}
}

func (module *ElasticModule) loadIndexMetadataFromES( clusterID string)([]byte, error){
 	esClient := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
	queryDsl := `{
	"size": 1000,
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "metadata.cluster_id": {
              "value": "%s"
            }
          }
        }
      ]
    }
  }
}`
	queryDsl = fmt.Sprintf(queryDsl, clusterID)
	 searchRes, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(elastic.IndexConfig{}), []byte(queryDsl))
	 if err != nil {
		return nil, err
	 }
	 states := util.MapStr{}
	 for _, hit := range searchRes.Hits.Hits {
		 indexName, _ := util.GetMapValueByKeys([]string{"metadata", "index_name"}, hit.Source)
		 indexState, _ := util.GetMapValueByKeys([]string{"payload", "index_state"}, hit.Source)
		 health, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "health_status"}, hit.Source)
		 if v, ok := indexName.(string); ok {
			 states[v] = util.MapStr{
				 "index_state": indexState,
				 "health": health,
				 "id": hit.ID,
			 }
		 }
	 }
	 return util.ToJSONBytes(states)
}

func (module *ElasticModule)saveRoutingTable(state *elastic.ClusterState, clusterID string) {
	if state == nil || state.RoutingTable == nil{
		return
	}
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
	//indexHealths := map[string]string{}
	//for iname, info := range *indexInfos {
	//	indexHealths[iname] = info.Health
	//}

	oldIndexBytes, err := kv.GetValue(elastic.KVElasticIndexMetadata, []byte(clusterID))
	if err != nil {
		log.Error(err)
		return
	}


	oldIndexMetadata := util.MapStr{}
	util.MustFromJSONBytes(oldIndexBytes, &oldIndexMetadata)

	notChanges := util.MapStr{}
	isIndicesStateChange := false
	queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexState)
	if queueConfig.Labels == nil {
		queueConfig.Labels = util.MapStr{
			"type":     "metadata",
			"name":     "index_state_change",
			"category": "elasticsearch",
			"activity": true,
		}
	}
	newIndexMetadata := util.MapStr{}
	for indexName, item := range oldIndexMetadata {
		if info, ok := item.(map[string]interface{}); ok {
			infoMap := util.MapStr(info)
			//tempIndexID, _ := infoMap.GetValue("settings.index.uuid")

			if state.Metadata.Indices[indexName] == nil {
				if infoMap["health"] == nil || infoMap["health"] == "unavailable" { //already deleted
					newIndexMetadata[indexName] = item
					continue
				}
				isIndicesStateChange = true
				var (
					version interface{}
					indexUUID interface{}
					aliases interface{}
				)
				if mp, ok := infoMap["index_state"].(map[string]interface{}); ok {
					mps := util.MapStr(mp)
					version = mps["version"]
					aliases = mps["aliases"]
					indexUUID, _ = mps.GetValue("settings.index.uuid")
				}
				indexConfig := &elastic.IndexConfig{
					ID:       infoMap["id"].(string),
					Timestamp: time.Now(),
					Metadata:  elastic.IndexMetadata{
						IndexID: fmt.Sprintf("%s:%s", clusterID, indexName),
						IndexName: indexName,
						ClusterName: esConfig.Name,
						ClusterID: clusterID,
						Category: "elasticsearch",
						Aliases: aliases,
						Labels: util.MapStr{
							"version": version,
							"state": "delete",
							"index_uuid": indexUUID,
							"health_status": "unavailable",
						},
					},
					Fields: util.MapStr{
						"index_state": infoMap["index_state"],
					},

				}
				activityInfo := &event.Activity{
					ID: util.GetUUID(),
					Timestamp: time.Now(),
					Metadata: event.ActivityMetadata{
						Category: "elasticsearch",
						Group: "metadata",
						Name: "index_state_change",
						Type: "delete",
						Labels: util.MapStr{
							"cluster_id": clusterID,
							"index_name": indexName,
							"cluster_name": esConfig.Name,
						},
					},
					Fields: util.MapStr{
						"index_state": infoMap["index_state"],
					},
				}

				err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
					Timestamp: time.Now(),
					Metadata: event.EventMetadata{
						Category: "elasticsearch",
						Name: "index_state_change",
						Labels: util.MapStr{
							"operation": "delete",
						},
					},
					Fields: util.MapStr{
						"index_state": indexConfig,
					}}))
				if err != nil {
					log.Error(err)
				}
				err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
					Timestamp: time.Now(),
					Metadata: event.EventMetadata{
						Category: "elasticsearch",
						Name: "activity",
					},
					Fields: util.MapStr{
						"activity": activityInfo,
					}}))
				if err != nil {
					log.Error(err)
				}
				newIndexMetadata[indexName] = util.MapStr{
					"id": indexConfig.ID,
					"index_state": infoMap["index_state"],
					"health": "unavailable",
				}
				continue
			}

			if infoMap["health"] != nil && infoMap["health"] != "unavailable" {
				if v, err := infoMap.GetValue("index_state.version"); err == nil {
					if newInfo, ok := state.Metadata.Indices[indexName].(map[string]interface{}); ok {
						indexUUID, _ := infoMap.GetValue("index_state.settings.index.uuid")
						newIndexUUID, _ := util.MapStr(newInfo).GetValue("settings.index.uuid")
						if v != nil && newInfo["version"] != nil && v.(float64) >= newInfo["version"].(float64) && indexUUID == newIndexUUID{
							newIndexMetadata[indexName] = infoMap
							notChanges[indexName] = true
						}
					}
				}
			}

			//else {
			//	//compare metadata for lower elasticsearch version
			//	if newData, ok := state.Metadata.Indices[indexName]; ok {
			//		newMetadata := util.MapStr(newData.(map[string]interface{}))
			//		if newMetadata.Equals(info) {
			//			notChanges[indexName] = true
			//		}
			//	}
			//}
		}
	}

	for indexName, indexMetadata := range state.Metadata.Indices {

		if _, ok := notChanges[indexName]; ok {
			continue
		}
		isIndicesStateChange = true
		health :=  (*indexInfos)[indexName].Health

		var (
			state interface{}
			version interface{}
			indexUUID interface{}
			aliases interface{}
		)
		if mp, ok := indexMetadata.(map[string]interface{}); ok {
			mps := util.MapStr(mp)
			state = mps["state"]
			version = mps["version"]
			aliases = mps["aliases"]
			indexUUID, _ = mps.GetValue("settings.index.uuid")
		}

		indexConfig := &elastic.IndexConfig{
			ID:  util.GetUUID()    ,
			Timestamp: time.Now(),
			Metadata:  elastic.IndexMetadata{
				IndexID: fmt.Sprintf("%s:%s", clusterID, indexName),
				IndexName: indexName,
				ClusterName: esConfig.Name,
				Aliases:  aliases,
				ClusterID: clusterID,
				Labels: util.MapStr{
					"version": version,
					"state": state,
					"index_uuid": indexUUID,
					"health_status": health,
				},
				Category: "elasticsearch",
			},
			Fields: util.MapStr{
				"index_state": indexMetadata,
			},
		}

		activityInfo := &event.Activity{
			ID: util.GetUUID(),
			Timestamp: time.Now(),
			Metadata: event.ActivityMetadata{
				Category: "elasticsearch",
				Group: "metadata",
				Name: "index_state_change",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"index_name": indexName,
					"cluster_name": esConfig.Name,
				},
			},
			Fields: util.MapStr{
				"index_state": indexMetadata,
			},
		}

		if oldIndexMetadata[indexName] != nil {
			//compare metadata for lower elasticsearch version
			oldConfig := oldIndexMetadata[indexName].(map[string]interface{})
			newIndexMetadata[indexName] = util.MapStr{
				"id": oldConfig["id"].(string),
				"index_state": indexMetadata,
				"health": health,
			}
			if oldConfig["health"] != "unavailable" {

				changeLog, _ := util.DiffTwoObject(oldConfig["index_state"], indexMetadata)
				var length = len(changeLog)
				if length == 0 {
					continue
				}
				var filterChangeLog diff.Changelog
				for _, logItem := range changeLog {
					//skip only version and primary_terms.x change
					if strings.HasPrefix(logItem.Path[0], "in_sync_allocations") {
						continue
					}
					if strings.HasPrefix(logItem.Path[0], "primary_terms") {
						continue
					}
					if logItem.Path[0] == "version" {
						continue
					}
					filterChangeLog = append(filterChangeLog, logItem)
				}
				if len(filterChangeLog) == 0 {
					continue
				}
				if oldHealth, ok := oldConfig["health"].(string); ok && oldHealth != health {
					actInfo := event.Activity{
						ID:        util.GetUUID(),
						Timestamp: time.Now(),
						Metadata: event.ActivityMetadata{
							Category: "elasticsearch",
							Group:    "metadata",
							Name:     "index_health_change",
							Type:     "update",
							Labels: util.MapStr{
								"cluster_id":   clusterID,
								"index_name":   indexName,
								"cluster_name": esConfig.Name,
								"from":         oldHealth,
								"to":           health,
							},
						},
					}
					err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
						Timestamp: time.Now(),
						Metadata: event.EventMetadata{
							Category: "elasticsearch",
							Name:     "activity",
						},
						Fields: util.MapStr{
							"activity": actInfo,
						}}))
					if err != nil {
						panic(err)
					}
				}
				activityInfo.Metadata.Type = "update"
				activityInfo.Changelog = filterChangeLog
			}else{
				activityInfo.Metadata.Type = "create"
			}
			indexConfig.ID = oldConfig["id"].(string)

		}else{
			//new
			activityInfo.Metadata.Type = "create"
			newIndexMetadata[indexName] = util.MapStr{
				"id": indexConfig.ID,
				"index_state": indexMetadata,
				"health": health,
			}
		}
		err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
			Timestamp: time.Now(),
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name: "index_state_change",
				Labels: util.MapStr{
					"operation": activityInfo.Metadata.Type,
				},
			},
			Fields: util.MapStr{
				"index_state": indexConfig,
			}}))
		if err != nil {
			log.Error(err)
		}
		err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
			Timestamp: time.Now(),
			Metadata: event.EventMetadata{
				Category: "elasticsearch",
				Name: "activity",
			},
			Fields: util.MapStr{
				"activity": activityInfo,
			}}))
		if err != nil {
			log.Error(err)
		}

	}
	if isIndicesStateChange {
		kv.AddValue(elastic.KVElasticIndexMetadata, []byte(clusterID), util.MustToJSONBytes(newIndexMetadata))
	}
}

//on demand, on state version change
func (module *ElasticModule) updateNodeInfo(meta *elastic.ElasticsearchMetadata, force bool, discovery bool) {
	if !meta.IsAvailable() {
		if !force {
			setNodeUnknown(meta.Config.ID)
		}
		log.Debugf("elasticsearch [%v] is not available, skip update node info", meta.Config.Name)
		return
	}

	client := elastic.GetClient(meta.Config.ID)
	nodes, err := client.GetNodes()
	if err != nil || nodes == nil || len(*nodes) <= 0 {
		if rate.GetRateLimiterPerSecond(meta.Config.ID, "get_nodes_failure_on_error", 1).Allow() {
			log.Errorf("elasticsearch [%v] failed to get nodes info, err: %v", meta.Config.Name,err)
		}
		setNodeUnknown(meta.Config.ID)
		return
	}

	var nodesChanged = false
	if _, ok := nodeAlreadyUnknown[meta.Config.ID]; ok && meta.Config.Source != "file"{
		delete(nodeAlreadyUnknown, meta.Config.ID)
		nodesChanged = true
	}
	var oldNodes = meta.Nodes
	if !discovery {
		buf, err := kv.GetValue(elastic.KVElasticNodeMetadata,[]byte(meta.Config.ID))
		if  err != nil{
			log.Errorf("read node metadata error: %v", err)
			return
		}
		if len(buf) > 0 {
			cacheNodes := struct {
				Nodes *map[string]elastic.NodesInfo `json:"nodes"`
				Timestamp time.Time `json:"timestamp"`
			}{}
			//oldNodes = &map[string]elastic.NodesInfo{}
			util.MustFromJSONBytes(buf, &cacheNodes)
			if cacheNodes.Nodes != nil && time.Since(cacheNodes.Timestamp).Seconds() <= 60 {
				oldNodes = cacheNodes.Nodes
			}
		}
	}

	if oldNodes == nil {
		nodesChanged = true
	} else {
		if len(*oldNodes) != len(*nodes) {
			nodesChanged = true
		} else {
			for k, v := range *nodes {
				v1, ok := (*oldNodes)[k]
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
				//todo check whether store elasticsearch change or not
				err = saveNodeMetadata(*nodes, meta.Config.ID)
				if err != nil {
					if rate.GetRateLimiterPerSecond(meta.Config.ID, "save_nodes_metadata_on_error", 1).Allow() {
						log.Errorf("elasticsearch [%v] failed to save nodes info: %v", meta.Config.Name, err)
					}
				}
			}

		}

		if discovery || force{
			meta.Nodes = nodes
			meta.NodesTopologyVersion++
			log.Tracef("cluster nodes [%v] updated", meta.Config.Name)

			//register host to do availability monitoring
			if discovery{
				for _, v := range *nodes {
					elastic.GetOrInitHost(v.GetHttpPublishHost(), meta.Config.ID)
				}
			}

		}else{
			cacheNodeInfo := util.MapStr{
				"nodes": nodes,
				"timestamp": time.Now(),
			}
			kv.AddValue(elastic.KVElasticNodeMetadata,[]byte(meta.Config.ID), util.MustToJSONBytes(cacheNodeInfo))
		}
		//TODO　save to es metadata
	}
	log.Trace(meta.Config.Name,"nodes changed:",nodesChanged)
}

var saveNodeMetadataMutex = sync.Mutex{}
var nodeAlreadyUnknown = map[string]bool{}
func setNodeUnknown(clusterID string) {
	kv.DeleteKey(elastic.KVElasticNodeMetadata,[]byte(clusterID))
	meta := elastic.GetMetadata(clusterID)
	if meta == nil {
		return
	}
	if meta.Config.Source == "file" {
		return
	}
	if v, ok := nodeAlreadyUnknown[clusterID]; ok && v {
		return
	}
	queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexState)
	if queueConfig.Labels == nil {
		queueConfig.Labels = util.MapStr{
			"type":     "metadata",
			"name":     "index_state_change",
			"category": "elasticsearch",
			"activity": true,
		}
	}
	err := queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
		Timestamp: time.Now(),
		Metadata: event.EventMetadata{
			Category: "elasticsearch",
			Name: "unknown_node_status",
			Labels: util.MapStr{
				"operation": "update",
			},
		},
		Fields: util.MapStr{
			"cluster_id": clusterID,
		}}))
	if err != nil {
		panic(err)
	}

	nodeAlreadyUnknown[clusterID] = true
}
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
		var changeLog diff.Changelog
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
						if oldMetadataM, ok := oldMetadata.(map[string]interface{}); ok { // && currentNodeInfo.Equals(oldMetadataM)
							healthStatus, _ := historyM.GetValue("metadata.labels.status")
							changeLog, _ = util.DiffTwoObject(oldMetadataM, currentNodeInfo)
							if len(changeLog) == 0 && healthStatus == "available" {
								continue
							}
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
							if st, ok := labelsM["status"].(string); ok && st == "unavailable" || st == "N/A" {
								activityInfo := &event.Activity{
									ID: util.GetUUID(),
									Timestamp: time.Now(),
									Metadata: event.ActivityMetadata{
										Category: "elasticsearch",
										Group: "health",
										Name: "node_health_change",
										Type: "update",
										Labels: util.MapStr{
											"cluster_id": clusterID,
											"to": "available",
											"node_id": rawNodeID,
											"node_name": nodeInfo.Name,
											"cluster_name": esConfig.Name,
										},
									},
								}
								err = orm.Save(activityInfo)
								if err != nil {
									log.Error(err)
								}
							}
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
				Name: "node_state_change",
				Type: typ,
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"node_id": innerID,
					"node_uuid": rawNodeID,
					"node_name": nodeInfo.Name,
					"cluster_name": esConfig.Name,
				},
			},
			Fields: util.MapStr{
				"node_state": nodeInfo,
			},
		}
		if typ == "update"{
			if len(changeLog) == 0 {
				continue
			}
			activityInfo.Changelog = changeLog
		}
		err = orm.Save(activityInfo)
		if err != nil {
			log.Error(err, activityInfo)
		}
	}

	//update unavailable node
	for nodeID, _ := range unavailableNodeIDs {
		oldMetadata := historyNodeMetadata[nodeID]
		oldBytes := util.MustToJSONBytes(oldMetadata)
		oldConfig := elastic.NodeConfig{}
		util.MustFromJSONBytes(oldBytes, &oldConfig)
		//skip already unavailable node
		if oldStatus, ok := oldConfig.Metadata.Labels["status"].(string); ok && oldStatus == "unavailable" {
			continue
		}
		oldConfig.Metadata.Labels["status"] = "unavailable"
		oldConfig.Timestamp = time.Now()

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
				Name: "node_health_change",
				Type: "update",
				Labels: util.MapStr{
					"cluster_id": clusterID,
					"to": "unavailable",
					"node_id": oldConfig.Metadata.NodeID,
					"node_uuid": nodeID,
					"node_name": oldConfig.Metadata.NodeName,
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
//	client := elastic.GetClient(meta.QueueConfig.ID)
//
//	//Indices
//	var indicesChanged bool
//	indices, err := client.GetIndices("")
//	if err != nil {
//		log.Errorf("[%v], %v", meta.QueueConfig.Name, err)
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
//		log.Tracef("cluster indices [%v] updated", meta.QueueConfig.Name)
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
//	client := elastic.GetClient(meta.QueueConfig.ID)
//
//	//Shards
//	var shardsChanged bool
//	shards, err := client.GetPrimaryShards()
//	if err != nil {
//		log.Errorf("[%v], %v", meta.QueueConfig.Name, err)
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
//		log.Tracef("cluster shards [%v] updated", meta.QueueConfig.Name)
//	}
//}

func (module *ElasticModule) updateClusterSettings(clusterId string) {
	meta := elastic.GetMetadata(clusterId)
	if meta==nil{
		return
	}
	if !meta.IsAvailable(){
		return
	}
	if meta.Config.Source == "file"{
		return
	}
	log.Trace("update cluster settings:",clusterId)

	client := elastic.GetClient(clusterId)
	settings,err := client.GetClusterSettings()
	if err!=nil{
		log.Errorf("failed to get cluster settings for [%v], err: %v", clusterId, err)
		meta.ReportFailure(err)
		return
	}

	if settings != nil {
		oldClusterSettings, err := kv.GetValue(elastic.KVElasticClusterSettings, []byte(clusterId))
		if err != nil {
			log.Errorf("failed to get kv %s of [%v] : %v",elastic.KVElasticClusterSettings, clusterId,err)
		}
		if oldClusterSettings == nil {
			oldClusterSettings, err = module.loadClusterSettingsFromES(clusterId)
			if err != nil {
				log.Errorf("failed to load cluster settings from es [%v] : %v", clusterId,err)
			}
		}

		if oldClusterSettings != nil {
			oldClusterSettingsM := util.MapStr{}
			util.MustFromJSONBytes(oldClusterSettings, &oldClusterSettingsM)
			changeLog, _ := diff.Diff(oldClusterSettingsM, settings)
			if len(changeLog) == 0 {
				return
			}
			queueConfig := queue.GetOrInitConfig(elastic.QueueElasticIndexState)
			if queueConfig.Labels == nil {
				queueConfig.Labels = util.MapStr{
					"type":     "metadata",
					"name":     "index_state_change",
					"category": "elasticsearch",
					"activity": true,
				}
			}
			activityInfo := &event.Activity{
				ID: util.GetUUID(),
				Timestamp: time.Now(),
				Metadata: event.ActivityMetadata{
					Category: "elasticsearch",
					Group: "metadata",
					Name: "cluster_settings_change",
					Labels: util.MapStr{
						"cluster_id": clusterId,
						"cluster_name": meta.Config.Name,
					},
					Type: "update",
				},
				Changelog: changeLog,
				Fields: util.MapStr{
					"settings": settings,
				},
			}
			err = queue.Push(queueConfig, util.MustToJSONBytes(event.Event{
				Timestamp: time.Now(),
				Metadata: event.EventMetadata{
					Category: "elasticsearch",
					Name: "activity",
				},
				Fields: util.MapStr{
					"activity": activityInfo,
				}}))
			if err != nil {
				panic(err)
			}
			kv.AddValue(elastic.KVElasticClusterSettings, []byte(clusterId), util.MustToJSONBytes(settings))
		}else{
			kv.AddValue(elastic.KVElasticClusterSettings, []byte(clusterId), util.MustToJSONBytes(settings))
		}

	}
}

func (module *ElasticModule) loadClusterSettingsFromES( clusterID string)([]byte, error){
	esClient := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
	queryDsl := `{
	"size": 1,
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "metadata.cluster_id": {
              "value": "%s"
            }
          }
        }
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
	queryDsl = fmt.Sprintf(queryDsl, clusterID)
	searchRes, err := esClient.SearchWithRawQueryDSL(orm.GetIndexName(event.Activity{}), []byte(queryDsl))
	if err != nil||searchRes==nil {
		return nil, err
	}
	if searchRes.GetTotal() == 0 {
		return nil, nil
	}

	if len(searchRes.Hits.Hits)>0{
		clusterSettings, _ := util.GetMapValueByKeys([]string{"payload", "settings"}, searchRes.Hits.Hits[0].Source)
		return util.ToJSONBytes(clusterSettings)
	}

	return nil, nil
}

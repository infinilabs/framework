package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/util"
	"strings"
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
					health := client.ClusterHealth()
					if health==nil||health.StatusCode!=200{
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

//update cluster state
func updateClusterState(clusterId string) {

	log.Trace("update cluster state:",clusterId)

	meta := elastic.GetMetadata(clusterId)
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
			meta.ClusterState = state
			event:=util.MapStr{
				"cluster_id":clusterId,
			}
			queue.Push("cluster_state_change",util.MustToJSONBytes(event))

		}
	}
}

func updateNodeInfo(meta *elastic.ElasticsearchMetadata) {

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
					if v.Http.PublishAddress != v1.Http.PublishAddress {
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
		meta.Nodes = nodes
		meta.NodesTopologyVersion++
		log.Tracef("cluster nodes [%v] updated", meta.Config.Name)

		//register host to do availability monitoring
		for _, v := range *nodes {
			if util.ContainStr(v.Http.PublishAddress,"/"){
				if global.Env().IsDebug{
					log.Tracef("node's public address contains `/`,try to remove prefix")
				}
				arr:=strings.Split(v.Http.PublishAddress,"/")
				if len(arr)==2{
					v.Http.PublishAddress=arr[1]
				}
			}
			elastic.GetOrInitHost(v.Http.PublishAddress)
		}
	}
	log.Trace("nodes changed:",nodesChanged,nodes)
}

func updateIndices(meta *elastic.ElasticsearchMetadata) {
	client := elastic.GetClient(meta.Config.ID)

	//Indices
	var indicesChanged bool
	indices, err := client.GetIndices("")
	if err != nil {
		log.Errorf("[%v], %v", meta.Config.Name, err)
		return
	}

	if indices != nil {
		if meta.Indices == nil {
			indicesChanged = true
		} else {
			for k, v := range *indices {
				v1, ok := (*meta.Indices)[k]
				if ok {
					if v.ID != v1.ID {
						indicesChanged = true
						break
					}
				} else {
					indicesChanged = true
					break
				}
			}
		}
	}

	if indicesChanged {
		//TOD locker
		meta.Indices = indices

		log.Tracef("cluster indices [%v] updated", meta.Config.Name)
	}
}

func updateAliases(meta *elastic.ElasticsearchMetadata) {
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

func updateShards(meta *elastic.ElasticsearchMetadata) {
	client := elastic.GetClient(meta.Config.ID)

	//Shards
	var shardsChanged bool
	shards, err := client.GetPrimaryShards()
	if err != nil {
		log.Errorf("[%v], %v", meta.Config.Name, err)
		return
	}

	if meta.PrimaryShards == nil {
		shardsChanged = true
	} else {
		if shards != nil {
			for k, v := range *shards {
				v1, ok := (*meta.PrimaryShards)[k]
				if ok {
					if len(v) != len(v1) {
						shardsChanged = true
						break
					} else {
						for x,y:=range v{
							z1, ok := v1[x]
							if ok{
								if y.NodeID!=z1.NodeID{
									shardsChanged = true
									break
								}
							}else{
								shardsChanged = true
								break
							}

						}
					}
				} else {
					shardsChanged = true
					break
				}
			}
		}
	}

	if shardsChanged {
		//TOD locker
		meta.PrimaryShards = shards
		log.Tracef("cluster shards [%v] updated", meta.Config.Name)
	}
}

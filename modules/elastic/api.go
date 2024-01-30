/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"net/http"
)

func init() {
	api.HandleAPIMethod(api.GET, "/elasticsearch/metadata", GetMetadata)
	api.HandleAPIMethod(api.GET, "/elasticsearch/hosts", GetHosts)
}

func GetMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	result := util.MapStr{}

	elastic.WalkMetadata(func(key, value interface{}) bool {
		m := util.MapStr{}
		k := key.(string)
		if value == nil {
			return true
		}

		v, ok := value.(*elastic.ElasticsearchMetadata)
		if ok {
			m["major_version"] = v.GetMajorVersion()
			m["seed_hosts"] = v.GetSeedHosts()
			m["state"] = v.ClusterState
			m["topology_version"] = v.NodesTopologyVersion
			m["nodes"] = v.Nodes
			//m["indices"]=v.Indices
			m["health"] = v.Health
			m["aliases"] = v.Aliases
			//m["primary_shards"]=v.PrimaryShards
			m["available"] = v.IsAvailable()
			m["schema"] = v.GetSchema()
			m["config"] = v.Config
			m["last_success"] = v.LastSuccess()
			result[k] = m
		}
		return true
	})

	api.DefaultAPI.WriteJSON(w, result, http.StatusOK)

}

func GetHosts(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	result := util.MapStr{}

	elastic.WalkHosts(func(key, value interface{}) bool {
		k := key.(string)
		if value == nil {
			return true
		}

		v, ok := value.(*elastic.NodeAvailable)
		if ok {
			result[k] = util.MapStr{
				"host":            v.Host,
				"available":       v.IsAvailable(),
				"dead":            v.IsDead(),
				"last_check":      v.LastCheck(),
				"last_success":    v.LastSuccess(),
				"failure_tickets": v.FailureTickets(),
			}
		}
		return true
	})

	api.DefaultAPI.WriteJSON(w, result, http.StatusOK)

}
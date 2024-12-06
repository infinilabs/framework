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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"github.com/rubyniu105/framework/core/api"
	httprouter "github.com/rubyniu105/framework/core/api/router"
	"github.com/rubyniu105/framework/core/elastic"
	"github.com/rubyniu105/framework/core/util"
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

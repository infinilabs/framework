/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
)

func Init() {
	handler := APIHandler{}
	api.HandleAPIMethod(api.POST, "/agent/instance", handler.createInstance)
	api.HandleAPIMethod(api.GET, "/agent/instance/_search", handler.searchInstance)
	api.HandleAPIMethod(api.PUT, "/agent/instance/:instance_id/_nodes", handler.updateInstanceNodes)
	api.HandleAPIMethod(api.GET, "/agent/instance/:instance_id", handler.getInstance)
	api.HandleAPIMethod(api.PUT, "/agent/instance/:instance_id", handler.updateInstance)
	api.HandleAPIMethod(api.DELETE, "/agent/instance/:instance_id", handler.deleteInstance)
	api.HandleAPIMethod(api.POST, "/agent/instance/:instance_id/_heartbeat", handler.heartbeat)
	api.HandleAPIMethod(api.POST, "/agent/instance/:instance_id/_assign_task", handler.setTaskToInstance)
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac/enum"
)

func Init() {
	handler := APIHandler{}
	api.HandleAPIMethod(api.POST, "/agent/instance", handler.createInstance)
	api.HandleAPIMethod(api.GET, "/agent/instance/_search", handler.RequirePermission(handler.searchInstance, enum.PermissionAgentInstanceRead))
	api.HandleAPIMethod(api.GET, "/agent/instance/:instance_id", handler.getInstance)
	api.HandleAPIMethod(api.PUT, "/agent/instance/:instance_id", handler.updateInstance)
	api.HandleAPIMethod(api.DELETE, "/agent/instance/:instance_id", handler.RequirePermission(handler.deleteInstance, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.POST, "/agent/instance/:instance_id/_heartbeat", handler.heartbeat)
	api.HandleAPIMethod(api.POST, "/agent/instance/:instance_id/_set_task", handler.RequirePermission(handler.setTaskToInstance, enum.PermissionAgentInstanceWrite))
	api.HandleAPIMethod(api.POST, "/agent/instance/_stats", handler.RequirePermission(handler.getInstanceStats, enum.PermissionAgentInstanceRead))
	api.HandleAPIMethod(api.POST, "/agent/instance/_enroll", handler.RequirePermission(handler.enrollInstance, enum.PermissionAgentInstanceWrite))

	api.HandleAPIMethod(api.GET, "/agent/log/node/:node_id/files", handler.getLogFilesByNode)
	api.HandleAPIMethod(api.POST, "/agent/log/node/:node_id/_scroll", handler.getLogFileContent)

}

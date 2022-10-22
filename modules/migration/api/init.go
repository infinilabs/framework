/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import "infini.sh/framework/core/api"

func Init() {
	handler := APIHandler{}
	api.HandleAPIMethod(api.GET, "/migration/data/_search",  handler.RequireLogin(handler.searchDataMigrationTask))
	api.HandleAPIMethod(api.POST, "/migration/data", handler.RequireLogin(handler.createDataMigrationTask))
	api.HandleAPIMethod(api.POST, "/migration/data/_validate",  handler.RequireLogin(handler.validateDataMigration))

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/index/:index/_partition", handler.getIndexPartitionInfo)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/index/:index/_count", handler.countDocuments)
	api.HandleAPIMethod(api.GET, "/migration/data/:task_id/_start",  handler.RequireLogin(handler.startDataMigration))
	api.HandleAPIMethod(api.GET, "/migration/data/:task_id/_stop",  handler.RequireLogin(handler.stopDataMigrationTask))
	//api.HandleAPIMethod(api.GET, "/migration/data/:task_id", handler.getMigrationTask)
	api.HandleAPIMethod(api.GET, "/migration/data/:task_id/info",  handler.RequireLogin(handler.getDataMigrationTaskInfo))
	api.HandleAPIMethod(api.GET, "/migration/data/:task_id/info/index",  handler.RequireLogin(handler.getDataMigrationTaskOfIndex))
	api.HandleAPIMethod(api.PUT, "/migration/data/:task_id/status",  handler.RequireLogin(handler.updateDataMigrationTaskStatus))

}

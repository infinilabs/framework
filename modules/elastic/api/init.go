package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/modules/elastic/common"
)

func Init(cfg common.ModuleConfig) {
	clusterAPI := APIHandler{Config: cfg}

	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/health", clusterAPI.GetClusterHealth)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/metrics", clusterAPI.HandleClusterMetricsAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/node_metrics", clusterAPI.HandleNodeMetricsAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/", clusterAPI.HandleCreateClusterAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/status", clusterAPI.GetClusterStatusAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id", clusterAPI.HandleGetClusterAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/nodes", clusterAPI.HandleGetNodesAction)
	api.HandleAPIMethod(api.PUT, "/elasticsearch/:id", clusterAPI.HandleUpdateClusterAction)
	api.HandleAPIMethod(api.DELETE, "/elasticsearch/:id", clusterAPI.HandleDeleteClusterAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/search_template", clusterAPI.HandleCreateSearchTemplateAction)
	api.HandleAPIMethod(api.PUT, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleUpdateSearchTemplateAction)
	api.HandleAPIMethod(api.DELETE, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleDeleteSearchTemplateAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/search_template", clusterAPI.HandleSearchSearchTemplateAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleGetSearchTemplateAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/search_template_history/_search", clusterAPI.HandleSearchSearchTemplateHistoryAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/_render/template", clusterAPI.HandleRenderTemplateAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/_search/template", clusterAPI.HandleSearchTemplateAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/alias", clusterAPI.HandleAliasAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/alias", clusterAPI.HandleGetAliasAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/saved_objects/view", clusterAPI.HandleCreateViewAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/saved_objects/_find", clusterAPI.HandleGetViewListAction)
	api.HandleAPIMethod(api.DELETE, "/elasticsearch/:id/saved_objects/view/:viewID", clusterAPI.HandleDeleteViewAction)
	api.HandleAPIMethod(api.PUT, "/elasticsearch/:id/saved_objects/view/:viewID", clusterAPI.HandleUpdateViewAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/internal/view-management/resolve_index/:wild", clusterAPI.HandleResolveIndexAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/saved_objects/_bulk_get", clusterAPI.HandleBulkGetViewAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/view/_fields_for_wildcard", clusterAPI.HandleGetFieldCapsAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/search/ese", clusterAPI.HandleEseSearchAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/suggestions/values/:index", clusterAPI.HandleValueSuggestionAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/setting", clusterAPI.HandleSettingAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/setting/:key", clusterAPI.HandleGetSettingAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/_proxy", clusterAPI.HandleProxyAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/try_connect", clusterAPI.HandleTestConnectionAction)


	api.HandleAPIMethod(api.GET, "/elasticsearch/metadata", clusterAPI.GetMetadata)
	api.HandleAPIMethod(api.GET, "/elasticsearch/hosts", clusterAPI.GetHosts)
}

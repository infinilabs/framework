package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/modules/elastic/common"
)

func Init(cfg common.ModuleConfig) {
	clusterAPI := APIHandler{Config: cfg}

	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/health", clusterAPI.GetClusterHealth)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/metrics", clusterAPI.HandleMetricsSummaryAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/cluster_metrics", clusterAPI.HandleClusterMetricsAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/node_metrics", clusterAPI.HandleNodeMetricsAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/index_metrics", clusterAPI.HandleIndexMetricsAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/queue_metrics", clusterAPI.HandleQueueMetricsAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/storage_metrics", clusterAPI.HandleGetStorageMetricAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/", clusterAPI.HandleCreateClusterAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/status", clusterAPI.GetClusterStatusAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id", clusterAPI.HandleGetClusterAction)
	//api.HandleAPIMethod(api.GET, "/elasticsearch/:id/nodes/kv", clusterAPI.HandleGetNodesAction)
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
	api.HandleAPIMethod(api.DELETE, "/elasticsearch/:id/saved_objects/view/:view_id", clusterAPI.HandleDeleteViewAction)
	api.HandleAPIMethod(api.PUT, "/elasticsearch/:id/saved_objects/view/:view_id", clusterAPI.HandleUpdateViewAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/internal/view-management/resolve_index/:wild", clusterAPI.HandleResolveIndexAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/saved_objects/_bulk_get", clusterAPI.HandleBulkGetViewAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/view/_fields_for_wildcard", clusterAPI.HandleGetFieldCapsAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/search/ese", clusterAPI.HandleEseSearchAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/search/trace_id", clusterAPI.HandleTraceIDSearchAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/suggestions/values/:index", clusterAPI.HandleValueSuggestionAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/setting", clusterAPI.HandleSettingAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/setting/:key", clusterAPI.HandleGetSettingAction)

	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/_proxy", clusterAPI.HandleProxyAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/try_connect", clusterAPI.HandleTestConnectionAction)


	api.HandleAPIMethod(api.GET, "/elasticsearch/metadata", clusterAPI.GetMetadata)
	api.HandleAPIMethod(api.GET, "/elasticsearch/hosts", clusterAPI.GetHosts)

	api.HandleAPIMethod(api.POST, "/elasticsearch/cluster/_search", clusterAPI.SearchClusterMetadata)
	api.HandleAPIMethod(api.POST, "/elasticsearch/cluster/info", clusterAPI.FetchClusterInfo)

	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/overview/treemap", clusterAPI.ClusterOverTreeMap)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/info", clusterAPI.GetClusterInfo)
	api.HandleAPIMethod(api.GET, "/elasticsearch/node/_search", clusterAPI.SearchNodeMetadata)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/nodes", clusterAPI.GetClusterNodes)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/indices", clusterAPI.GetClusterIndices)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/node/:node_id/info", clusterAPI.GetNodeInfo)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/node/:node_id/metrics", clusterAPI.GetSingleNodeMetrics)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/node/:node_id/indices", clusterAPI.getNodeIndices)
	api.HandleAPIMethod(api.GET, "/elasticsearch/index/_search", clusterAPI.SearchIndexMetadata)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/index/:index/metrics", clusterAPI.GetSingleIndexMetrics)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/index/:index/info", clusterAPI.GetIndexInfo)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/index/:index/nodes", clusterAPI.getIndexNodes)

	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/trace_template", clusterAPI.HandleSearchTraceTemplateAction)
	api.HandleAPIMethod(api.GET, "/elasticsearch/:id/trace_template/:template_id", clusterAPI.HandleGetTraceTemplateAction)
	api.HandleAPIMethod(api.POST, "/elasticsearch/:id/trace_template", clusterAPI.HandleCrateTraceTemplateAction)
	api.HandleAPIMethod(api.PUT, "/elasticsearch/:id/trace_template/:template_id", clusterAPI.HandleSaveTraceTemplateAction)
	api.HandleAPIMethod(api.DELETE, "/elasticsearch/:id/trace_template/:template_id", clusterAPI.HandleDeleteTraceTemplateAction)
}

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/ui"
	"infini.sh/framework/modules/elastic/common"
)

func Init(cfg common.ModuleConfig) {
	clusterAPI := APIHandler{Config: cfg}

	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/health", clusterAPI.GetClusterHealth)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/metrics", clusterAPI.HandleClusterMetricsAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/", clusterAPI.HandleCreateClusterAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/status", clusterAPI.GetClusterStatusAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id", clusterAPI.HandleGetClusterAction)
	ui.HandleUIMethod(api.PUT, "/elasticsearch/:id", clusterAPI.HandleUpdateClusterAction)
	ui.HandleUIMethod(api.DELETE, "/elasticsearch/:id", clusterAPI.HandleDeleteClusterAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/search_template", clusterAPI.HandleCreateSearchTemplateAction)
	ui.HandleUIMethod(api.PUT, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleUpdateSearchTemplateAction)
	ui.HandleUIMethod(api.DELETE, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleDeleteSearchTemplateAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/search_template", clusterAPI.HandleSearchSearchTemplateAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleGetSearchTemplateAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/search_template_history/_search", clusterAPI.HandleSearchSearchTemplateHistoryAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/_render/template", clusterAPI.HandleRenderTemplateAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/_search/template", clusterAPI.HandleSearchTemplateAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/alias", clusterAPI.HandleAliasAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/alias", clusterAPI.HandleGetAliasAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/saved_objects/view", clusterAPI.HandleCreateIndexPatternAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/saved_objects/_find", clusterAPI.HandleGetIndexPatternListAction)
	ui.HandleUIMethod(api.DELETE, "/elasticsearch/:id/saved_objects/view/:indexPatternID", clusterAPI.HandleDeleteIndexPatternAction)
	ui.HandleUIMethod(api.PUT, "/elasticsearch/:id/saved_objects/view/:indexPatternID", clusterAPI.HandleUpdateIndexPatternAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/internal/view-management/resolve_index/:wild", clusterAPI.HandleResolveIndexAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/saved_objects/_bulk_get", clusterAPI.HandleBulkGetIndexPatternAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/view/_fields_for_wildcard", clusterAPI.HandleGetFieldCapsAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/search/ese", clusterAPI.HandleEseSearchAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/suggestions/values/:index", clusterAPI.HandleValueSuggestionAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/setting", clusterAPI.HandleSettingAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/setting/:key", clusterAPI.HandleGetSettingAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/_proxy", clusterAPI.HandleProxyAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/try_connect", clusterAPI.HandleTestConnectionAction)

}

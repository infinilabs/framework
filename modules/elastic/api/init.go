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
	ui.HandleUIMethod(api.POST, "/elasticsearch", clusterAPI.HandleCreateClusterAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id", clusterAPI.HandleGetClusterAction)
	ui.HandleUIMethod(api.PUT, "/elasticsearch/:id", clusterAPI.HandleUpdateClusterAction)
	ui.HandleUIMethod(api.DELETE, "/elasticsearch/:id", clusterAPI.HandleDeleteClusterAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)
	ui.HandleUIMethod(api.POST, "/elasticsearch/_search", clusterAPI.HandleSearchClusterAction)

	ui.HandleUIMethod(api.POST, "/elasticsearch/:id/search_template", clusterAPI.HandleCreateSearchTemplateAction)
	ui.HandleUIMethod(api.PUT, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleUpdateSearchTemplateAction)
	ui.HandleUIMethod(api.DELETE, "/elasticsearch/:id/search_template/:template_id", clusterAPI.HandleDeleteSearchTemplateAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/search_template/_get/:template_id", clusterAPI.HandleGetSearchTemplateAction)
	ui.HandleUIMethod(api.GET, "/elasticsearch/:id/search_template/_search", clusterAPI.HandleSearchSearchTemplateAction)

}

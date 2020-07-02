package admin

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/modules/ui/admin/console"
	"infini.sh/framework/modules/ui/admin/dashboard"
	"infini.sh/framework/modules/ui/common"
	"net/http"
)

type AdminUI struct {
	api.Handler
}

func (h AdminUI) DashboardAction(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	dashboard.Index(w, r)
}

func (h AdminUI) ConsolePageAction(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	console.Index(w, r)
}

func (h AdminUI) ExplorePageAction(w http.ResponseWriter, r *http.Request) {
	common.Message(w, r, "hello", "world")
	//explore.Index(w, r)
}

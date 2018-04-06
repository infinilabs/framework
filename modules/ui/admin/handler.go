package admin

import (
	"github.com/infinitbyte/framework/core/api"
	"github.com/infinitbyte/framework/core/api/router"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/modules/ui/admin/console"
	"github.com/infinitbyte/framework/modules/ui/admin/dashboard"
	"github.com/infinitbyte/framework/modules/ui/admin/setting"
	"github.com/infinitbyte/framework/modules/ui/common"
	"gopkg.in/yaml.v2"
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

func (h AdminUI) SettingPageAction(w http.ResponseWriter, r *http.Request) {

	o, _ := yaml.Marshal(global.Env().RuntimeConfig)
	setting.Setting(w, r, string(o))
}

func (h AdminUI) UpdateSettingAction(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	body, _ := h.GetRawBody(r)
	yaml.Unmarshal(body, global.Env().RuntimeConfig) //TODO extract method, save to file

	o, _ := yaml.Marshal(global.Env().RuntimeConfig)

	setting.Setting(w, r, string(o))
}

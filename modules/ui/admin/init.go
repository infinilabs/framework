/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admin

import (
	"github.com/infinitbyte/framework/core/api"
	"github.com/infinitbyte/framework/core/ui"
	"github.com/infinitbyte/framework/modules/ui/admin/ajax"
	"github.com/infinitbyte/framework/modules/ui/common"
)

// InitUI register ui handlers
func InitUI() {
	//Nav init
	common.RegisterNav("Console", "Console", "/admin/console/")
	//common.RegisterNav("Dashboard", "Dashboard", "/admin/")
	//common.RegisterNav("Explore","Explore","/ui/explore/")
	//common.RegisterNav("Setting", "Setting", "/admin/setting/")

	//UI pages init
	adminUI := AdminUI{}

	ui.HandleUIMethod(api.GET, "/admin/", api.NeedPermission(api.PERMISSION_ADMIN_MINIMAL, adminUI.DashboardAction))
	ui.HandleUIMethod(api.POST, "/admin/setting/", api.NeedPermission(api.PERMISSION_ADMIN_MINIMAL, adminUI.UpdateSettingAction))
	ui.HandleUIMethod(api.GET, "/admin/dashboard/", api.NeedPermission(api.PERMISSION_ADMIN_MINIMAL, adminUI.DashboardAction))
	ui.HandleUIMethod(api.GET, "/admin/console/", api.NeedPermission(api.PERMISSION_ADMIN_MINIMAL, adminUI.ConsolePageAction))

	ui.HandleUIFunc("/admin/explore/", adminUI.ExplorePageAction)
	ui.HandleUIFunc("/admin/setting/", adminUI.SettingPageAction)

	//Ajax
	ajax := ajax.Ajax{}
	ui.HandleUIFunc("/setting/logger", ajax.LoggingSettingAction)
	ui.HandleUIFunc("/setting/logger/", ajax.LoggingSettingAction)

}

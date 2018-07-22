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

package ui

import (
	log "github.com/cihub/seelog"
	uis "github.com/infinitbyte/framework/core/api"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/fs"
	"github.com/infinitbyte/framework/core/logger"
	"github.com/infinitbyte/framework/core/ui"
	"github.com/infinitbyte/framework/core/ui/websocket"
	"github.com/infinitbyte/framework/modules/ui/admin"
	"github.com/infinitbyte/framework/modules/ui/common"
	"github.com/infinitbyte/framework/modules/ui/public"
	"github.com/infinitbyte/framework/static"
	_ "net/http/pprof"
)

type UIModule struct {
}

func LoggerReceiver(message string, level log.LogLevel, context log.LogContextInterface) {

	websocket.BroadcastMessage(message)
}

func (module UIModule) Name() string {
	return "Web"
}

func (module UIModule) Start(cfg *Config) {

	adminConfig := common.UIConfig{}
	cfg.Unpack(&adminConfig)

	uis.EnableAuth(adminConfig.AuthConfig.Enabled)

	//init admin ui
	admin.InitUI()

	//init public ui
	public.InitUI(adminConfig.AuthConfig)

	//register websocket logger
	logger.RegisterWebsocketHandler(LoggerReceiver)

	fs.RegisterFS(static.StaticFS{StaticFolder: "static", TrimLeftPath: "", CheckLocalFirst: true})
	go func() {
		ui.StartUI(cfg)
	}()

}

func (module UIModule) Stop() error {

	return nil
}

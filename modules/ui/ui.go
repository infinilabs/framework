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
	"compress/gzip"
	log "github.com/cihub/seelog"
	uis "infini.sh/framework/core/api"
	"infini.sh/framework/core/api/websocket"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/logger"
	_ "net/http/pprof"
)

type UIModule struct {
	uiConfig *uis.UIConfig
}

func LoggerReceiver(message string, level log.LogLevel, context log.LogContextInterface) {
	websocket.BroadcastMessage(message)
}

func (module *UIModule) Name() string {
	return "web"
}

func (module *UIModule) Setup() {


	module.uiConfig = &uis.UIConfig{
		Gzip: GzipConfig{
			Enabled: true,
			Level:   gzip.BestCompression,
		},
		WebsocketConfig: WebsocketConfig{
			Enabled: true,
			SkipHostVerify: true,
		},
	}

	env.ParseConfig("web", module.uiConfig)

	if module.uiConfig.Enabled {

		uis.EnableAuth(module.uiConfig.AuthConfig.Enabled)

		//register websocket logger
		logger.RegisterWebsocketHandler(LoggerReceiver)
	}

}

func (module *UIModule) Start() error {
	if module.uiConfig.Enabled {
		uis.StartUI(module.uiConfig)
	}
	return nil
}

func (module *UIModule) Stop() error {

	if module.uiConfig.Enabled {
		uis.StopUI(module.uiConfig)
	}

	return nil
}

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package web

import (
	log "github.com/cihub/seelog"
	uis "infini.sh/framework/core/api"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logging/logger"
	_ "net/http/pprof"
)

type WebModule struct {
}

func LoggerReceiver(message string, level log.LogLevel, context log.LogContextInterface) {
	websocket.BroadcastMessage(message)
}

func (module *WebModule) Name() string {
	return "web"
}

func (module *WebModule) Setup() {

	if global.Env().SystemConfig.WebAppConfig.Enabled {

		uis.EnableAuth(global.Env().SystemConfig.WebAppConfig.AuthConfig.Enabled)

		//register websocket logger
		logger.RegisterWebsocketHandler(LoggerReceiver)
	}
}

func (module *WebModule) Start() error {
	if global.Env().SystemConfig.WebAppConfig.Enabled {
		uis.StartWeb(global.Env().SystemConfig.WebAppConfig)
	}
	return nil
}

func (module *WebModule) Stop() error {
	if global.Env().SystemConfig.WebAppConfig.Enabled {
		uis.StopWeb(global.Env().SystemConfig.WebAppConfig)
	}
	return nil
}

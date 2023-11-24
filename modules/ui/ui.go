/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package ui

import (
	log "github.com/cihub/seelog"
	uis "infini.sh/framework/core/api"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logger"
	_ "net/http/pprof"
)

type UIModule struct {
}

func LoggerReceiver(message string, level log.LogLevel, context log.LogContextInterface) {
	websocket.BroadcastMessage(message)
}

func (module *UIModule) Name() string {
	return "web"
}

func (module *UIModule) Setup() {

	if global.Env().SystemConfig.WebAppConfig.Enabled {

		uis.EnableAuth(global.Env().SystemConfig.WebAppConfig.AuthConfig.Enabled)

		//register websocket logger
		logger.RegisterWebsocketHandler(LoggerReceiver)
	}
}

func (module *UIModule) Start() error {
	if global.Env().SystemConfig.WebAppConfig.Enabled {
		uis.StartUI(global.Env().SystemConfig.WebAppConfig)
	}
	return nil
}

func (module *UIModule) Stop() error {
	if global.Env().SystemConfig.WebAppConfig.Enabled {
		uis.StopUI(global.Env().SystemConfig.WebAppConfig)
	}
	return nil
}

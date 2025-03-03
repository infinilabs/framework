// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package web

import (
	log "github.com/cihub/seelog"
	uis "infini.sh/framework/core/api"
	uic "infini.sh/framework/core/api/common"
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

		uic.EnableAuth(global.Env().SystemConfig.WebAppConfig.AuthConfig.Enabled)

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

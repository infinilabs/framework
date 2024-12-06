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
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package logger

import (
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/util"
	"github.com/ryanuber/go-glob"
	"strings"
)

// WebsocketReceiver is a struct of websocket log receiver, which implements seelog.CustomReceiver
type WebsocketReceiver struct {
	config            *config.LoggingConfig
	minLogLevel       log.LogLevel
	handleMessageFunc func(message string, level log.LogLevel, context log.LogContextInterface)
}

// ReceiveMessage impl how to receive log message
func (ar *WebsocketReceiver) ReceiveMessage(message string, level log.LogLevel, context log.LogContextInterface) error {
	//truncate huge message
	if len(message) > 300 {
		message = util.SubString(message, 0, 300) + "..."
	}

	f := context.Func()
	spl := strings.Split(f, ".")
	funcName := spl[len(spl)-1]

	if ar.config != nil {
		if level < ar.minLogLevel {
			return nil
		}

		if len(ar.config.FileFilterPattern) > 0 && ar.config.FileFilterPattern != "*" {
			if !glob.Glob(ar.config.FileFilterPattern, context.FileName()) {
				return nil
			}
		}
		if len(ar.config.FuncFilterPattern) > 0 && ar.config.FuncFilterPattern != "*" {
			if !glob.Glob(ar.config.FuncFilterPattern, funcName) {
				return nil
			}
		}
		if len(ar.config.MessageFilterPattern) > 0 && ar.config.MessageFilterPattern != "*" {
			if !glob.Glob(ar.config.MessageFilterPattern, message) {
				return nil
			}
		}
	}

	//push message to websocket
	if ar.handleMessageFunc != nil {
		ar.handleMessageFunc(message, level, context)
	}

	return nil
}

// AfterParse nothing to do here
func (ar *WebsocketReceiver) AfterParse(initArgs log.CustomReceiverInitArgs) error {
	return nil
}

// Flush logs
func (ar *WebsocketReceiver) Flush() {

}

// Close logs
func (ar *WebsocketReceiver) Close() error {
	return nil
}

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package logger

import (
	log "github.com/cihub/seelog"
	"github.com/ryanuber/go-glob"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
	"strings"
)

// WebsocketReceiver is a struct of websocket log receiver, which implements seelog.CustomReceiver
type WebsocketReceiver struct {
	config          *config.LoggingConfig
	minLogLevel     log.LogLevel
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

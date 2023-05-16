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

package logger

import (
	"fmt"
	"path"
	"strings"
	"sync"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/rotate"
	"infini.sh/framework/core/util"
)

var file string
var loggingLock sync.RWMutex
var loggingConfig *config.LoggingConfig

var oldQuoteStr = []byte("\"")
var newQuoteStr = []byte("”")

func createMyLevelFormatter(params string) log.FormatterFunc {
	return func(message string, level log.LogLevel, context log.LogContextInterface) interface{} {
		if util.ContainStr(message, "\"") {
			return util.UnsafeBytesToString(util.ReplaceByte(util.UnsafeStringToBytes(message), oldQuoteStr, newQuoteStr))
		}
		return message
	}
}

func init() {
	err := log.RegisterCustomFormatter("EscapedMsg", createMyLevelFormatter)
	if err != nil {
		panic(err)
	}
}

// SetLogging init set logging
func SetLogging(loggingCfg *config.LoggingConfig, appName string, baseDir string) {
	if loggingCfg == nil {
		panic("empty logging config")
	}
	if appName == "" {
		appName = "app"
	}
	loggingLock.Lock()
	loggingConfig = loggingCfg
	loggingLock.Unlock()

	if loggingConfig.FuncFilterPattern == "" {
		loggingConfig.FuncFilterPattern = "*"
	}
	if loggingConfig.FileFilterPattern == "" {
		loggingConfig.FileFilterPattern = "*"
	}
	if loggingConfig.LogLevel == "" {
		loggingConfig.LogLevel = "info"
	}
	if loggingConfig.PushLogLevel == "" {
		loggingConfig.PushLogLevel = "info"
	}

	consoleWriter, _ := NewConsoleWriter()

	format := "[%Date(01-02) %Time] [%LEV] [%File:%Line] %Msg%n"
	if loggingConfig.LogFormat != "" {
		format = loggingConfig.LogFormat
	}
	formatter, err := log.NewFormatter(format)
	if err != nil {
		fmt.Println(err)
	}

	l, _ := log.LogLevelFromString(strings.ToLower(loggingConfig.LogLevel))
	pushl, _ := log.LogLevelFromString(strings.ToLower(loggingConfig.PushLogLevel))

	//logging receivers
	consoleReceiver := NewFileReceiver(consoleWriter, l)
	consoleOutput, err := log.NewCustomReceiverDispatcherByValue(formatter, consoleReceiver, "console", log.CustomReceiverInitArgs{})
	receivers := []interface{}{consoleOutput}

	if !loggingConfig.DisableFileOutput {
		if baseDir != "" {
			file = path.Join(baseDir, appName+".log")
		} else {
			file = "./log/" + appName + ".log"
		}

		cfg1 := rotate.RotateConfig{
			Compress:     true,
			MaxFileAge:   0,
			MaxFileCount: 100,
			MaxFileSize:  1024,
		}
		fileHandler := rotate.GetFileHandler(file, cfg1)
		fileReceiver := NewFileReceiver(fileHandler, l)
		realtimeOutput, err := log.NewCustomReceiverDispatcherByValue(formatter, fileReceiver, "file", log.CustomReceiverInitArgs{})
		if err != nil {
			fmt.Println(err)
		}else{
			receivers = append(receivers, realtimeOutput)
		}
	}
	if loggingConfig.RealtimePushEnabled {
		realtimeReceiver := &WebsocketReceiver{config: loggingConfig, minLogLevel: pushl, handleMessageFunc: websocketHandler}
		realtimeOutput, err := log.NewCustomReceiverDispatcherByValue(formatter, realtimeReceiver, "websocket", log.CustomReceiverInitArgs{})
		if err != nil {
			fmt.Println(err)
		}else{
			receivers = append(receivers, realtimeOutput)
		}
	}

	root, err := log.NewSplitDispatcher(formatter, receivers)
	if err != nil {
		fmt.Println(err)
	}

	globalConstraints, err := log.NewMinMaxConstraints(log.TraceLvl, log.Off)
	if err != nil {
		panic(err)
	}

	exceptions := []*log.LogLevelException{}

	logger := log.NewAsyncLoopLogger(log.NewLoggerConfig(globalConstraints, exceptions, root))
	err = log.ReplaceLogger(logger)
	if err != nil {
		fmt.Println(err)
	}

}

// GetLoggingConfig return logging configs
func GetLoggingConfig() *config.LoggingConfig {
	loggingLock.RLock()
	defer loggingLock.RUnlock()
	return loggingConfig
}

// Flush is flush logs to output
func Flush() {
	log.Flush()
}

var websocketHandler func(message string, level log.LogLevel, context log.LogContextInterface)

// RegisterWebsocketHandler used to register websocket handler
func RegisterWebsocketHandler(func1 func(message string, level log.LogLevel, context log.LogContextInterface)) {

	websocketHandler = func1
	if func1 != nil {
		log.Debug("websocket logging ready")
	}
}

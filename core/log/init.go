package log

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/logger"
	defaultLog "log"
)

func init() {
	defaultLog.SetOutput(logger.EmptyLogger{})
	e := env.EmptyEnv()
	var (
		appName = e.GetAppLowercaseName()
		baseDir = e.GetLogDir()
	)
	logger.SetLogging(&e.SystemConfig.LoggingConfig, appName, baseDir )
}
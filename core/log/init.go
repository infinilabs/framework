package log

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/logger"
	defaultLog "log"
)

func init() {
	defaultLog.SetOutput(logger.EmptyLogger{})
	logger.SetLogging(env.EmptyEnv(), "info")
}
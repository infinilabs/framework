package log

// Package log provides the main logging entrypoint for the framework.
// It wraps the internal seelog implementation.

import (
	"infini.sh/framework/lib/seelog"
)

type LoggerInterface = seelog.LoggerInterface

// Logger is the main logger instance.
var Logger = seelog.Default

// Convenience wrappers for seelog functions
var (
	UseLogger     = seelog.UseLogger
	ReplaceLogger = seelog.ReplaceLogger
	Flush         = seelog.Flush

	Trace    = seelog.Trace
	Debug    = seelog.Debug
	Info     = seelog.Info
	Warn     = seelog.Warn
	Error    = seelog.Error
	Critical = seelog.Critical

	Tracef    = seelog.Tracef
	Debugf    = seelog.Debugf
	Infof     = seelog.Infof
	Warnf     = seelog.Warnf
	Errorf    = seelog.Errorf
	Criticalf = seelog.Criticalf
)

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"

	log "github.com/cihub/seelog"
)

// pipelinelog: processor logging with automatic trace-id correlation.
//
// Processors emit text diagnostics via seelog; the trace_id lives on the
// RECORD (Metadata.Labels["_trace_id"]), not the context — so plain
// log.Errorf cannot correlate to a specific record's flight trail.
// These helpers read the current record's trace_id from the context and
// prefix every message, making logs grep-able by trace_id:
//
//	pipelinelog.Warnf(ctx, "extract failed: %v", err)
//	// → "[tr-abc123] extract failed: timeout"
//
// The prefix format is stable: [tr-<id>] — log panels and grep use it.

// logPrefix extracts the trace prefix from the context's current record.
func logPrefix(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	rec, ok := CurrentRecord(ctx)
	if !ok {
		return ""
	}
	id := TraceID(rec)
	if id == "" {
		return ""
	}
	return "[" + id + "] "
}

func Tracef(ctx *Context, format string, args ...interface{}) {
	log.Trace(logPrefix(ctx) + sprintf(format, args...))
}

func Debugf(ctx *Context, format string, args ...interface{}) {
	log.Debug(logPrefix(ctx) + sprintf(format, args...))
}

func Infof(ctx *Context, format string, args ...interface{}) {
	log.Info(logPrefix(ctx) + sprintf(format, args...))
}

func Warnf(ctx *Context, format string, args ...interface{}) {
	log.Warn(logPrefix(ctx) + sprintf(format, args...))
}

func Errorf(ctx *Context, format string, args ...interface{}) {
	log.Error(logPrefix(ctx) + sprintf(format, args...))
}

// sprintf avoids importing fmt at the top level for a one-liner.
func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

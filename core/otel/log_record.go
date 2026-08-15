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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

// Package otel implements the canonical log data model of INFINI's data
// pipeline, aligned with the OpenTelemetry Logs Data Model:
//
//	https://opentelemetry.io/docs/specs/otel/logs/data-model/
//
// Structural standard  : OTel LogRecord (top-level typed fields + Resource
//	                    /Attributes attribute collections).
// Field naming standard: lowercase snake_case keys ("log_level",
// "host_name", ...), matching the house convention. The
// field_standardize processor (enterprise) can switch attribute naming to
// dotted OTel semconv ("log.level") or keep nested maps when needed.
//
// The in-memory working unit of the pipeline stays framework's
// event.Event: Attributes live in event.Fields, stable source attributes
// (Resource) live in event.Meta["resource"] so they are not repeated per
// record on the wire. LogRecord is the boundary type used when decoding
// (OTLP intake) or encoding (OTLP export) and as the model reference.
package otel

import (
	"time"

	"infini.sh/framework/core/util"
)

// Canonical snake_case keys of the OTel top-level fields, as they appear
// inside event.Fields (i.e. the LogRecord's Attributes collection).
const (
	FieldTimestamp         = "timestamp"
	FieldObservedTimestamp = "observed_timestamp"
	FieldLogLevel          = "log_level"      // OTel SeverityText, original string
	FieldSeverityNumber    = "severity_number" // OTel SeverityNumber, 1-24
	FieldMessage           = "message"         // OTel Body
	FieldEventName         = "event_name"
	FieldTraceID           = "trace_id"
	FieldSpanID            = "span_id"
	FieldTraceFlags        = "trace_flags"

	// MetaResourceKey is the event.Meta key under which the stable
	// Resource attribute collection (host_name, service_name, ...) is
	// carried, separated from per-record Attributes.
	MetaResourceKey = "resource"
)

// Common Resource attribute keys (stable per source, snake_case).
const (
	ResourceHostName        = "host_name"
	ResourceHostIP          = "host_ip"
	ResourceServiceName     = "service_name"
	ResourceServiceVersion  = "service_version"
	ResourceProcessPID      = "process_pid"
	ResourceProcessName     = "process_name"
	ResourceCloudProvider   = "cloud_provider"
	ResourceCloudRegion     = "cloud_region"
	ResourceCloudAccountID  = "cloud_account_id"
)

// OTel SeverityNumber scale (see the Logs Data Model): 1-24, the 9 "well
// known" levels land on 1,5,9,13,17,21.
const (
	SeverityTrace1 int32 = 1
	SeverityDebug  int32 = 5
	SeverityInfo   int32 = 9
	SeverityWarn   int32 = 13
	SeverityError  int32 = 17
	SeverityFatal  int32 = 21
)

// LogRecord mirrors the OpenTelemetry LogRecord.
type LogRecord struct {
	// Time when the event occurred, measured by the origin clock.
	Timestamp time.Time
	// Time when the collection system observed the event.
	ObservedTimestamp time.Time

	TraceID    string // W3C trace id, hex
	SpanID     string // W3C span id, hex
	TraceFlags string

	SeverityText   string      // original severity string, e.g. "ERROR"
	SeverityNumber int32       // normalized severity, 1-24
	Body           interface{} // log payload, usually the message string
	EventName      string

	// Resource describes the source of the log; it is identical across
	// records coming from the same source (host/service/process).
	Resource util.MapStr
	// Attributes carries additional per-event information.
	Attributes util.MapStr
}

// SeverityNumberFromText maps a severity text to the OTel SeverityNumber
// scale, tolerating common variants ("WARN", "warning", "err", "fatal",
// "critical", numeric strings). Unknown text maps to SeverityInfo.
func SeverityNumberFromText(text string) int32 {
	switch normalizeSeverity(text) {
	case "":
		return 0
	case "trace", "verbose":
		return SeverityTrace1
	case "debug":
		return SeverityDebug
	case "info", "information", "notice":
		return SeverityInfo
	case "warn", "warning":
		return SeverityWarn
	case "err", "error":
		return SeverityError
	case "fatal", "critical", "crit", "emerg", "alert", "panic":
		return SeverityFatal
	default:
		return SeverityInfo
	}
}

func normalizeSeverity(text string) string {
	t := text
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c >= 'A' && c <= 'Z' {
			t = t[:i] + string(c-'A'+'a') + t[i+1:]
		}
	}
	return t
}

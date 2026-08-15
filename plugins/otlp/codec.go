// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
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

// Package otlp implements the OTLP (OpenTelemetry Protocol) wire codec
// between framework events and OTLP protobuf messages, used by the
// otlp_export processor (agent egress) and the gateway OTLP intake
// module. This is the only place besides the export processor that
// should import the OTLP proto packages; the framework core stays free
// of this dependency.
package otlp

import (
	"encoding/hex"
	"fmt"
	"time"

	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logsdata "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/otel"
	"infini.sh/framework/core/util"
)

func nanoToTime(n uint64) time.Time {
	return time.Unix(0, int64(n)).UTC()
}

// BuildExportRequest groups events by their Resource and renders them as
// one OTLP ExportLogsServiceRequest.
func BuildExportRequest(events []*event.Event) *logsv1.ExportLogsServiceRequest {
	req := &logsv1.ExportLogsServiceRequest{}
	type group struct {
		rl *logsdata.ResourceLogs
	}
	groups := map[string]*group{}

	for _, e := range events {
		if e == nil {
			continue
		}
		rec := otel.FromEvent(e)
		key := "default"
		var resAttrs []*commonv1.KeyValue
		if len(rec.Resource) > 0 {
			key = string(util.MustToJSONBytes(rec.Resource))
			resAttrs = mapToAttributes(rec.Resource)
		}
		g, ok := groups[key]
		if !ok {
			g = &group{rl: &logsdata.ResourceLogs{
				Resource: &resourcev1.Resource{Attributes: resAttrs},
				ScopeLogs: []*logsdata.ScopeLogs{{
					LogRecords: []*logsdata.LogRecord{},
				}},
			}}
			groups[key] = g
			req.ResourceLogs = append(req.ResourceLogs, g.rl)
		}
		g.rl.ScopeLogs[0].LogRecords = append(g.rl.ScopeLogs[0].LogRecords, toLogRecord(rec))
	}
	return req
}

// RecordsFromExportRequest decodes an OTLP export request back into
// framework events (used by the gateway intake).
func RecordsFromExportRequest(req *logsv1.ExportLogsServiceRequest) []*event.Event {
	var out []*event.Event
	for _, rl := range req.GetResourceLogs() {
		resource := attributesToMap(rl.GetResource().GetAttributes())
		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				rec := fromLogRecord(lr)
				rec.Resource = resource
				out = append(out, rec.ToEvent())
			}
		}
	}
	return out
}

func toLogRecord(rec *otel.LogRecord) *logsdata.LogRecord {
	lr := &logsdata.LogRecord{
		SeverityText: rec.SeverityText,
		EventName:    rec.EventName,
	}
	if !rec.Timestamp.IsZero() {
		lr.TimeUnixNano = uint64(rec.Timestamp.UnixNano())
	}
	if !rec.ObservedTimestamp.IsZero() {
		lr.ObservedTimeUnixNano = uint64(rec.ObservedTimestamp.UnixNano())
	}
	if rec.SeverityNumber != 0 {
		lr.SeverityNumber = logsdata.SeverityNumber(rec.SeverityNumber)
	} else if rec.SeverityText != "" {
		lr.SeverityNumber = logsdata.SeverityNumber(otel.SeverityNumberFromText(rec.SeverityText))
	}
	if rec.Body != nil {
		lr.Body = toAnyValue(rec.Body)
	}
	if id, err := hex.DecodeString(rec.TraceID); err == nil && len(id) > 0 {
		lr.TraceId = id
	}
	if id, err := hex.DecodeString(rec.SpanID); err == nil && len(id) > 0 {
		lr.SpanId = id
	}
	for k, v := range rec.Attributes {
		if isTypedField(k) {
			continue
		}
		lr.Attributes = append(lr.Attributes, &commonv1.KeyValue{Key: k, Value: toAnyValue(v)})
	}
	return lr
}

func fromLogRecord(lr *logsdata.LogRecord) *otel.LogRecord {
	rec := &otel.LogRecord{
		SeverityText:   lr.GetSeverityText(),
		SeverityNumber: int32(lr.GetSeverityNumber()),
		EventName:      lr.GetEventName(),
		Attributes:     util.MapStr{},
	}
	if ts := lr.GetTimeUnixNano(); ts > 0 {
		rec.Timestamp = nanoToTime(ts)
	}
	if ts := lr.GetObservedTimeUnixNano(); ts > 0 {
		rec.ObservedTimestamp = nanoToTime(ts)
	}
	if b := lr.GetBody(); b != nil {
		rec.Body = fromAnyValue(b)
	}
	if len(lr.GetTraceId()) > 0 {
		rec.TraceID = hex.EncodeToString(lr.GetTraceId())
	}
	if len(lr.GetSpanId()) > 0 {
		rec.SpanID = hex.EncodeToString(lr.GetSpanId())
	}
	for _, kv := range lr.GetAttributes() {
		if kv.GetValue() == nil {
			continue
		}
		rec.Attributes[kv.GetKey()] = fromAnyValue(kv.GetValue())
	}
	// re-materialize the typed fields from the model projection
	if rec.SeverityText != "" {
		rec.Attributes[otel.FieldLogLevel] = rec.SeverityText
	}
	if rec.Body != nil {
		rec.Attributes[otel.FieldMessage] = rec.Body
	}
	if rec.EventName != "" {
		rec.Attributes[otel.FieldEventName] = rec.EventName
	}
	if rec.TraceID != "" {
		rec.Attributes[otel.FieldTraceID] = rec.TraceID
	}
	if rec.SpanID != "" {
		rec.Attributes[otel.FieldSpanID] = rec.SpanID
	}
	if rec.SeverityNumber != 0 {
		rec.Attributes[otel.FieldSeverityNumber] = rec.SeverityNumber
	}
	return rec
}

func isTypedField(key string) bool {
	switch key {
	case otel.FieldLogLevel, otel.FieldSeverityNumber, otel.FieldMessage,
		otel.FieldEventName, otel.FieldTraceID, otel.FieldSpanID,
		otel.FieldTraceFlags, otel.FieldObservedTimestamp, otel.FieldTimestamp:
		return true
	}
	return false
}

func toAnyValue(v interface{}) *commonv1.AnyValue {
	switch val := v.(type) {
	case string:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: val}}
	case bool:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: val}}
	case int:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: int64(val)}}
	case int32:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: int64(val)}}
	case int64:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: val}}
	case float32:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_DoubleValue{DoubleValue: float64(val)}}
	case float64:
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_DoubleValue{DoubleValue: val}}
	case []string:
		arr := &commonv1.ArrayValue{}
		for _, s := range val {
			arr.Values = append(arr.Values, &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: s}})
		}
		return &commonv1.AnyValue{Value: &commonv1.AnyValue_ArrayValue{ArrayValue: arr}}
	case util.MapStr:
		return mapToKvlistValue(val)
	}
	return &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: fmt.Sprintf("%v", v)}}
}

func fromAnyValue(v *commonv1.AnyValue) interface{} {
	switch val := v.GetValue().(type) {
	case *commonv1.AnyValue_StringValue:
		return val.StringValue
	case *commonv1.AnyValue_BoolValue:
		return val.BoolValue
	case *commonv1.AnyValue_IntValue:
		return val.IntValue
	case *commonv1.AnyValue_DoubleValue:
		return val.DoubleValue
	case *commonv1.AnyValue_ArrayValue:
		out := make([]interface{}, 0, len(val.ArrayValue.GetValues()))
		for _, item := range val.ArrayValue.GetValues() {
			out = append(out, fromAnyValue(item))
		}
		return out
	case *commonv1.AnyValue_KvlistValue:
		return attributesToMap(val.KvlistValue.GetValues())
	case *commonv1.AnyValue_BytesValue:
		return val.BytesValue
	}
	return nil
}

func mapToAttributes(m util.MapStr) []*commonv1.KeyValue {
	attrs := make([]*commonv1.KeyValue, 0, len(m))
	for k, v := range m {
		attrs = append(attrs, &commonv1.KeyValue{Key: k, Value: toAnyValue(v)})
	}
	return attrs
}

func mapToKvlistValue(m util.MapStr) *commonv1.AnyValue {
	return &commonv1.AnyValue{Value: &commonv1.AnyValue_KvlistValue{
		KvlistValue: &commonv1.KeyValueList{Values: mapToAttributes(m)},
	}}
}

func attributesToMap(kvs []*commonv1.KeyValue) util.MapStr {
	m := util.MapStr{}
	for _, kv := range kvs {
		if kv.GetValue() == nil {
			continue
		}
		m[kv.GetKey()] = fromAnyValue(kv.GetValue())
	}
	return m
}

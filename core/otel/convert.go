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

package otel

import (
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// FromEvent projects a framework event.Event onto the canonical OTel
// LogRecord model.
//
// Mapping (see package doc):
//
//	event.Timestamp          -> Timestamp
//	event.Fields             -> Attributes (plus the typed fields below)
//	event.Meta["resource"]   -> Resource
//	Fields["log_level"]      -> SeverityText
//	Fields["severity_number"]-> SeverityNumber
//	Fields["message"]        -> Body
//	Fields["event_name"]     -> EventName
//	Fields["trace_id"] etc.  -> TraceID/SpanID/TraceFlags
//	Fields["observed_timestamp"] -> ObservedTimestamp
func FromEvent(e *event.Event) *LogRecord {
	if e == nil {
		return nil
	}
	r := &LogRecord{
		Timestamp:  e.Timestamp,
		Attributes: util.MapStr{},
	}
	if e.Fields != nil {
		r.Attributes = e.Fields.Clone()
	}
	if v, ok := resourceMap(e.Meta); ok {
		r.Resource = v
	}
	r.Resource = agentToResource(e.Agent, r.Resource)

	r.SeverityText, _ = r.Attributes[FieldLogLevel].(string)
	r.Body = r.Attributes[FieldMessage]
	r.EventName, _ = r.Attributes[FieldEventName].(string)
	r.TraceID, _ = r.Attributes[FieldTraceID].(string)
	r.SpanID, _ = r.Attributes[FieldSpanID].(string)
	r.TraceFlags, _ = r.Attributes[FieldTraceFlags].(string)
	if n, ok := toInt64(r.Attributes[FieldSeverityNumber]); ok {
		r.SeverityNumber = int32(n)
	}
	r.ObservedTimestamp = parseTimeField(r.Attributes[FieldObservedTimestamp])
	return r
}

// ToEvent renders the LogRecord back onto a framework event.Event. The
// typed fields are written into Fields under their canonical snake_case
// keys so the event round-trips losslessly.
func (r *LogRecord) ToEvent() *event.Event {
	e := &event.Event{
		Timestamp: r.Timestamp,
		Fields:    util.MapStr{},
	}
	if r.Attributes != nil {
		e.Fields = r.Attributes.Clone()
	} else {
		e.Fields = util.MapStr{}
	}

	if r.ObservedTimestamp.IsZero() {
		r.ObservedTimestamp = time.Now().UTC()
	}
	e.Fields[FieldObservedTimestamp] = r.ObservedTimestamp.UTC().Format(time.RFC3339Nano)
	if r.SeverityText != "" {
		e.Fields[FieldLogLevel] = r.SeverityText
	}
	if r.SeverityNumber != 0 {
		e.Fields[FieldSeverityNumber] = r.SeverityNumber
	}
	if r.Body != nil {
		e.Fields[FieldMessage] = r.Body
	}
	if r.EventName != "" {
		e.Fields[FieldEventName] = r.EventName
	}
	if r.TraceID != "" {
		e.Fields[FieldTraceID] = r.TraceID
	}
	if r.SpanID != "" {
		e.Fields[FieldSpanID] = r.SpanID
	}
	if r.TraceFlags != "" {
		e.Fields[FieldTraceFlags] = r.TraceFlags
	}
	if len(r.Resource) > 0 {
		if e.Meta == nil {
			e.Meta = util.MapStr{}
		}
		e.Meta[MetaResourceKey] = r.Resource.Clone()
	}
	if e.Agent == nil {
		e.Agent = agentFromResource(r.Resource)
	}
	return e
}

// agentToResource merges the collecting agent's identity into the record's
// Resource attributes so it survives the OTLP wire — the LogRecord model has
// no other carrier for it. Existing keys win: a resource that already
// describes its host (e.g. written by an upstream processor) is not
// overwritten, which also keeps cascaded exports idempotent. The merge is
// deterministic, so mutating a referenced resource map in place is safe.
func agentToResource(a *event.AgentMeta, res util.MapStr) util.MapStr {
	if a == nil {
		return res
	}
	if res == nil {
		res = util.MapStr{}
	}
	setIfAbsent(res, ResourceHostName, a.Hostname)
	setIfAbsent(res, ResourceHostIP, a.MajorIP)
	setIfAbsent(res, ResourceAgentID, a.AgentID)
	if len(a.Tags) > 0 {
		setIfAbsent(res, ResourceAgentTags, a.Tags)
	}
	return res
}

// agentFromResource rebuilds the collecting agent identity from Resource
// attributes written by agentToResource — the OTLP intake path has no other
// source for it. A resource without any agent-describing key yields nil.
func agentFromResource(res util.MapStr) *event.AgentMeta {
	if res == nil {
		return nil
	}
	a := &event.AgentMeta{}
	a.AgentID, _ = res[ResourceAgentID].(string)
	a.Hostname, _ = res[ResourceHostName].(string)
	a.MajorIP, _ = res[ResourceHostIP].(string)
	switch v := res[ResourceAgentTags].(type) {
	case []string:
		a.Tags = v
	case []interface{}: // OTLP wire round-trip decodes arrays as []interface{}
		tags := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
		if len(tags) > 0 {
			a.Tags = tags
		}
	}
	if a.AgentID == "" && a.Hostname == "" && a.MajorIP == "" && len(a.Tags) == 0 {
		return nil
	}
	return a
}

func setIfAbsent(m util.MapStr, k string, v interface{}) {
	if _, ok := m[k]; ok {
		return
	}
	if s, ok := v.(string); ok && s == "" {
		return
	}
	m[k] = v
}

func resourceMap(meta util.MapStr) (util.MapStr, bool) {
	if meta == nil {
		return nil, false
	}
	v, ok := meta[MetaResourceKey]
	if !ok {
		return nil, false
	}
	switch m := v.(type) {
	case util.MapStr:
		return m, true
	case map[string]interface{}:
		return util.MapStr(m), true
	}
	return nil, false
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	}
	return 0, false
}

func parseTimeField(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case util.Time:
		return time.Time(t)
	case string:
		if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return ts
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts
		}
	}
	return time.Time{}
}

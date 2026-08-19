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

package otel

import (
	"encoding/json"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// Envelope is the on-the-wire (queue) representation of one log record,
// byte-compatible with the agent's LogEvent JSON:
//
//	{"agent":{...},"metadata":{...},"payload":{...},"timestamp":"..."}
//
// payload carries the record Attributes (canonical snake_case keys plus
// the typed otel fields, see log_record.go); metadata carries the stable
// Resource attributes; agent identifies the collecting agent.
type Envelope struct {
	Agent     *event.AgentMeta `json:"agent,omitempty"`
	Meta      util.MapStr      `json:"metadata,omitempty"`
	Fields    util.MapStr      `json:"payload,omitempty"`
	Timestamp string           `json:"timestamp,omitempty"`
}

// DecodeEnvelope decodes one queue message payload into an event.Event.
//
// It tolerates both the agent LogEvent envelope and a bare JSON map
// (which is then treated as the record's Attributes wholesale).
//
// The envelope's flat metadata is the record's stable Resource attribute
// collection and is stored solely under Meta["resource"] — re-encoding
// via EncodeEnvelope then restores the original wire shape.
func DecodeEnvelope(data []byte) (*event.Event, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}

	// Bare JSON map fallback: a payload matching none of the envelope
	// fields is treated as the record's Attributes wholesale.
	if env.Agent == nil && len(env.Meta) == 0 && len(env.Fields) == 0 && env.Timestamp == "" {
		var raw util.MapStr
		if err := json.Unmarshal(data, &raw); err == nil && len(raw) > 0 {
			env.Fields = raw
		}
	}

	e := &event.Event{Agent: env.Agent}
	e.Fields = env.Fields
	if e.Fields == nil {
		e.Fields = util.MapStr{}
	}

	// metadata is the stable Resource attribute collection. A metadata
	// map that already carries an explicit resource key (produced by
	// EncodeEnvelope from a richer event.Meta) is kept as-is; otherwise
	// the flat map becomes the resource collection.
	e.Meta = util.MapStr{}
	if len(env.Meta) > 0 {
		if _, ok := env.Meta[MetaResourceKey]; ok {
			e.Meta = env.Meta.Clone()
		} else {
			e.Meta[MetaResourceKey] = env.Meta.Clone()
		}
	}

	if env.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339Nano, env.Timestamp); err == nil {
			e.Timestamp = ts
		}
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return e, nil
}

// EncodeEnvelope encodes an event.Event back into the queue envelope.
func EncodeEnvelope(e *event.Event) ([]byte, error) {
	env := Envelope{Agent: e.Agent}
	if len(e.Fields) > 0 {
		env.Fields = e.Fields
	}
	if len(e.Meta) > 0 {
		// A Meta holding only the resource collection flattens back to
		// the agent's wire shape (DecodeEnvelope's inverse).
		if res, ok := resourceMap(e.Meta); ok && len(e.Meta) == 1 {
			env.Meta = res
		} else {
			env.Meta = e.Meta.Clone()
		}
	}
	if !e.Timestamp.IsZero() {
		env.Timestamp = e.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return json.Marshal(env)
}

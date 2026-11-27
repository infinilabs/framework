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

	"infini.sh/framework/core/util"
)

// FieldDocTimestamp is the data stream document's one true timestamp
// (house standard): data streams reject documents without a date
// @timestamp, and every other time field folds into it at document
// output.
const FieldDocTimestamp = "@timestamp"

// Keys of the LogEvent envelope (see Envelope's json tags).
const (
	envelopePayloadKey = "payload"
	envelopeMetaKey    = "metadata"
)

// NormalizeDataStreamDoc turns one otel envelope (the queue's LogEvent
// JSON: {"metadata":{...},"payload":{...},"timestamp":"..."}) into a data
// stream compatible document. It is the document-rendering counterpart of
// DecodeEnvelope: the queue wire shape is defined in this package, and so
// is the policy that renders it for indexing — every indexing consumer
// (indexing_merge, lane_bulk) must call this instead of keeping its own
// copy, or the two paths silently diverge.
//
//   - payload fields are promoted to the top level (message etc. become
//     first-class doc fields)
//   - a top-level @timestamp is derived from payload.timestamp /
//     payload.observed_timestamp / the envelope timestamp / now — data
//     streams reject documents without a date @timestamp. @timestamp is the
//     document's one true timestamp (house standard): the mirrored top-level
//     FieldTimestamp, any payload FieldTimestamp and the OTel wire field
//     FieldObservedTimestamp (payload or top-level on bare documents) are
//     folded into it and removed.
//   - a bare document (no "payload" key) keeps its own @timestamp when
//     present; only a missing one is stamped with now.
//   - metadata.file (agent collection origin) lives in @agent.file_* — the
//     former top-level "file" promotion is gone (zero readers).
//
// Documents that are not envelopes (no "payload" key) pass through
// unchanged except for the @timestamp stamp.
func NormalizeDataStreamDoc(doc []byte) []byte {
	var m util.MapStr
	if err := util.FromJSONBytes(doc, &m); err != nil {
		return doc
	}

	ts := ""
	if payload, ok := m[envelopePayloadKey].(map[string]interface{}); ok {
		for _, k := range []string{FieldTimestamp, FieldDocTimestamp, FieldObservedTimestamp} {
			if v, ok := payload[k].(string); ok && v != "" {
				ts = v
				break
			}
		}
	}
	if ts == "" {
		if v, ok := m[FieldTimestamp].(string); ok {
			ts = v
		}
	}
	if ts == "" {
		if v, ok := m[FieldDocTimestamp].(string); ok { // bare doc already carrying one
			ts = v
		}
	}
	if ts == "" {
		// bare doc carrying only the OTel wire field: the agent's
		// observation time is the document's best-known time, and the
		// deletion below would otherwise drop it in favor of "now"
		if v, ok := m[FieldObservedTimestamp].(string); ok && v != "" {
			ts = v
		}
	}
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339Nano)
	}

	out := util.MapStr{}
	if payload, ok := m[envelopePayloadKey].(map[string]interface{}); ok {
		for k, v := range payload {
			out[k] = v
		}
	} else {
		for k, v := range m {
			out[k] = v
		}
	}
	out[FieldDocTimestamp] = ts
	delete(out, FieldTimestamp) // @timestamp is the single source of truth
	// observed_timestamp is OTel-model wire state (agent observation time,
	// auto-filled in this package). At document output it is folded into
	// @timestamp (fallback above) and stripped: the chain already records
	// the semantics with better precision per hop — @timestamp (log time),
	// @agent.observed_at (collection), @gateway.observed_at (relay) — and
	// nothing reads a top-level observed_timestamp.
	delete(out, FieldObservedTimestamp)

	if meta, ok := m[envelopeMetaKey].(map[string]interface{}); ok {
		if rt, ok := meta["log_type"].(string); ok && rt != "" {
			out["log_type"] = rt
		}
		// per-pattern label (e.g. server|deprecation|slowlog|gc) — virtual
		// streams split on this field.
		if lk, ok := meta["log_kind"].(string); ok && lk != "" {
			out["log_kind"] = lk
		}
	}
	return util.MustToJSONBytes(out)
}

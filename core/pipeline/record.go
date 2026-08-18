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

package pipeline

import "infini.sh/framework/core/event"

// ──────────────────────────────────────────────────────────────────────────
// Record processing convention (framework-level; see
// docs/references/pipeline_record.md for the full specification)
//
// This convention turns the queue-oriented pipeline (batches of opaque
// payloads) into a record-processing ecosystem:
//
//   1. A batch-splitting processor (for_each, or any custom one) decodes
//      each payload into an *event.Event via a registered RecordCodec.
//   2. The record is exposed to the sub-chain under RecordContextKey;
//      transform processors read/mutate it in place via CurrentRecord.
//   3. Processors may MarkDropped the record to remove it from the batch.
//   4. Optional failure tags are stored under FailureTagsKey by the
//      splitter when a sub-chain errors (on_failure: tag strategy).
//   5. The re-encoded payload replaces the original message in the batch.
//
// Writing a compatible record processor means: use CurrentRecord(ctx) to
// fetch the record, mutate Fields/Meta in place, call MarkDropped to drop,
// and NEVER touch the raw payload bytes — encoding belongs to the codec.
//
// Nesting: a sub-chain may itself contain another batch-splitting
// processor bound to a different message field (see the nested-for_each
// tests); per-record state is scoped by the splitter, not global.
// ──────────────────────────────────────────────────────────────────────────

// RecordContextKey is the Context key under which per-record processors
// find the *event.Event of the record currently being processed.
const RecordContextKey = "record"

// FailureTagsKey is the Context key holding the mutable []string of
// failure tags attached to the current record. The batch splitter
// initializes it per record (when the on_failure: tag strategy is
// configured) and appends the failure_tag on sub-chain errors, mirroring
// the Beats/Vector failure-tag convention.
const FailureTagsKey = "record_failure_tags"

// CurrentRecord returns the record being processed in this context, if
// the processor is running inside a per-record sub-chain.
func CurrentRecord(ctx *Context) (*event.Event, bool) {
	if ctx == nil {
		return nil, false
	}
	rec, ok := ctx.Get(RecordContextKey).(*event.Event)
	if !ok || rec == nil {
		return nil, false
	}
	return rec, true
}

// droppedMarker is stored in event.Private by MarkDropped; for_each
// removes dropped records from the batch before it is forwarded.
type droppedMarker struct{}

// MarkDropped marks the current record to be dropped from the batch.
func MarkDropped(rec *event.Event) {
	if rec != nil {
		rec.Private = droppedMarker{}
	}
}

// IsDropped reports whether the record was marked for dropping.
func IsDropped(rec *event.Event) bool {
	if rec == nil {
		return false
	}
	_, ok := rec.Private.(droppedMarker)
	return ok
}

// CurrentFailureTags returns the failure-tag list of the current record
// (attaching it lazily when absent), so processors can append diagnostic
// tags without nil checks. Returns nil when no record scope is active.
func CurrentFailureTags(ctx *Context) []string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Get(FailureTagsKey).([]string); ok {
		return v
	}
	return nil
}

// AppendFailureTag records a processing-failure tag on the current record.
// No-op outside a record scope or when no tag slice was initialized.
func AppendFailureTag(ctx *Context, tag string) {
	if ctx == nil || tag == "" {
		return
	}
	if tags, ok := ctx.Get(FailureTagsKey).(*[]string); ok && tags != nil {
		*tags = append(*tags, tag)
	}
}

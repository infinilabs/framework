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

import (
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

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
//   3. Processors may MarkDropped the record to remove it from the batch,
//      or deliver it to a sink (output{lane}/output{queue} record scope)
//      which MarkDelivered's it — both states mean the record has left
//      the active batch: later processors skip it, and the host's
//      encode-back pass empties its message slot (single delivery).
//   4. Optional failure tags are stored under FailureTagsKey by the
//      splitter when a sub-chain errors (on_failure: tag strategy).
//   5. The re-encoded payload replaces the original message in the batch.
//
// Writing a compatible record processor means: use CurrentRecord(ctx) to
// fetch the record, mutate Fields/Meta in place, call MarkDropped to drop,
// and NEVER touch the raw payload bytes — encoding belongs to the codec.
//
// ── Single-owner rule ──────────────────────────────────────────────────
// A record has exactly one owner at any moment: the host while it is in
// the batch, the sink after delivery. Delivering a record transfers
// ownership — pointer handoffs (lanes) included: the receiver may read
// and even mutate (codec Encode that stamps fields) the record the
// moment it arrives. Consequently the sender must not touch a record
// once it is handed over — not even to READ a marker: the channel's
// happens-before is one-directional (pre-send writes are visible to the
// receiver), while any post-send access by the sender is unsynchronized
// with the receiver's writes.
//
// Terminal state therefore lives WHERE ITS READERS ARE:
//   - dropped: on the record (event.Private) — a dropped record never
//     crosses goroutines, so the sender's later reads are safe;
//   - delivered: on the per-record CONTEXT (DeliveredContextKey) — the
//     context stays sender-goroutine-local, so the sink sets it before
//     the handoff and the runner guards / hosts read it afterwards,
//     all properly ordered.
// After delivery: the chain runner stops the remaining processors of the
// record's chain, hosts skip the record in later segments and the
// encode-back pass (emptying its message slot), and a second sink in the
// same chain is rejected with a warning (use lanes: [...] for fan-out or
// the clone processor for forks).
//
// Receiving side: a lane consumer takes ownership at intake
// (TakeOwnership clears any stale record-borne markers; delivery state
// never rides the pointer across a lane).
//
// Nesting: a sub-chain may itself contain another batch-splitting
// processor bound to a different message field (see the nested-for_each
// tests); per-record state is scoped by the splitter, not global.
// ──────────────────────────────────────────────────────────────────────────

// RecordContextKey is the Context key under which per-record processors
// find the *event.Event of the record currently being processed.
const RecordContextKey = "record"

// FailureTagsKey is the Context key holding the mutable *[]string of
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
	// Peek skips the GetValue miss path, which wraps a stack-capturing
	// error for every probe outside a record scope.
	v, ok := ctx.Peek(RecordContextKey)
	if !ok {
		return nil, false
	}
	rec, ok := v.(*event.Event)
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

// DeliveredContextKey is the Context key carrying the CURRENT record's
// delivery state. It lives on the context, never on the record: at Emit
// the record pointer crosses to the receiver, and any later sender-side
// access — even reading a marker out of event.Private — would race with
// the receiver's writes. The context stays on the sender's goroutine, so
// the sink sets the flag before the handoff and the runner guards and
// hosts read it afterwards, all in happens-before order.
const DeliveredContextKey = "record_delivered"

// MarkDelivered records that the CURRENT record's delivery has been taken
// over by a sink (single-owner rule). MUST be called before the record is
// handed over; delivery is terminal for this record's batch lifecycle.
func MarkDelivered(ctx *Context) {
	if ctx != nil {
		ctx.Set(DeliveredContextKey, true)
	}
}

// IsDelivered reports the current record's delivery state (read-only;
// runner guards between processors). Hosts that finish a record use
// TakeDelivery instead, which also clears the flag.
func IsDelivered(ctx *Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Peek(DeliveredContextKey)
	b, ok2 := v.(bool)
	return ok && ok2 && b
}

// TakeDelivery reports and clears the current record's delivery state.
// Host-side, once per record after its sub-chain returned: the flag rides
// contexts that outlive one record (for_each rebinds RecordContextKey on a
// shared context), so leaving it set would leak "delivered" onto the next
// record. When it returns true the caller must not touch the record again.
func TakeDelivery(ctx *Context) bool {
	if !IsDelivered(ctx) {
		return false
	}
	ctx.Set(DeliveredContextKey, false)
	return true
}

// RecordLeft reports whether the CURRENT record has left the active batch
// — dropped, or delivered to a sink that now owns it. The mid-chain guard
// form for chain runners: delivery state comes from the context, drop
// state from the record (a dropped record never crosses goroutines, so
// reading its marker is safe). Evaluation is ordered: a delivered record
// short-circuits before any record access.
func RecordLeft(ctx *Context) bool {
	if IsDelivered(ctx) {
		return true
	}
	rec, ok := CurrentRecord(ctx)
	return ok && IsDropped(rec)
}

// TakeOwnership clears terminal markers on a record received across an
// ownership-transfer boundary (a lane subscription intake). Delivery
// state never rides the pointer (it lives on the sender's context), but
// this defends the boundary: a foreign emitter that marks the record
// itself would otherwise have its stale marker read by the receiver's
// RecordLeft guards, silently skipping every record.
type deliveredMarker struct{} // legacy record-borne form; see above

func TakeOwnership(rec *event.Event) {
	if rec == nil {
		return
	}
	switch rec.Private.(type) {
	case droppedMarker, deliveredMarker:
		rec.Private = nil
	}
}

// CurrentFailureTags returns the failure-tag list of the current record,
// or nil when no record scope is active or the splitter did not initialize
// a tag list (only the on_failure: tag strategy does). The canonical
// storage is a *[]string so appends via AppendFailureTag stay visible.
func CurrentFailureTags(ctx *Context) []string {
	if ctx == nil {
		return nil
	}
	switch v := ctx.Get(FailureTagsKey).(type) {
	case *[]string:
		if v != nil {
			return *v
		}
	case []string:
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

// CloneContextKey is the Context key under which the clone processor
// deposits cloned records; the batch-splitting host materializes them as
// additional batch members after the sub-chain finishes the original.
const CloneContextKey = "record_clones"

// AppendClone registers a clone of the current record for the host to
// materialize (see clone processor). No-op outside a record scope.
func AppendClone(ctx *Context, rec *event.Event) {
	if ctx == nil || rec == nil {
		return
	}
	if list, ok := ctx.Get(CloneContextKey).(*[]*event.Event); ok {
		*list = append(*list, rec)
	}
}

// TakeClones returns and clears the pending clones registered in this
// context (host-side; call after the per-record sub-chain).
func TakeClones(ctx *Context) []*event.Event {
	if ctx == nil {
		return nil
	}
	if list, ok := ctx.Get(CloneContextKey).(*[]*event.Event); ok && len(*list) > 0 {
		out := *list
		*list = nil
		return out
	}
	return nil
}

// TraceIDLabel is the metadata label carrying the record-level trace id.
// Entries (consumer / lane hosts / connectors) stamp it once; it then rides
// the record across queues and lanes unchanged (D5 correlation).
const TraceIDLabel = "_trace_id"

// SetTraceID stamps the trace id onto the record's metadata labels.
func SetTraceID(rec *event.Event, id string) {
	if rec == nil || id == "" {
		return
	}
	if rec.Metadata.Labels == nil {
		rec.Metadata.Labels = util.MapStr{}
	}
	rec.Metadata.Labels[TraceIDLabel] = id
}

// TraceID reads the record's trace id ("" when unset).
func TraceID(rec *event.Event) string {
	if rec == nil || rec.Metadata.Labels == nil {
		return ""
	}
	v, _ := rec.Metadata.Labels[TraceIDLabel].(string)
	return v
}

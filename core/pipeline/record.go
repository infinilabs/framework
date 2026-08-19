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

// RecordContextKey is the Context key under which per-record processors
// find the *event.Event of the record currently being processed.
//
// The for_each processor (see modules/pipeline) splits a batch of
// queue.Message into records, decodes each one, stores it under this key
// and runs its sub-chain; transform processors (e.g. the enterprise
// dissect/field_standardize processors) read and mutate the record
// in place through this convention.
const RecordContextKey = "record"

// CurrentRecord returns the record being processed in this context, if
// the processor is running inside a per-record (for_each) sub-chain.
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

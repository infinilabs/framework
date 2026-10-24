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
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package event

import (
	"sync"
	"sync/atomic"
	"time"
)

// The record pool recycles *Event across record-mode batches: decode-heavy
// hosts (for_each) otherwise allocate a fresh Event, its field maps and the
// whole decoded tree per record only to drop them after encode.
//
// Contract:
//   - AcquireEvent returns a zeroed event; Decode-style producers write into
//     it as if it were freshly allocated.
//   - ReleaseEvent returns the event to the pool. It must be called only
//     after the record has been fully encoded (or dropped) AND no downstream
//     processor retains a reference — the host owns the record lifetime.
//   - ReleaseEvent is a no-op on events that did not come from the pool
//     (codec- or clone-allocated), so a host can release every record
//     unconditionally.
//   - A double release is rejected atomically (second call returns false)
//     instead of pooling the same event twice.
//
// Reset policy: scalar fields are zeroed; the hot maps (Fields, Metadata
// labels, Meta) are cleared in place — their bucket arrays are reused —
// unless they grew past maxRecycledEntries, in which case they are dropped
// so one giant record cannot pin memory in the pool forever.

// maxRecycledEntries caps in-place map reuse; bigger maps are released to
// the GC instead of living in the pool.
const maxRecycledEntries = 128

type pooledEvent struct {
	live atomic.Bool
	ev   Event
}

var recordPool = sync.Pool{New: func() interface{} { return &pooledEvent{} }}

var (
	poolAcquires atomic.Int64
	poolReleases atomic.Int64
)

// AcquireEvent returns an event from the record pool (zeroed).
func AcquireEvent() *Event {
	pe := recordPool.Get().(*pooledEvent)
	pe.live.Store(true)
	pe.ev.pooled = pe
	poolAcquires.Add(1)
	return &pe.ev
}

// ReleaseEvent recycles a pooled event; returns true when the event went
// back to the pool, false for non-pooled events or a rejected double
// release.
func ReleaseEvent(e *Event) bool {
	if e == nil {
		return false
	}
	pe, ok := e.pooled.(*pooledEvent)
	if !ok {
		return false
	}
	if !pe.live.CompareAndSwap(true, false) {
		return false
	}
	e.resetForPool()
	e.pooled = nil
	poolReleases.Add(1)
	recordPool.Put(pe)
	return true
}

// PoolStats reports cumulative acquire/release counts; the difference is
// the number of pooled events currently in use (leak detection).
func PoolStats() (acquired, released int64) {
	return poolAcquires.Load(), poolReleases.Load()
}

// resetForPool clears an event for reuse; see the pool doc for the
// map-retention policy.
func (e *Event) resetForPool() {
	e.Fields = recycleMap(e.Fields)
	e.Metadata.Labels = recycleMap(e.Metadata.Labels)
	e.Meta = recycleMap(e.Meta)
	e.Metadata.Category = ""
	e.Metadata.Name = ""
	e.Metadata.Version = ""
	e.Metadata.Datatype = ""
	e.Agent = nil
	e.Private = nil
	e.QueueName = ""
	e.TimeSeries = false
	e.Timestamp = time.Time{}
}

// recycleMap clears a map in place when it is small enough to be worth
// keeping, else drops it.
func recycleMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if len(m) > maxRecycledEntries {
		return nil
	}
	for k := range m {
		delete(m, k)
	}
	return m
}

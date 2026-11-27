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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package kafka_queue

import (
	"fmt"
	"unsafe"

	"github.com/twmb/franz-go/pkg/kgo"
)

// defaultOrderMemoryBudget bounds allocated interval blocks per session, excluding
// SDK buffers and partition maps. Exhaustion fails the read without implicit commit.
const defaultOrderMemoryBudget = 64 << 20

// deliveryBlockRuns amortizes allocations while allowing whole committed blocks to be freed.
const deliveryBlockRuns = 1024

// deliveryRun represents one partition's consecutive NextOffsets in delivery order.
//
// Every position in this inclusive interval identifies an actual delivered message.
// Partition changes and offset gaps start another run so unseen positions cannot
// become valid acknowledgement boundaries.
type deliveryRun struct {
	partition   int32
	first, last int64
}

// deliveryBlock stores a fixed number of delivery intervals in a linked sequence.
//
// Consumed blocks can be freed without retaining one large backing array.
// Only runs[begin:end] remain uncommitted. end never moves backwards.
type deliveryBlock struct {
	next       *deliveryBlock
	runs       [deliveryBlockRuns]deliveryRun
	begin, end int
}

// deliveryOrder retains the delivery sequence needed to resolve an acknowledgement prefix.
//
// A caller returns one NextOffset even when earlier messages came from several
// partitions. The intervals preserve that order without adding shared interface fields.
//
// Consumer.opMu protects all fields. Rebalance callbacks only expire the session.
// Kafka stores recovery positions. This in-memory structure is discarded on Close.
type deliveryOrder struct {
	head, tail *deliveryBlock
	// base stores next positions from the last fully successful prefix.
	// delivered stores highest appended positions, even after retirement, to reject rewinds.
	base, delivered map[int32]int64
	// used counts linked block bytes. budget limits them, excluding maps and SDK buffers.
	used, budget uint64
}

// append records the given delivery order and merges adjacent positions of the same partition.
//
// Only strictly increasing positions within each partition are valid. Gaps remain
// separate intervals because a gap does not identify a delivered message.
//
// FetchMessages supplies validated record addresses and holds Consumer.opMu. Failure
// can leave a partial append, so the caller must expire the session and discard
// the unpublished batch rather than reuse the table.
func (o *deliveryOrder) append(records []*kgo.Record) error {
	if o.delivered == nil {
		o.delivered = make(map[int32]int64)
	}
	for _, r := range records {
		next := r.Offset + 1
		if last, ok := o.delivered[r.Partition]; ok && next <= last {
			return fmt.Errorf("kafka delivery moved backwards on partition %d", r.Partition)
		}
		if o.tail != nil {
			last := &o.tail.runs[o.tail.end-1]
			if last.partition == r.Partition && last.last == r.Offset {
				last.last = next
				o.delivered[r.Partition] = next
				continue
			}
		}
		if o.tail == nil || o.tail.end == deliveryBlockRuns {
			size := uint64(unsafe.Sizeof(deliveryBlock{}))
			if size > o.budget-o.used {
				return fmt.Errorf("kafka delivery order exceeded memory budget %d; commit more frequently", o.budget)
			}
			b := &deliveryBlock{}
			if o.tail == nil {
				o.head = b
			} else {
				o.tail.next = b
			}
			o.tail = b
			o.used += size
		}
		o.tail.runs[o.tail.end] = deliveryRun{partition: r.Partition, first: next, last: next}
		o.tail.end++
		o.delivered[r.Partition] = next
	}
	return nil
}

// target resolves an acknowledgement boundary into a fixed map of per-partition next positions.
//
// The map includes all earlier deliveries and the successful base. Later appends
// cannot widen it. Resolving a target neither commits nor changes retained history.
//
// The caller holds Consumer.opMu. The boolean is true when the base already covers
// the position, which needs no broker request. Unknown uncommitted positions fail.
func (o *deliveryOrder) target(partition int32, next int64) (map[int32]int64, bool, error) {
	if partition < 0 || next <= 0 {
		return nil, false, fmt.Errorf("invalid Kafka acknowledgement %d:%d", partition, next)
	}
	if n, ok := o.base[partition]; ok && next <= n {
		return nil, true, nil
	}
	targets := make(map[int32]int64, len(o.base)+1)
	for p, n := range o.base {
		targets[p] = n
	}
	for b := o.head; b != nil; b = b.next {
		for i := b.begin; i < b.end; i++ {
			r := b.runs[i]
			if r.partition == partition && next >= r.first && next <= r.last {
				targets[partition] = next
				return targets, false, nil
			}
			targets[r.partition] = r.last
		}
	}
	return nil, false, fmt.Errorf("kafka acknowledgement %d:%d was not delivered by this session", partition, next)
}

// retire advances the successful base and frees the corresponding delivery prefix.
//
// A failed or incomplete response can leave some Kafka positions unchanged.
// Keeping the history until every target succeeds prevents treating that result
// as a complete business confirmation.
//
// The caller holds Consumer.opMu and supplies the boundary and map returned by target.
// A boundary inside an interval trims it. Only fully consumed blocks are freed.
func (o *deliveryOrder) retire(partition int32, next int64, targets map[int32]int64) {
	o.base = targets
	for o.head != nil {
		b := o.head
		for b.begin < b.end {
			r := &b.runs[b.begin]
			if r.partition == partition && next >= r.first && next <= r.last {
				if next < r.last {
					r.first = next + 1
					return
				}
				b.begin++
				if b.begin == b.end {
					o.head = b.next
					o.used -= uint64(unsafe.Sizeof(deliveryBlock{}))
					if o.head == nil {
						o.tail = nil
					}
				}
				return
			}
			b.begin++
		}
		o.head = b.next
		o.used -= uint64(unsafe.Sizeof(deliveryBlock{}))
	}
	o.tail = nil
}

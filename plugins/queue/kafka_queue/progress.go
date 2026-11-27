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
	"context"
	"fmt"
	"math"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/queue"
)

// progressSegment reserves a value outside Kafka's int32 partition range for query results.
// These markers support equality comparisons and cannot address a message or reset.
const progressSegment = math.MaxInt64

// Helper function to load retained starts and ends for browsing, resets, and statistics.
//
// A missing partition or failed partition query must not become a valid address
// or an empty queue. Both maps must cover the same nonempty partition set.
//
// The calls are separate Kafka requests, not an atomic snapshot of concurrent writes.
func (q *KafkaQueue) positionBounds(ctx context.Context, topic string) (map[int32]int64, map[int32]int64, error) {
	starts, err := q.adminClient.ListStartOffsets(ctx, topic)
	if err != nil {
		return nil, nil, err
	}
	ends, err := q.adminClient.ListEndOffsets(ctx, topic)
	if err != nil {
		return nil, nil, err
	}
	if len(starts[topic]) == 0 || len(starts[topic]) != len(ends[topic]) {
		return nil, nil, fmt.Errorf("incomplete Kafka partition bounds for %s", topic)
	}
	first, last := make(map[int32]int64), make(map[int32]int64)
	for part, start := range starts[topic] {
		end, ok := ends[topic][part]
		if start.Err != nil {
			return nil, nil, start.Err
		}
		if !ok {
			return nil, nil, fmt.Errorf("missing Kafka partition end")
		}
		if end.Err != nil {
			return nil, nil, end.Err
		}
		if start.Offset < 0 || end.Offset < start.Offset {
			return nil, nil, fmt.Errorf("invalid Kafka log bounds")
		}
		first[part], last[part] = start.Offset, end.Offset
	}
	return first, last, nil
}

// LatestOffset returns the fixed marker that GetOffset uses for a fully caught-up group.
//
// Kafka partition positions have no topic-wide order. Shared queue code compares
// this marker for equality, rather than comparing unrelated physical positions.
//
// The result is not a browsing address, reset target, or cross-consumer ranking.
func (q *KafkaQueue) LatestOffset(_ *queue.QueueConfig) queue.Offset {
	return queue.NewOffset(progressSegment, 1)
}

// GetOffset reports whether a group's saved recovery positions reach every queried partition end.
//
// Shared lag aggregation ignores returned errors. Query uncertainty therefore returns
// the lag marker with nil error instead of reporting caught-up. Logging failures
// cannot change this conservative result.
//
// Missing commits imply lag when retained data exists. An empty partition needs
// no commit. The marker is not a physical position, and the queries are not atomic.
func (q *KafkaQueue) GetOffset(k *queue.QueueConfig, c *queue.ConsumerConfig) (queue.Offset, error) {
	lag := queue.NewOffset(progressSegment, 0)
	if k == nil || c == nil {
		return lag, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	starts, ends, err := q.positionBounds(ctx, k.ID)
	if err != nil {
		_ = log.Errorf("Kafka progress query for %s failed: %v", k.ID, err)
		return lag, nil
	}
	offsets, err := q.adminClient.FetchOffsetsForTopics(ctx, getGroupForKafka(c.Group, k.ID), k.ID)
	if err != nil {
		_ = log.Errorf("Kafka commit query for %s failed: %v", k.ID, err)
		return lag, nil
	}
	for part, end := range ends {
		committed, ok := offsets.Lookup(k.ID, part)
		if !ok || committed.At < 0 {
			if starts[part] == end {
				continue
			}
			return lag, nil
		}
		if committed.At != end {
			return lag, nil
		}
	}
	// Expansion between the two queries must not create a false caught-up result.
	topics, err := q.adminClient.ListTopics(ctx, k.ID)
	if err != nil {
		_ = log.Errorf("Kafka metadata query for %s failed: %v", k.ID, err)
		return lag, nil
	}
	topic, ok := topics[k.ID]
	if !ok || topic.Err != nil || len(topic.Partitions) != len(ends) {
		return lag, nil
	}
	for part, detail := range topic.Partitions {
		if _, ok := ends[part]; !ok || detail.Err != nil {
			return lag, nil
		}
	}
	return queue.NewOffset(progressSegment, 1), nil
}

// Depth sums retained offset spans across every partition in the topic.
//
// Counting only partition zero hides data elsewhere. Compacted logs can have gaps,
// so the sum is an offset span rather than an exact count of remaining messages.
//
// The shared signature has no error result. Query failures and overflow panic
// instead of returning a misleading empty depth.
func (q *KafkaQueue) Depth(topic string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	starts, ends, err := q.positionBounds(ctx, topic)
	if err != nil {
		panic(err)
	}
	var total int64
	for part, end := range ends {
		span := end - starts[part]
		if span > math.MaxInt64-total {
			panic("Kafka depth overflow")
		}
		total += span
	}
	return total
}
